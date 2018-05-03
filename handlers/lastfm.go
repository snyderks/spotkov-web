// Package handlers provides endpoints for the web client to request song, post
// playlists to Spotify, and authenticate the server to make requests
// on its behalf.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/snyderks/spotkov/lastFm"
	"github.com/snyderks/spotkov/markov"
)

func createLastFmPlaylist(w http.ResponseWriter, r *http.Request) {
	// By accepting only POST requests, it prevents a possible XSS attack
	// where somehow a separate server could get playlist data.
	if r.Method != "POST" {
		w.WriteHeader(403)
		return
	}
	maxBytes := 4000 // NOTHING should be sending 4KB requests to this.
	if r.ContentLength > int64(maxBytes) {
		return
	}
	req, err := openPlaylistRequest(r)
	if err != nil {
		print("Couldn't open the playlist request. Error: ", err.Error())
		w.WriteHeader(400)
		e, err := json.Marshal(friendlyError{"Reading the playlist request failed."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	songs, err := lastFm.ReadLastFMSongs(req.LastFmUsername)
	if err != nil {
		w.WriteHeader(500)
		print("Couldn't read songs from Last.FM. Error: ", err.Error())
		w.Write([]byte("An error occurred. Please try again later."))
		return
	}

	list, err := getSongsForRequest(w, req, songs)
	if err != nil {
		fmt.Println("couldn't make the song list")
		w.WriteHeader(500)
		w.Write([]byte("Couldn't create the playlist. Try again."))
		return
	}
	listJSON, err := json.Marshal(list)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(listJSON)
}

func getSongsForRequest(w http.ResponseWriter, req playlistRequest, songs []lastFm.Song) ([]lastFm.Song, error) {
	length, err := strconv.Atoi(req.Length)
	// These lines prevent a number from being too large or too small.
	if err != nil {
		fmt.Println("couldn't convert length to int")
		w.WriteHeader(400)
		return nil, errors.New("Length passed was invalid. Atoi error: " + err.Error())
	}
	if length < 1 {
		length = 1
	}
	if length > 200 {
		length = 200
	}
	return markov.GenerateSongList(length, 1,
		lastFm.Song{Title: req.Title, Artist: req.Artist},
		markov.BuildChain(songs))
}

func openPlaylistRequest(r *http.Request) (playlistRequest, error) {
	maxBytes := 4000 // NOTHING should be sending 4KB requests to this.
	if r.ContentLength > int64(maxBytes) {
		return playlistRequest{}, errors.New("")
	}
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	err = r.Body.Close()
	if err != nil {
		return playlistRequest{}, errors.New("Failed to read the request.")
	}
	req := playlistRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println("couldn't unmarshal", err)
		return playlistRequest{}, errors.New("Couldn't parse the request.")
	}
	return req, nil
}
