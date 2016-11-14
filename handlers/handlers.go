package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/snyderks/spotkov/configRead"
	"github.com/snyderks/spotkov/lastFm"
	"github.com/snyderks/spotkov/markov"
	"github.com/zmb3/spotify"
)

// Page A basic page, with Body being an HTML doc.
type Page struct {
	Title string
	Body  []byte
}

const configLocation = "config.json"
const redirectURI = "http://localhost:8080/callback" // Put this in the config.

var client *spotify.Client
var config configRead.Config

var (
	scopes = []string{spotify.ScopeUserReadPrivate,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPrivate,
		spotify.ScopePlaylistModifyPublic}
	auth  = spotify.NewAuthenticator(redirectURI, scopes...)
	state = "abc123" // TODO: Make this a guid or something.
)

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
	http.Redirect(w, r, "http://localhost:8080/", http.StatusSeeOther)
}

// Basic load, render functions
func (p *Page) save() error {
	filename := p.Title + ".html"
	return ioutil.WriteFile(filename, p.Body, 0600) // 0600 = r/w for user
}

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
}

// SetUpBasicHandlers Create handler functions for path handlers
func SetUpBasicHandlers() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/assets/", assetsHandler)
	http.HandleFunc("/404/", notFoundHandler)
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
