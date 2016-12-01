package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/snyderks/spotkov-web/handlers"
	"github.com/snyderks/spotkov/configRead"
)

func main() {
	config, err := configRead.ReadConfig("config.json")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req,
			"https://"+config.Hostname+config.TLSPort+req.URL.String(),
			http.StatusMovedPermanently)
	})
	go http.ListenAndServe(config.HTTPPort, mux)

	handlers.SetUpAPICalls()
	handlers.SetUpBasicHandlers()
	fmt.Println("Successfully initialized!")
	err = http.ListenAndServeTLS(config.TLSPort, config.CertPath, config.CertKeyPath, nil)
	log.Fatal(err)
}
