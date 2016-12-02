// handles the retrieval of data from Last.FM (currently without authentication)

package lastFm

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/snyderks/spotkov/configRead"
)

// Types to interpret JSON data returned from tracks played
type SongsPage struct {
	RecentTracks tracksWrapper `json:"recentTracks"`
}

type tracksWrapper struct {
	Tracks   []track  `json:"track"`
	Metadata metadata `json:"@attr"`
}

type metadata struct {
	UserId       string `json:"user"`
	Page         string `json:"page"`
	SongsPerPage string `json:"SongsPerPage"`
	TotalPages   string `json:"totalPages"`
	TotalSongs   string `json:"total"`
}

type track struct {
	Artist     artist                 `json:"artist"`
	Title      string                 `json:"name"`
	Album      album                  `json:"album"`
	Timestamp  trackDate              `json:"date"`
	Attributes map[string]interface{} `json:"@attr"`
}

type trackDate struct {
	UnixTime string `json:"uts"`
	TextDate string `json:"#text"`
}

type artist struct {
	Title string `json:"#text"`
}

type album struct {
	Title string `json:"#text"`
}

type Song struct {
	Artist    string
	Title     string
	Timestamp time.Time
}

type songFile struct {
	Songs []Song
}

type lastFMError struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

func readCachedSongs(userID string, songs interface{}) error {
	file, err := os.Open("./cached-songs/" + userID + ".gob")
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(songs)
	}
	file.Close()
	return err
}

func cacheSongs(userID string, songs songFile) error {
	file, err := os.Create("./cached-songs/" + userID + ".gob")
	if err != nil {
		err = os.Mkdir("./cached-songs", 0666)
	}
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(songs)
	}
	file.Close()
	return err
}

var pagesWg sync.WaitGroup

const baseLastURI = "http://ws.audioscrobbler.com/2.0/"

func ReadLastFMSongs(user_id string) ([]Song, error) {
	file := songFile{}
	err := readCachedSongs(user_id, &file)
	titlesConcat := file.Songs
	if len(titlesConcat) == 0 {
		err = errors.New("Length of cached songs is 0. Regenerating...")
	}

	var errLastFM lastFMError

	if err != nil { // couldn't retrieve a cached version
		titlesConcat, errLastFM = getAllTitles(make([]Song, 0), time.Time{}, user_id)
	} else {
		var lastDate time.Time
		for _, song := range titlesConcat {
			if !song.Timestamp.IsZero() {
				lastDate = song.Timestamp
				break
			}
		}
		titlesConcat, errLastFM = getAllTitles(titlesConcat, lastDate, user_id)
	}

	if errLastFM.Error != 0 {
		return nil, errors.New("An error has occurred. Please try again later.")
	}

	err = cacheSongs(user_id, songFile{titlesConcat})
	if err != nil {
		fmt.Println("Couldn't cache the songs:", err)
	}

	if len(titlesConcat) == 0 {
		err = errors.New("Failed to retrieve any play history. Did you type your username correctly?")
	}

	return titlesConcat, err

}

func getAllTitles(titles []Song, startTime time.Time, user_id string) (newTitles []Song, errLastFM lastFMError) {
	defer func() {
		if r := recover(); r != nil {
			errLastFM.Error = r.(int)
			errLastFM.Message = r.(string)
			newTitles = make([]Song, 0)
		}
	}()
	// try to do things with last.fm
	method := "user.getrecenttracks"
	api_key, key_success := os.LookupEnv("LASTFM_KEY")
	get_json := true
	if key_success == false {
		config, err := configRead.ReadConfig("config.json")
		if err != nil {
			panic("Couldn't read config or get env vars")
		} else {
			api_key = config.LastFmKey
		}
	}
	urlTime := "0"
	if !startTime.IsZero() {
		timeInt := startTime.UTC().Unix()
		if timeInt > 0 {
			urlTime = strconv.FormatInt(timeInt+1, 10)
		}
	}
	last_url := baseLastURI + "?method=" + method + "&user=" + user_id + "&api_key=" + api_key +
		"&limit=200" + "&from=" + urlTime
	if get_json {
		last_url += "&format=json"
	}
	resp, err := http.Get(last_url)
	var songsJSON []byte
	if err == nil {
		songsJSON, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Fatal("Couldn't read the body of the last.fm response")
		}
	} else {
		fmt.Println(err)
	}

	songs := SongsPage{}
	err = json.Unmarshal(songsJSON, &songs)
	// We don't want the currently playing track there. This checks for that.
	containsNowPlaying := false
	if len(songs.RecentTracks.Tracks) > 0 {
		if songs.RecentTracks.Tracks[0].Attributes != nil &&
			songs.RecentTracks.Tracks[0].Attributes["nowplaying"].(string) == "true" {
			containsNowPlaying = true
		}
	}
	if containsNowPlaying {
		songs.RecentTracks.Tracks = songs.RecentTracks.Tracks[1:]
	}
	topIndex := len(songs.RecentTracks.Tracks) - 1
	for i := topIndex; i >= 0; i-- {
		if topIndex-i != i {
			temp := songs.RecentTracks.Tracks[i]
			songs.RecentTracks.Tracks[i] = songs.RecentTracks.Tracks[topIndex-i]
			songs.RecentTracks.Tracks[topIndex-i] = temp
		}
	}
	pageSongs := make([]Song, 0, 50)

	for _, track := range songs.RecentTracks.Tracks {
		utime, err := strconv.ParseInt(track.Timestamp.UnixTime, 10, 64)
		var ts time.Time
		if err == nil {
			ts = time.Unix(utime, 0)
		}
		pageSongs = append(pageSongs, Song{track.Artist.Title, track.Title, ts})
	}

	max_page, _ := strconv.Atoi(songs.RecentTracks.Metadata.TotalPages)

	if max_page < 1 {
		max_page = 1
	}

	songPages := make([][]Song, max_page)

	songPages[0] = pageSongs

	batchAmt := 100

	if max_page > batchAmt { // have to batch to avoid socket overload
		for i := 0; i <= max_page/batchAmt; i++ {
			var maxBatch int
			if (i+1)*batchAmt > max_page {
				maxBatch = max_page
			} else {
				maxBatch = (i + 1) * batchAmt
			}
			if i == 0 {
				for j := 2; j <= maxBatch; j++ {
					pagesWg.Add(1)
					go getLastFMPagesAsync(last_url, j, maxBatch, songPages)
				}
			} else {
				for j := i*batchAmt + 1; j <= maxBatch; j++ {
					pagesWg.Add(1)
					go getLastFMPagesAsync(last_url, j, maxBatch, songPages)
				}
			}
			pagesWg.Wait()
		}
	} else {
		for i := 2; i <= max_page; i++ {
			pagesWg.Add(1)
			go getLastFMPagesAsync(last_url, i, max_page, songPages)
		}
		pagesWg.Wait()
	}

	// reversing all of the pages
	topIndex = len(songPages) - 1
	for i := topIndex; i >= 0; i-- {
		if topIndex-i != i {
			temp := songPages[i]
			songPages[i] = songPages[topIndex-i]
			songPages[topIndex-i] = temp
		}
	}
	if !startTime.IsZero() && len(titles) > 0 {
		// normal append for a new list
		for i := 0; i < max_page; i++ {
			titles = append(songPages[i], titles...)
		}
	} else {
		// if we're adding to the cache, have to prepend the new songs
		for i := 0; i < max_page; i++ {
			titles = append(titles, songPages[i]...)
		}
	}

	return titles, lastFMError{}
}

func getLastFMPagesAsync(url string, page int, max_page int, allTitles [][]Song) {
	defer pagesWg.Done()
	pageStr := strconv.Itoa(page)
	songs := SongsPage{}
	rateLimited := true
	tries := 0
	for rateLimited == true && tries < 4 {
		tr := &http.Transport{
			DisableKeepAlives: true,
		}
		c := &http.Client{Transport: tr, Timeout: 5 * time.Second}
		resp, err := c.Get(url + "&page=" + pageStr)
		if err == nil && resp.StatusCode == http.StatusOK {
			songsJSON, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			err = json.Unmarshal(songsJSON, &songs)
			if err != nil {
				rateLimited = true
				tries = tries + 1
			} else {
				rateLimited = false
			}
		}
	}

	var tracksRaw []track
	// Eliminate currently playing track if returned.
	containsNowPlaying := false
	if len(songs.RecentTracks.Tracks) > 0 {
		if songs.RecentTracks.Tracks[0].Attributes != nil &&
			songs.RecentTracks.Tracks[0].Attributes["nowplaying"].(string) == "true" {
			containsNowPlaying = true
		}
	}
	if containsNowPlaying {
		tracksRaw = songs.RecentTracks.Tracks[1:]
	} else {
		tracksRaw = songs.RecentTracks.Tracks
	}
	// Reverse the array so that the suffixes are built in the right order.
	topIndex := len(tracksRaw) - 1
	for i := topIndex; i >= 0; i-- {
		if topIndex-i != i {
			temp := tracksRaw[i]
			tracksRaw[i] = tracksRaw[topIndex-i]
			tracksRaw[topIndex-i] = temp
		}
	}
	titles := make([]Song, 0)
	for _, track := range tracksRaw {
		utime, err := strconv.ParseInt(track.Timestamp.UnixTime, 10, 64)
		var ts time.Time
		if err == nil {
			ts = time.Unix(utime, 0)
		}
		titles = append(titles, Song{track.Artist.Title, track.Title, ts})
	}
	allTitles[page-1] = titles
}