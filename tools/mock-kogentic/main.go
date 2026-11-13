// Command mock-kogentic runs the local mock of the proposed Kogentic API so
// you can try --send-to-kogentic without any real service:
//
//	go run ./tools/mock-kogentic -addr 127.0.0.1:4810 -key kg_test_example
//	KOGENTIC_BASE_URL=http://127.0.0.1:4810 KOGENTIC_API_KEY=kg_test_example \
//	  kogentic-rules-lint lint --send-to-kogentic --yes
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4810", "listen address (loopback only)")
	key := flag.String("key", "kg_test_example", "API key the mock accepts")
	flag.Parse()
	s := &mockkogentic.Server{Key: *key}
	log.Printf("mock Kogentic API on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, logRequests(s)))
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
