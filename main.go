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
	config, err := configRead.Read("config.json")
	if err != nil {
		log.Fatal(err)
	}
	handlers.SetUpAPICalls()
	handlers.SetUpBasicHandlers()
	svr := http.Server{
		Addr:           config.HTTPPort,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 8175, // if it's good enough for Apache, it's good enough for me
	}
	fmt.Println("Serving", config.Hostname, "on", config.HTTPPort)
	svr.ListenAndServe()
}
