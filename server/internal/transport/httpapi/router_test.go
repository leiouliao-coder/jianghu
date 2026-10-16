package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRouter(t *testing.T) {
	tests := []struct {
		method, path string
		status       int
		want         map[string]string
	}{
		{"GET", "/healthz", 200, map[string]string{"status": "ok", "service": "jianghu-server"}},
		{"GET", "/healthz?probe=1", 200, map[string]string{"status": "ok", "service": "jianghu-server"}},
		{"GET", "/version", 200, map[string]string{"version": "bootstrap", "service": "jianghu-server"}},
		{"POST", "/healthz", 405, map[string]string{"error": "method_not_allowed"}},
		{"DELETE", "/version", 405, map[string]string{"error": "method_not_allowed"}},
		{"HEAD", "/healthz", 405, map[string]string{"error": "method_not_allowed"}},
		{"OPTIONS", "/version", 405, map[string]string{"error": "method_not_allowed"}},
		{"GET", "/", 404, map[string]string{"error": "not_found"}},
		{"GET", "/healthz/", 404, map[string]string{"error": "not_found"}},
		{"POST", "/unknown", 404, map[string]string{"error": "not_found"}},
	}
	handler := NewRouter()
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.status {
				t.Fatalf("status=%d want=%d", rec.Code, tt.status)
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Fatal("not JSON")
			}
			if tt.status == http.StatusMethodNotAllowed && rec.Header().Get("Allow") != "GET" {
				t.Fatal("missing Allow")
			}
			var got map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("body=%v want=%v", got, tt.want)
			}
		})
	}
}
