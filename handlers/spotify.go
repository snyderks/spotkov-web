// Package handlers provides endpoints for the web client to request song, post
// playlists to Spotify, and authenticate the server to make requests
// on its behalf.
package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/snyderks/spotkov-web/randString"
	"github.com/snyderks/spotkov/spotifyPlaylistGenerator"
)

// Spotify handler
func spotifyAuthHandler(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
	}
	if st := r.FormValue("state"); st != state { // spotify returns the state key
		http.NotFound(w, r) // passed to make sure the call wasn't intercepted
		// currently takes down the application. This could be overkill.
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	tokJSON, err := json.Marshal(tok)
	if err != nil {
		w.Write(tokJSON)
	}
	http.Redirect(w, r, "//"+config.Hostname+"/auth?token="+string(tokJSON), http.StatusPermanentRedirect)
}

func spotifyAuthReceiver(w http.ResponseWriter, r *http.Request) {
	p, err := loadPage("auth")
	if err != nil {
		w.WriteHeader(404)
		return
	}
	fmt.Fprintf(w, "%s", p.Body)
}

func postPlaylistToSpotify(w http.ResponseWriter, r *http.Request) {
	maxBytes := 50000 // More than 50KB is almost certainly either hacked over the length or just spam.
	if r.ContentLength > int64(maxBytes) {
		return
	}
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Could not read requestBody. Detail:", err.Error())
		w.WriteHeader(400)
		e, err := json.Marshal(friendlyError{"Request couldn't be read. Please try again."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	req := spotifyPlaylistCreation{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Failed to parse requestBody. Detail:", err.Error())
		w.WriteHeader(400)
		e, err := json.Marshal(friendlyError{"Playlist was incorrectly formatted. Please try again."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	client, err := initializeClientWithToken(req.Token)
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Failed to initialize a client with the passed token. Detail:", err.Error())
		w.WriteHeader(400)
		e, err := json.Marshal(friendlyError{"Spotify authentication failed. Please try again."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	user, err := client.CurrentUser()
	// on failure, retry up to 10 times.
	// reasons for this can be found at https://github.com/snyderks/spotkov-web/issues/7
	for i := 0; err != nil && i < 10; i++ {
		user, err = client.CurrentUser()
	}
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Could not retrieve the current user. Detail:", err.Error())
		w.WriteHeader(400)
		e, err := json.Marshal(friendlyError{"Your profile couldn't be retrieved."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	spotifyPlaylistGenerator.CreatePlaylist(req.Songs, &client, user.ID)
	w.WriteHeader(200)
}

func spotifyUserHandler(w http.ResponseWriter, r *http.Request) {
	client, err := initializeClient(r)
	if err != nil {
		fmt.Println("could not initialize the client", err)
	}
	user, err := client.CurrentUser()
	if err != nil {
		fmt.Println("executing the client data failed", err)
		w.WriteHeader(500)
		e, err := json.Marshal(friendlyError{"Couldn't get your user data."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	userJSON, err := json.Marshal(user)
	if err != nil {
		fmt.Println("marshaling the response failed", err)
		w.WriteHeader(500)
		e, err := json.Marshal(friendlyError{"Failed to return your user data correctly. Please try again."})
		if err == nil {
			w.Write(e)
		}
		return
	}
	w.Write(userJSON)
	return
}

// spotifyLoginURLHandler
func spotifyLoginURLHandler(w http.ResponseWriter, r *http.Request) {
	type loginURL struct {
		URL string `json:"URL"`
	}
	var err error
	state, err = randString.GenerateRandomString(32)
	if err != nil {
		http.Error(w, "Failed to generate state.", http.StatusInternalServerError)
	}
	url := loginURL{URL: auth.AuthURL(state)}
	urlJSON, err := json.Marshal(url)
	if err == nil {
		w.Write(urlJSON)
	} else {
		w.WriteHeader(500)
		e, err := json.Marshal(friendlyError{"Couldn't generate the URL correctly."})
		if err == nil {
			w.Write(e)
		}
	}
}
