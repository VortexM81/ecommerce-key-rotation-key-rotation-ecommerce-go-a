package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/ecommerce-key-rotation/internal/infrai"
	"github.com/infrai-examples/ecommerce-key-rotation/rotation"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = infrai.DefaultBaseURL
	}
	client := infrai.NewClient(baseURL, key, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /rotations", func(w http.ResponseWriter, r *http.Request) {
		var request rotation.Request
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		result, err := rotation.Run(r.Context(), client, request)
		if err != nil {
			writeRotationError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("rotation service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeRotationError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 600 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "rotation request did not complete"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
