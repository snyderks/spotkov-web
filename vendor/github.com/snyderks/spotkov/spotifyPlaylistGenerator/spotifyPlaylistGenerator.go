package spotifyPlaylistGenerator

import (
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/snyderks/spotkov/configRead"
	"github.com/snyderks/spotkov/lastFm"
	"github.com/snyderks/spotkov/tools"
	"github.com/zmb3/spotify"
)

const playlistName = "Generated by Spotkov"
const maxSongLength = 50

var songsWg sync.WaitGroup

var debugMessages bool

// songIDs is a map of known songs that is safe for use with goroutines
// with the embedded mutex.
type songIDs struct {
	sync.RWMutex
	M map[songAndArtist]spotify.ID
}

var IDs songIDs

// songAndArtist is used as a key for a map
// to organize song IDs with.
type songAndArtist struct {
	Title  string
	Artist string
}

func CreatePlaylist(songs []lastFm.Song, client *spotify.Client, userID string) error {
	playlistsPage, err := client.GetPlaylistsForUser(userID)
	playlistExists := false
	var playlistId spotify.ID
	if err == nil {
		playlists := playlistsPage.Playlists
		for _, playlist := range playlists {
			if playlist.Name == playlistName {
				playlistId = playlist.ID
				playlistExists = true
				break
			}
		}
		if playlistExists == false {
			playlistCreated, err := client.CreatePlaylistForUser(userID, playlistName, false)
			if err != nil {
				panic(err)
			}
			playlistId = playlistCreated.SimplePlaylist.ID
		}
	} else {
		s := fmt.Sprint("Couldn't retrieve the playlists for", userID, ".")
		fmt.Println(s)
		return errors.New(s)
	}
	// retrieve tracklist async
	tracks := make([]spotify.ID, len(songs))
	for i, song := range songs {
		songsWg.Add(1)
		go searchAndAddTrackToList(client, song.Title, song.Artist, tracks, i)
	}
	songsWg.Wait()

	tracks = clearFailuresFromList(tracks)
	// can do this in one pass
	if len(tracks) <= maxSongLength {
		// rewrite the playlist that Spotkov writes to
		err := client.ReplacePlaylistTracks(userID, playlistId, tracks...)
		if err != nil {
			s := fmt.Sprint("Couldn't clear and update the playlist. No changes have been made to" + playlistName)
			fmt.Println(s)
			return errors.New(s)
		}
		fmt.Println("Successfully created the playlist under the name", playlistName)
	} else {
		// split the playlist into chunks of length maxSongLength.
		chunks := len(tracks)/maxSongLength + 1
		trackChunks := make([][]spotify.ID, 0)
		for i := 0; i < chunks; i++ {
			if maxSongLength*(i+1) > len(tracks) {
				trackChunks = append(trackChunks, tracks[maxSongLength*i:])
			} else {
				trackChunks = append(trackChunks, tracks[maxSongLength*i:maxSongLength*(i+1)])
			}
		}
		for i, trackChunk := range trackChunks {
			attempts := 0
			maxAttempts := 100
			if i == 0 {
				err := client.ReplacePlaylistTracks(userID, playlistId, trackChunk...)
				for err != nil {
					if attempts >= maxAttempts {
						s := fmt.Sprint("Couldn't clear and update the playlist. No changes have been made.")
						fmt.Println(s)
						return errors.New(s)
					}
					time.Sleep(500 * time.Millisecond)
					err = client.ReplacePlaylistTracks(userID, playlistId, trackChunk...)
					attempts += 1
				}
			} else {
				_, err := client.AddTracksToPlaylist(userID, playlistId, trackChunk...)
				for err != nil {
					if attempts >= maxAttempts {
						s := fmt.Sprint("Adding some tracks failed. The playlist contains at least" + string(maxSongLength) + "tracks.")
						fmt.Println(s)
						return errors.New(s)
					}
					time.Sleep(500 * time.Millisecond)
					_, err = client.AddTracksToPlaylist(userID, playlistId, trackChunk...)
					attempts += 1
				}
			}
		}
		fmt.Println("Successfully created the playlist under the name", playlistName)
	}
	// save back the known songs
	cacheSongIDs(IDs.M)
	return nil
}

func searchAndAddTrackToList(client *spotify.Client, title string, artist string, tracks []spotify.ID, index int) error {
	defer songsWg.Done()

	// check if there's already a known ID for this song
	// read lock the map
	IDs.RLock()
	id, in := IDs.M[songAndArtist{title, artist}]
	IDs.RUnlock()

	if in {
		tracks[index] = id
		if debugMessages {
			fmt.Println("Got id", id, "for", title, "from cache.")
		}
	} else {
		// only want one result
		limit := 1
		options := spotify.Options{Limit: &limit}
		query := "track:" + tools.LowerAndStripNonAlphaNumeric(title) + " artist:" + tools.LowerAndStripNonAlphaNumeric(artist)
		if debugMessages {
			fmt.Println("Searching for", title, "with query", query)
		}
		results, err := client.SearchOpt(query, spotify.SearchTypeTrack, &options)
		// keep retrying until there's a response (10 retries max)
		i := 0
		for err != nil && i < 10 {
			if debugMessages {
				fmt.Println("Failed on searching for track:" + title)
			}
			time.Sleep(250 * time.Millisecond)
			results, err = client.SearchOpt(query, spotify.SearchTypeTrack, &options)
			i++
		}
		if i >= 10 {
			if debugMessages {
				fmt.Println("Gave up on track:" + title + "by" + artist)
			}
			return errors.New("Failed to find track:" + title + "by" + artist)
		}
		resultTracks := results.Tracks.Tracks
		if len(resultTracks) > 0 {
			trackID := resultTracks[0].ID
			tracks[index] = trackID
			if debugMessages {
				fmt.Println("Got id", id, "for", title)
			}
			// save the ID in the known list
			// lock and unlock with the embedded mutex
			IDs.Lock()
			IDs.M[songAndArtist{title, artist}] = trackID
			IDs.Unlock()
		} else if debugMessages {
			fmt.Println("Got no results for query:", query)
		}
	}
	return nil
}

func clearFailuresFromList(list []spotify.ID) []spotify.ID {
	newList := make([]spotify.ID, 0, len(list))
	// any blank IDs are removed.
	for _, ID := range list {
		if ID != "" {
			newList = append(newList, ID)
		}
	}
	return newList
}

// readCachedSongIDs takes in a map[songAndArtist]spotify.ID object and reads in the
// cached (on filesystem) list of known IDs for Spotify songs. Uses maps for fast lookup.
func readCachedSongIDs(songs interface{}) error {
	file, err := os.Open("songIDs" + ".gob")
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(songs)
	} else if debugMessages {
		fmt.Println("Error: ", err)
	}
	file.Close()
	return err
}

func cacheSongIDs(songs map[songAndArtist]spotify.ID) error {
	// for now, hardcoding the file cache.
	file, err := os.Create("songIDs" + ".gob")
	if debugMessages {
		fmt.Println("Saving cache...")
	}
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(songs)
	} else if debugMessages {
		fmt.Println("Error: ", err)
	}
	file.Close()
	return err
}

func init() {
	c, err := configRead.Read("config.json")
	if err == nil {
		debugMessages = c.Debug
	} else {
		debugMessages = false
	}
	IDs = songIDs{M: make(map[songAndArtist]spotify.ID)}
	// pull all of the known songs.
	readCachedSongIDs(IDs.M)
}
