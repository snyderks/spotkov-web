package main

import (
	"fmt"
	"net/http"

	"github.com/snyderks/spotkov-web/handlers"
)

func main() {
	handlers.SetUpAPICalls()
	handlers.SetUpBasicHandlers()
	fmt.Println("Successfully initialized!")
	http.ListenAndServe(":8080", nil)
}
