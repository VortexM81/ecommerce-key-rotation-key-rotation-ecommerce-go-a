package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/infrai-examples/ecommerce-key-rotation/internal/infrai"
)

func TestWriteRotationErrorPreservesInfraiServerError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeRotationError(recorder, &infrai.APIError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "try again later",
		HTTPStatus: http.StatusServiceUnavailable,
	})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "SERVICE_UNAVAILABLE" || body["message"] != "try again later" {
		t.Fatalf("got body %#v", body)
	}
}

func TestWriteRotationErrorUsesBadGatewayForNonAPIError(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeRotationError(recorder, errors.New("connection failed"))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadGateway)
	}
}
