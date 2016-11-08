package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/snyderks/spotkov/configRead"
)

type Page struct {
	Title string
	Body  []byte
}

const configLocation = "config.json"
const templateLocation = "./templates"

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

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Path[len("/404/"):]
	renderTemplate(w, "notfound", &Page{Title: title})
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	p, err := loadPage("index")
	if err != nil {
		fmt.Fprintf(w, "Error")
		return
	}
	fmt.Fprintf(w, "%s", p.Body)
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	loc := r.URL.Path[len("/assets/"):]
	f, err := ioutil.ReadFile("assets/" + loc)
	if err != nil {
		w.WriteHeader(404)
		return
	}
	fmt.Fprintf(w, "%s", f)
}

func setUpApiCalls() {
	http.HandleFunc("/api/getLastFMSongs/", lastFmHandler)
}

func lastFmHandler(w http.ResponseWriter, r *http.Request) {
	//user := r.URL.Path[len("/api/getLastFMSongs/"):]
	/*songs := lastFm.ReadLastFMSongs(user)
	chain := markov.BuildChain(songs)
	genSongs := markov.GenerateSongList(20, lastFm.Song{Artist: "Artist", Title: "Title"}, chain)
	songsJSON, _ := json.Marshal(genSongs)
	w.Write(songsJSON)*/
}

func renderTemplate(w http.ResponseWriter, templ string, p *Page) {
	t, _ := template.ParseFiles(templ + ".html")
	t.Execute(w, p)
}

func main() {
	_, err := configRead.ReadConfig(configLocation)
	if err != nil {
		fmt.Println("Could not read configuration file. Quitting...")
		return
	}
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/assets/", assetsHandler)
	http.HandleFunc("/404/", notFoundHandler)
	setUpApiCalls()
	http.ListenAndServe(":8080", nil)
}
