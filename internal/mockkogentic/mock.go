// Package mockkogentic is a local stand-in for the proposed Kogentic API,
// used by tests and by tools/mock-kogentic. It is not the real service.
package mockkogentic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Server records the reports it receives.
type Server struct {
	Key     string // the only accepted API key
	mu      sync.Mutex
	Reports [][]byte
}

// Received returns a copy of the raw report bodies.
func (s *Server) Received() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.Reports...)
}

func (s *Server) authorized(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+s.Key
}

// ServeHTTP implements GET /v1/me and POST /v1/rules-lint/reports.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/me":
		_ = json.NewEncoder(w).Encode(map[string]any{"user": "mock-user", "workspace": "mock-workspace", "scopes": []string{"read:profile", "write:lint"}})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/rules-lint/reports":
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
		if err != nil || !json.Valid(b) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.Reports = append(s.Reports, b)
		n := len(s.Reports)
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"report_id": fmt.Sprintf("rpt_%d", n), "url": fmt.Sprintf("http://%s/reports/rpt_%d", r.Host, n)})
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not found"}`)
	}
}
