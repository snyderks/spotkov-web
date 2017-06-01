package handlers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"encoding/json"

	"strings"

	"github.com/snyderks/spotkov/lastFm"
	"github.com/xrash/smetrics"
)

type distance struct {
	Key    lastFm.BaseSong
	Amount int
}

type distances []distance

func (d distances) Len() int           { return len(d) }
func (d distances) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d distances) Less(i, j int) bool { return d[i].Amount < d[j].Amount }

type matchRequest struct {
	S      string `json:"s"`
	UserID string `json:"userID"`
}

type matchResponse struct {
	Num     int               `json:"num"`
	Matches []lastFm.BaseSong `json:"matches"`
}

func bestMatches(s string, c map[lastFm.BaseSong]bool, matches int, useTitles bool) matchResponse {
	var d distances
	for key := range c {
		if len(key.Title) > 0 {
			var cmp string
			if useTitles {
				cmp = key.Title
			} else {
				cmp = key.Artist
			}
			if strings.Contains(cmp, s) {
				d = append(d, distance{Key: key, Amount: smetrics.WagnerFischer(s, cmp, 1, 1, 2)})
			}
		}
	}
	sort.Sort(d)
	var ret []lastFm.BaseSong
	for i, el := range d {
		if i >= matches {
			break
		}
		ret = append(ret, el.Key)
	}
	return matchResponse{len(ret), ret}
}

func autocomplete(req matchRequest, useTitles bool) (matchResponse, error) {
	songs := lastFm.SongMap{}
	songs.Songs = make(map[lastFm.BaseSong]bool)
	err := lastFm.ReadCachedUniqueSongs(req.UserID, &songs)
	if err != nil {
		return matchResponse{}, err
	}
	matches := bestMatches(req.S, songs.Songs, 10, useTitles)
	return matches, nil
}

func autocompleteSongHandler(w http.ResponseWriter, r *http.Request) {
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	err = r.Body.Close()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := matchRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	matches, err := autocomplete(req, true)
	resp, err := json.Marshal(matches)
	w.Write(resp)
}

func autocompleteArtistHandler(w http.ResponseWriter, r *http.Request) {
	var requestBody []byte
	requestBody, err := ioutil.ReadAll(r.Body)
	err = r.Body.Close()

	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	req := matchRequest{}
	err = json.Unmarshal(requestBody, &req)
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	matches, err := autocomplete(req, false)
	resp, err := json.Marshal(matches)
	w.Write(resp)
}
