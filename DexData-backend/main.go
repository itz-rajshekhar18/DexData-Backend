package main

import (
	"log"
	"net/http"

	"dex-workshop/backend/internal/app"
)

func main() {
	application, err := app.NewApplication()
	if err != nil {
		log.Fatalf("failed to initialize backend: %v", err)
	}

	log.Printf("backend listening on :%s", application.Port)
	if err := http.ListenAndServe(":"+application.Port, application.Handler); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
