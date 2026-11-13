package mockkogentic

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
)

// Test servers bind to 127.0.0.1 inside this fixed port range so local runs
// stay out of the way of other services.
const (
	PortFirst = 4800
	PortLast  = 4839
)

// Listen returns a loopback listener on the first free port in
// PortFirst..PortLast.
func Listen() (net.Listener, error) {
	var last error
	for p := PortFirst; p <= PortLast; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return l, nil
		}
		last = err
	}
	return nil, fmt.Errorf("no free port in %d-%d: %w", PortFirst, PortLast, last)
}

// NewServer starts an httptest server (TLS if tls is true) on a port from
// Listen. The caller must Close it.
func NewServer(h http.Handler, tls bool) (*httptest.Server, error) {
	l, err := Listen()
	if err != nil {
		return nil, err
	}
	srv := httptest.NewUnstartedServer(h)
	_ = srv.Listener.Close()
	srv.Listener = l
	if tls {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	return srv, nil
}

// Recorder counts every request it receives and keeps the Authorization
// headers and bodies, so tests can prove that nothing reached it.
type Recorder struct {
	mu     sync.Mutex
	Hits   int
	Auth   []string
	Bodies []string
}

// ServeHTTP records the request and answers 200 {}.
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	b, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.Hits++
	r.Auth = append(r.Auth, req.Header.Get("Authorization"))
	r.Bodies = append(r.Bodies, string(b))
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{}`)
}

// Count returns the number of requests received.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Hits
}

// Redirect answers every request with the given 3xx status and Location.
func Redirect(status int, location func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Location", location(req))
		w.WriteHeader(status)
	})
}
