package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

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
	svr := http.Server{
		Addr:           config.HTTPPort,
		Handler:        mux,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 8175, // if it's good enough for Apache, it's good enough for me
	}
	go svr.ListenAndServe()

	handlers.SetUpAPICalls()
	handlers.SetUpBasicHandlers()
	svrTLS := http.Server{
		Addr:           config.TLSPort,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 8175,
	}
	fmt.Println("Successfully initialized!")
	err = svrTLS.ListenAndServeTLS(config.CertPath, config.CertKeyPath)
	log.Fatal(err)
}
