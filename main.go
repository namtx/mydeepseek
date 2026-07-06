package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           newServer(cfg).routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("fake ollama server listening on %s (upstream %s)", cfg.listenAddr, cfg.baseURL)
	log.Fatal(srv.ListenAndServe())
}
