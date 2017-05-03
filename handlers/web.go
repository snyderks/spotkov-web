// Package handlers provides endpoints for the web client to request song, post
// playlists to Spotify, and authenticate the server to make requests
// on its behalf.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/snyderks/spotkov/configRead"
	"github.com/snyderks/spotkov/lastFm"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

// Page is a basic page, with Body being an HTML doc.
type Page struct {
	Title string
	Body  []byte
}

// playlistRequest is the expected format for a client request to generate
// a new playlist from Last.FM data.
type playlistRequest struct {
	Token          oauth2.Token `json:"token"`
	Length         string       `json:"length"`
	Title          string       `json:"title"`
	Artist         string       `json:"artist"`
	LastFmUsername string       `json:"lastFmUsername"`
}

// spotifyPlaylistCreation is the expected format for a client request
// to post a generated playlist to Spotify. Contains a token stored on
// the client to authenticate.
type spotifyPlaylistCreation struct {
	Token        oauth2.Token  `json:"token"`
	PlaylistName string        `json:"playlistName"`
	Songs        []lastFm.Song `json:"songs"`
}

// friendlyError is used to write back to the web client what went wrong
// in processing so more detailed errors can be sent.
type friendlyError struct {
	Error string
}

// configLocation is the location of the config
// (should be in the same directory as the application)
const configLocation = "config.json"

// redirectURI is the web address that Spotify redirects to
// on successful authentication. Server must be listening at this endpoint.
var redirectURI string

// config is the translated structure of the application's config file.
var config configRead.Config

// state is a randomly generated string appended to Spotify auth requests to
// help flag possible MITM.
var state string

var (
	// scopes is the required permissions given to the server when accessing
	// the user's Spotify acct.
	scopes = []string{spotify.ScopeUserReadPrivate,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPrivate,
		spotify.ScopePlaylistModifyPublic}
	// auth is an instance of the authenticator that is used when first
	// authorizing the application to a new user.
	auth spotify.Authenticator
)

// initializeClient initializes a Spotify client using an existing token embedded
// within a request.
func initializeClient(r *http.Request) (spotify.Client, error) {
	var storedToken []byte
	storedToken, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return spotify.Client{}, errors.New("Reading the token failed.")
	}
	token := oauth2.Token{}
	err = json.Unmarshal(storedToken, &token)
	if err != nil {
		return spotify.Client{}, errors.New("Unmarshaling the token failed.")
	}
	client := auth.NewClient(&token)
	return client, nil
}

// initializeClientWithToken initializes a Spotify client using an already-extracted token.
func initializeClientWithToken(token oauth2.Token) (spotify.Client, error) {
	client := auth.NewClient(&token)
	return client, nil
}

// assetsHandler is a catch-all for any static assets that the page needs,
// such as JS dependencies, images, CSS files, etc.
// Meant to be passed to AddHandler in an http server.
func assetsHandler(w http.ResponseWriter, r *http.Request) {
	loc := r.URL.Path[len("/assets/"):]
	f, err := ioutil.ReadFile("assets/" + loc)
	var contentType string
	if strings.HasSuffix(loc, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(loc, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(loc, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(loc, ".svg") {
		contentType = "image/svg+xml"
	} else {
		contentType = "text/plain"
	}
	if err != nil {
		w.WriteHeader(404)
		return
	}
	w.Header().Add("Content-Type", contentType)
	fmt.Fprintf(w, "%s", f)
}

// indexHandler serves up the landing page for the site. Meant to be passed to AddHandler in
// an http server.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	p, err := loadPage("index")
	if err != nil {
		fmt.Fprintf(w, "Error")
		return
	}
	fmt.Fprintf(w, "%s", p.Body)
}

// notFoundHandler is for serving a 404 page. Meant to be passed to AddHandler in an http
// server for a default.
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/404/"):]
	renderTemplate(w, "notfound", &Page{Title: title})
}

// Basic load, render functions
func loadPage(title string) (*Page, error) {
	filename := title + ".html"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

func renderTemplate(w http.ResponseWriter, templ string, p *Page) {
	t, _ := template.ParseFiles(templ + ".html")
	t.Execute(w, p)
}

// SetUpAPICalls creates handler functions for api calls
func SetUpAPICalls() {
	http.HandleFunc("/api/spotifyLoginUrl/", spotifyLoginURLHandler)
	http.HandleFunc("/callback", spotifyAuthHandler)
	http.HandleFunc("/api/getSpotifyUser", spotifyUserHandler)
	http.HandleFunc("/api/getPlaylist", createLastFmPlaylist)
	http.HandleFunc("/api/createPlaylist", postPlaylistToSpotify)
}

// SetUpBasicHandlers creates handler functions for path handlers
func SetUpBasicHandlers() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/assets/", assetsHandler)
	http.HandleFunc("/404/", notFoundHandler)
	http.HandleFunc("/auth", spotifyAuthReceiver)
}

// init sets up the Spotify authentication and reads the current configuration.
func init() {
	fmt.Println("Handlers initializing")
	var err error
	config, err = configRead.ReadConfig(configLocation)
	if err != nil {
		panic("Couldn't read the config. It's either not there or isn't in the correct format.")
	}
	redirectURI = config.AuthRedirectURL
	auth = spotify.NewAuthenticator(redirectURI, scopes...)
	auth.SetAuthInfo(config.SpotifyKey, config.SpotifySecret)
}
