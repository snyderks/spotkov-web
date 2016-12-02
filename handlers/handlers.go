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

// Page A basic page, with Body being an HTML doc.
type Page struct {
	Title string
	Body  []byte
}

type playlistRequest struct {
	Token          oauth2.Token `json:"token"`
	Length         string       `json:"length"`
	Title          string       `json:"title"`
	Artist         string       `json:"artist"`
	LastFmUsername string       `json:"lastFmUsername"`
}

type spotifyPlaylistCreation struct {
	Token        oauth2.Token  `json:"token"`
	PlaylistName string        `json:"playlistName"`
	Songs        []lastFm.Song `json:"songs"`
}

const configLocation = "config.json"

var redirectURI string
var config configRead.Config
var state string

var (
	scopes = []string{spotify.ScopeUserReadPrivate,
		spotify.ScopePlaylistReadPrivate,
		spotify.ScopePlaylistModifyPrivate,
		spotify.ScopePlaylistModifyPublic}
	auth spotify.Authenticator
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
	var err error
	state, err = randString.GenerateRandomString(32)
	if err != nil {
		http.Error(w, "Failed to generate state. Something went wrong or something is vulnerable.", http.StatusInternalServerError)
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
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		w.WriteHeader(400)
		return
	}
	req := playlistRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println("couldn't unmarshal", err)
		w.WriteHeader(400)
		return
	}
	songs, err := lastFm.ReadLastFMSongs(req.LastFmUsername)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	length, err := strconv.Atoi(req.Length)
	// These lines prevent a number from being too large or too small.
	if length < 1 {
		length = 1
	}
	if length > 200 {
		length = 200
	}
	if err != nil {
		fmt.Println("couldn't convert length to int")
		w.WriteHeader(400)
		return
	}
	list, err := markov.GenerateSongList(length,
		lastFm.Song{Title: req.Title, Artist: req.Artist},
		markov.BuildChain(songs))
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
		w.WriteHeader(400)
		return
	}
	req := spotifyPlaylistCreation{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	client, err := initializeClientWithToken(req.Token)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	user, err := client.CurrentUser()
	if err != nil {
		w.WriteHeader(400)
		return
	}
	spotifyPlaylistGenerator.CreatePlaylist(req.Songs, &client, user.ID)
	w.WriteHeader(200)
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
	http.Redirect(w, r, "https://"+config.Hostname+config.HTTPPort+"/auth?token="+string(tokJSON), http.StatusPermanentRedirect)
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
