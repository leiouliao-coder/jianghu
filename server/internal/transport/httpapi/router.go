// Package httpapi exposes bootstrap diagnostics, not a game protocol.
package httpapi

import (
	"encoding/json"
	"net/http"
)

const Service = "jianghu-server"
const Version = "bootstrap"

// NewRouter uses exact paths and JSON responses, including errors.
func NewRouter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/version" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
			return
		}
		if r.URL.Path == "/healthz" {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": Service})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"service": Service, "version": Version})
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// Payloads are fixed string maps and cannot fail JSON serialization.
	_ = json.NewEncoder(w).Encode(payload)
}
