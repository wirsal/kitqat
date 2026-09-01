package api

import (
	"log"
	"net/http"
	"time"
)

// NewServer builds the HTTP server exposing KitQAt's three use cases
// (download test cases, update scenario, set done) as JSON/file endpoints.
func NewServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/test-cases/download", handleDownload)
	mux.HandleFunc("POST /api/v1/scenarios/update", handleUpdateScenario)
	mux.HandleFunc("POST /api/v1/jobs/set-done", handleSetDone)

	return &http.Server{
		Addr:         addr,
		Handler:      logRequests(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}
}

// logRequests logs method, path, and status only — request bodies carry
// credentials and must never be logged.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d", r.Method, r.URL.Path, sw.status)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}
