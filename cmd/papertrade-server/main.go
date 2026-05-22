package main

import (
	"flag"
	"log"

	"github.com/felix/papertrading/internal/api"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	jm := api.NewJobManager()
	if err := api.Start(*addr, jm); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
