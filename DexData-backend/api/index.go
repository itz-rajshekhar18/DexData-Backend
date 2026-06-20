package handler

import (
	"net/http"

	"dex-workshop/backend/internal/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	handler, err := app.CachedHandler()
	if err != nil {
		app.WriteStartupError(w, err)
		return
	}

	handler.ServeHTTP(w, r)
}
