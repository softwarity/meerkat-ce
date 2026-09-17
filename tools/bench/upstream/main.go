// Command upstream is the service every gateway of the benchmark stands in
// front of, and the fastest one that can be written: a fixed body, no work,
// no allocation worth the name. The gateway is what is measured, so anything
// the upstream costs is noise in its figure - and a direct call to it is the
// floor every overhead is read against.
//
// It is also the EXTERNAL authentication service of the "delegated" scenario
// (Traefik's ForwardAuth): /_auth answers 200 to the expected bearer and 401
// to anything else. Being the fastest possible decision makes that scenario a
// lower bound - a real oauth2-proxy, Authelia or identity provider only adds
// to the hop it measures.
package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	expected := "Bearer " + os.Getenv("BENCH_TOKEN")
	body := []byte(`{"ok":true}`)

	mux := http.NewServeMux()
	mux.HandleFunc("/_auth", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("BENCH_TOKEN") == "" || r.Header.Get("Authorization") != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})

	srv := &http.Server{Addr: ":9000", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
