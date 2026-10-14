package certs

import (
	"context"
	"net"
	"net/http"
	"testing"
)

// A plain door switched off listens on the loopback only, and back on, on
// every address again (SSL-15).
func TestAPlainDoorSwitchedOffKeepsTheLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	_ = ln.Close()
	p := NewPlain("test", ":"+port, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })
	get := func() int {
		res, err := http.Get("http://127.0.0.1:" + port + "/")
		if err != nil {
			return 0
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	if err := p.Switch(true); err != nil || get() != 0 {
		t.Fatal("a door main has not opened stays closed through a reload")
	}
	for _, loopback := range []bool{false, true, false, true} {
		if err := p.Open(loopback); err != nil {
			t.Fatalf("open (loopback %v): %v", loopback, err)
		}
		if p.Loopback() != loopback || get() != http.StatusNoContent {
			t.Fatalf("loopback %v: reported %v, answered %d", loopback, p.Loopback(), get())
		}
		if loopback && p.ln.Addr().(*net.TCPAddr).IP.String() != "127.0.0.1" {
			t.Fatalf("switched off, it listens on %s", p.ln.Addr())
		}
	}
}
