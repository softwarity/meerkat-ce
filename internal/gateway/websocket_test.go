package gateway

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// wsGUID is the constant RFC 6455 makes both ends hash the key with.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B10"

// echoUpgrader is an upstream that speaks the WebSocket handshake by hand and
// then echoes raw bytes. No library: what is being tested is the GATEWAY, and
// a dependency would only add its own opinion about framing.
func echoUpgrader(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			http.Error(w, "not an upgrade: "+r.Header.Get("Upgrade"), http.StatusBadRequest)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "upstream cannot hijack", http.StatusInternalServerError)
			return
		}
		conn, rw, err := hj.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + wsGUID)) //nolint:gosec // RFC 6455 says sha1
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
		_ = rw.Flush()
		// Echo one line, which is enough to prove bytes cross in both
		// directions once the protocol has been switched.
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		_, _ = rw.WriteString("echo:" + line)
		_ = rw.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A WebSocket is how an application keeps a page alive, and it crosses the
// gateway as a protocol SWITCH rather than as a request and a response: the
// proxy has to pass the upgrade up, pass the 101 back down, and then get out
// of the way of the bytes. Nothing else in the router exercises that path -
// every other test ends when a body has been read.
func TestWebSocketUpgradeCrossesTheGateway(t *testing.T) {
	up := echoUpgrader(t)
	rt := newRouter(t, pathRoute("ws", "ws", 1, "/ws/**", up.URL))
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(srv.URL, "http://"), 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	const key = "dGhlIHNhbXBsZSBub25jZQ=="
	_, err = conn.Write([]byte("GET /ws/live HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(srv.URL, "http://") + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: " + key + "\r\n\r\n"))
	if err != nil {
		t.Fatalf("write handshake: %v", err)
	}

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("the gateway answered %q, want 101 Switching Protocols", strings.TrimSpace(status))
	}
	var accept string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "Sec-WebSocket-Accept") {
			accept = strings.TrimSpace(value)
		}
	}
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // RFC 6455 says sha1
	if want := base64.StdEncoding.EncodeToString(sum[:]); accept != want {
		t.Errorf("Sec-WebSocket-Accept = %q, want %q: the upstream's own answer must reach the client untouched", accept, want)
	}

	// And the tunnel is open in both directions, which is the whole point.
	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatalf("write after upgrade: %v", err)
	}
	got, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read after upgrade: %v", err)
	}
	if strings.TrimSpace(got) != "echo:ping" {
		t.Errorf("read %q back, want the upstream's echo", strings.TrimSpace(got))
	}
}

// The same switch on a UI route, which is where it could quietly stop working:
// those carry the response filters that inject the user button and stamp the
// page. They are guarded on the content type, so a 101 - which has none, and
// whose "body" IS the tunnel - goes through untouched. This pins that guard
// from the outside, since the day one filter reads a body it should not, the
// symptom is a page that never connects rather than a failing test.
func TestWebSocketCrossesAUIRouteWithItsInjections(t *testing.T) {
	up := echoUpgrader(t)
	r := pathRoute("ws-ui", "ws-ui", 1, "/ws/**", up.URL)
	r.IsUI = true
	r.UI = &store.RouteUI{
		UserButton: store.UserButton{Enabled: true},
		CustomCSS:  "body { color: red }",
	}
	rt := newRouter(t, r)
	srv := httptest.NewServer(rt)
	t.Cleanup(srv.Close)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(srv.URL, "http://"), 3*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write([]byte("GET /ws/live HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(srv.URL, "http://") + "\r\n" +
		"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"))
	if err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("a UI route answered %q, want 101: an injection swallowed the switch", strings.TrimSpace(status))
	}
}
