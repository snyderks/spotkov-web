package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/snyderks/spotkov/configRead"
	"github.com/snyderks/spotkov/lastFm"
	"github.com/snyderks/spotkov/markov"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

// Page A basic page, with Body being an HTML doc.
type Page struct {
	Title string
	Body  []byte
}

type playlistRequest struct {
	Token          oauth2.Token `json:"token"`
	Length         int          `json:"length"`
	Title          string       `json:"title"`
	Artist         string       `json:"artist"`
	LastFmUsername string       `json:"lastFmUsername"`
}

const configLocation = "config.json"
const redirectURI = "http://localhost:8080/callback" // Put this in the config.

var config configRead.Config

var (
	scopes = []string{spotify.ScopeUserReadPrivate,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPrivate,
		spotify.ScopePlaylistModifyPublic}
	auth  = spotify.NewAuthenticator(redirectURI, scopes...)
	state = "abc123" // TODO: Make this a guid or something.
)

// Initialize the client.
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

func initializeClientWithToken(token oauth2.Token) (spotify.Client, error) {
	client := auth.NewClient(&token)
	return client, nil
}

// Path handlers
func assetsHandler(w http.ResponseWriter, r *http.Request) {
	loc := r.URL.Path[len("/assets/"):]
	f, err := ioutil.ReadFile("assets/" + loc)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	fmt.Fprintf(w, "%s", f)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	p, err := loadPage("index")
	if err != nil {
		fmt.Fprintf(w, "Error")
		return
	}
	fmt.Fprintf(w, "%s", p.Body)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/404/"):]
	renderTemplate(w, "notfound", &Page{Title: title})
}

// API calls (/api)
func spotifyLoginURLHandler(w http.ResponseWriter, r *http.Request) {
	type loginURL struct {
		URL string `json:"URL"`
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

func lastFmHandler(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Path[len("/api/getLastFMSongs/"):]
	songs := lastFm.ReadLastFMSongs(user)
	chain := markov.BuildChain(songs)
	genSongs, err := markov.GenerateSongList(20, lastFm.Song{Artist: "Oasis", Title: "Wonderwall"}, chain)
	if err == nil {
		songsJSON, jsonErr := json.Marshal(genSongs)
		if jsonErr == nil {
			w.Write(songsJSON)
		}
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

func createLastFmPlaylist(w http.ResponseWriter, r *http.Request) {
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		fmt.Println("couldn't read body")
		w.WriteHeader(500)
		return
	}
	req := playlistRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println("couldn't unmarshal", err)
		w.WriteHeader(500)
		return
	}
	_, err = initializeClientWithToken(req.Token)
	if err != nil {
		fmt.Println("couldn't get a client")
		w.WriteHeader(500)
		return
	}
	songs := lastFm.ReadLastFMSongs(req.LastFmUsername)
	list, err := markov.GenerateSongList(req.Length,
		lastFm.Song{Title: req.Title, Artist: req.Artist},
		markov.BuildChain(songs))
	if err != nil {
		fmt.Println("couldn't make the song list")
		w.WriteHeader(500)
		return
	}
	listJSON, err := json.Marshal(list)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(listJSON)
}

// Spotify handler
func spotifyAuthHandler(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	fmt.Println("Woo!")
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
	http.Redirect(w, r, "http://localhost:8080/auth?token="+string(tokJSON), http.StatusPermanentRedirect)
}

func spotifyAuthReceiver(w http.ResponseWriter, r *http.Request) {
	p, err := loadPage("index")
	if err != nil {
		w.WriteHeader(404)
		return
	}
	renderTemplate(w, "auth", p)
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
	http.HandleFunc("/api/getLastFMSongs/", lastFmHandler)
	http.HandleFunc("/api/spotifyLoginUrl/", spotifyLoginURLHandler)
	http.HandleFunc("/callback", spotifyAuthHandler)
	http.HandleFunc("/api/getSpotifyUser", spotifyUserHandler)
	http.HandleFunc("/api/getPlaylist", createLastFmPlaylist)
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
		panic("Couldn't read the config. Who wrote this horrific file?")
	}
	auth.SetAuthInfo(config.SpotifyKey, config.SpotifySecret)
}
