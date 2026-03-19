package main

import (
	"log"
	"net/http"

	"github.com/dmytrobereznii/web-crawler/internal/api"
)

func main() {
	h := api.NewHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /crawls", h.CreateCrawl)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
