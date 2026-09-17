// Command goproxy is the benchmark's REFERENCE, not a competitor: the Go
// standard library's reverse proxy with nothing around it - no route table,
// no identity, no metrics, no filters - on the transport settings Meerkat uses
// (a pool of idle upstream connections, a lent copy buffer).
//
// It answers the question a comparison with nginx-based gateways leaves open:
// how much of the gap is Meerkat's own code, and how much is Go's HTTP stack.
// Kong and APISIX are nginx and LuaJIT, an event loop in C tuned for twenty
// years; Meerkat is net/http, which is what gives it HTTP/2, gRPC trailers,
// WebSockets and TLS from one audited library. Whatever Meerkat reads close to
// this figure, it reads close to the ceiling of the language - and going past
// it would mean leaving net/http, not tuning Meerkat.
package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

const bufferSize = 32 << 10

type buffers struct{ pool sync.Pool }

func (b *buffers) Get() []byte {
	if buf, ok := b.pool.Get().(*[bufferSize]byte); ok {
		return buf[:]
	}
	return new([bufferSize]byte)[:]
}

func (b *buffers) Put(buf []byte) {
	if len(buf) == bufferSize {
		b.pool.Put((*[bufferSize]byte)(buf))
	}
}

func main() {
	target, err := url.Parse("http://upstream:9000")
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.BufferPool = &buffers{}
	proxy.Transport = &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 256,
		IdleConnTimeout:     55 * time.Second,
	}

	mux := http.NewServeMux()
	mux.Handle("/proxy/", proxy)
	srv := &http.Server{Addr: ":8000", Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
