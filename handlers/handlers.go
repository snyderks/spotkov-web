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
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/snyderks/spotkov-web/randString"
	"github.com/snyderks/spotkov/configRead"
	"github.com/snyderks/spotkov/lastFm"
	"github.com/snyderks/spotkov/markov"
	"github.com/snyderks/spotkov/spotifyPlaylistGenerator"
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

// spotifyLoginURLHandler
func spotifyLoginURLHandler(w http.ResponseWriter, r *http.Request) {
	type loginURL struct {
		URL string `json:"URL"`
	}
	var err error
	state, err = randString.GenerateRandomString(32)
	if err != nil {
		http.Error(w, "Failed to generate state. Something went wrong "+
			"or something is vulnerable.", http.StatusInternalServerError)
	}
	url := loginURL{URL: auth.AuthURL(state)}
	urlJSON, err := json.Marshal(url)
	if err == nil {
		w.Write(urlJSON)
	} else {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
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
		return
	}
	userJSON, err := json.Marshal(user)
	if err != nil {
		fmt.Println("marshaling the response failed", err)
		w.WriteHeader(500)
		return
	}
	w.Write(userJSON)
	return
}

func openPlaylistRequest(r *http.Request) (playlistRequest, error) {
	maxBytes := 4000 // NOTHING should be sending 4KB requests to this.
	if r.ContentLength > int64(maxBytes) {
		return playlistRequest{}, errors.New("Content length was over max. Possible test of a vulnerability.")
	}
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	err = r.Body.Close()
	if err != nil {
		return playlistRequest{}, errors.New("Failed to read the request body. Read error:" + err.Error())
	}
	req := playlistRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println("couldn't unmarshal", err)
		return playlistRequest{}, errors.New("Couldn't parse the request. Unmarshal error: " + err.Error())
	}
	return req, nil
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
	}
	songs, err := lastFm.ReadLastFMSongs(req.LastFmUsername)
	if err != nil {
		w.WriteHeader(500)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			print("Couldn't write the error back to the client. Base error: ", err.Error())
		}
	}

	list, err := getSongsForRequest(w, req, songs)
	if err != nil {
		fmt.Println("couldn't make the song list")
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	listJSON, err := json.Marshal(list)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(listJSON)
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
		return
	}
	req := spotifyPlaylistCreation{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Failed to parse requestBody. Detail:", err.Error())
		w.WriteHeader(400)
		return
	}
	client, err := initializeClientWithToken(req.Token)
	if err != nil {
		print("ERROR (postPlaylistToSpotify): Failed to initialize a client with the passed token. Detail:", err.Error())
		w.WriteHeader(400)
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
		return
	}
	spotifyPlaylistGenerator.CreatePlaylist(req.Songs, &client, user.ID)
	w.WriteHeader(200)
}

// Spotify handler
func spotifyAuthHandler(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
	}
	if st := r.FormValue("state"); st != state { // spotify returns the state key
		http.NotFound(w, r) // passed to make sure the call wasn't intercepted
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

// SetUpAPICalls Create handler functions for api calls
func SetUpAPICalls() {
	http.HandleFunc("/api/spotifyLoginUrl/", spotifyLoginURLHandler)
	http.HandleFunc("/callback", spotifyAuthHandler)
	http.HandleFunc("/api/getSpotifyUser", spotifyUserHandler)
	http.HandleFunc("/api/getPlaylist", createLastFmPlaylist)
	http.HandleFunc("/api/createPlaylist", postPlaylistToSpotify)
}

// SetUpBasicHandlers Create handler functions for path handlers
func SetUpBasicHandlers() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/assets/", assetsHandler)
	http.HandleFunc("/404/", notFoundHandler)
	http.HandleFunc("/auth", spotifyAuthReceiver)
}

// Initial setup.
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
