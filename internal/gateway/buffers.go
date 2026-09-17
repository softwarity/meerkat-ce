package gateway

import "sync"

// proxyBuffers lends every reverse proxy the buffer a body is copied through.
//
// Without one, httputil.ReverseProxy makes a fresh 32 KB slice for EVERY
// response: at the 28,000 req/s one core carried in the benchmark (tools/bench)
// that is close to a gigabyte a second handed to the garbage collector, on the
// same core that is meant to be proxying. Lent from a pool instead, the same
// run reads 38,000. Traefik does the same.
//
// The pool holds fixed-size arrays rather than slices so that giving one back
// allocates nothing: a slice put in a sync.Pool escapes as a new header on
// every Put.
var proxyBuffers = &bufferPool{}

const proxyBufferSize = 32 << 10

type bufferPool struct{ pool sync.Pool }

func (b *bufferPool) Get() []byte {
	if buf, ok := b.pool.Get().(*[proxyBufferSize]byte); ok {
		return buf[:]
	}
	return new([proxyBufferSize]byte)[:]
}

func (b *bufferPool) Put(buf []byte) {
	// Only what was lent comes back: the proxy returns the slice it was given.
	if len(buf) == proxyBufferSize {
		b.pool.Put((*[proxyBufferSize]byte)(buf))
	}
}
