package events

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // the handshake's own hash, see ws.go.
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// A hand-written client, because the server is hand-written: the tests have to
// mask their frames and read unmasked ones, which is the same code path being
// asserted from the other end.

func dial(t *testing.T, server string, extra http.Header) (net.Conn, *bufio.Reader, *http.Response) {
	t.Helper()
	u, err := url.Parse(server)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	key := base64.StdEncoding.EncodeToString(nonce[:])
	req, _ := http.NewRequest(http.MethodGet, server, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	for k, vs := range extra {
		req.Header[k] = vs
	}
	if err := req.Write(c); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // see above.
		if got := resp.Header.Get("Sec-WebSocket-Accept"); got != base64.StdEncoding.EncodeToString(sum[:]) {
			t.Fatalf("the handshake did not prove it read our key: %q", got)
		}
	}
	return c, br, resp
}

// readFrame reads one server frame. Server frames are never masked.
func readFrame(t *testing.T, c net.Conn, br *bufio.Reader) (byte, []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var head [2]byte
	if _, err := io.ReadFull(br, head[:]); err != nil {
		t.Fatal(err)
	}
	if head[1]&0x80 != 0 {
		t.Fatal("the server masked a frame")
	}
	n := int(head[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			t.Fatal(err)
		}
		n = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(br, ext[:]); err != nil {
			t.Fatal(err)
		}
		n = int(binary.BigEndian.Uint64(ext[:]))
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(br, payload); err != nil {
		t.Fatal(err)
	}
	return head[0] & 0x0F, payload
}

func writeFrame(t *testing.T, c net.Conn, op byte, payload []byte) {
	t.Helper()
	var mask [4]byte
	_, _ = rand.Read(mask[:])
	frame := []byte{0x80 | op, 0x80 | byte(len(payload))}
	frame = append(frame, mask[:]...)
	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}
	if _, err := c.Write(frame); err != nil {
		t.Fatal(err)
	}
}

// channel stands up a server that hands every caller the same subscription's
// topics, and returns the hub to publish on.
func channel(t *testing.T, topics ...string) (*Hub, *httptest.Server) {
	t.Helper()
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := h.Subscribe(topics...)
		if err != nil {
			http.Error(w, "full", http.StatusServiceUnavailable)
			return
		}
		_ = Serve(w, r, sub)
	}))
	t.Cleanup(srv.Close)
	return h, srv
}

// The whole point, end to end: a published message comes out of a real socket
// as a text frame carrying the envelope.
func TestAPublishedMessageArrivesOnTheSocket(t *testing.T) {
	h, srv := channel(t, TopicAll)
	c, br, resp := dial(t, srv.URL, nil)
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake: %s", resp.Status)
	}
	// Wait for the subscription to exist before publishing: the handler runs
	// on its own goroutine, and a message published into an empty hub is
	// simply not sent.
	for range 100 {
		if h.Count() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.Publish(TopicAll, Message{Type: "served", Data: map[string]string{"name": "checkout"}})

	op, payload := readFrame(t, c, br)
	if op != opText {
		t.Fatalf("opcode = %#x", op)
	}
	if got := string(payload); got != `{"type":"served","data":{"name":"checkout"}}` {
		t.Fatalf("payload = %s", got)
	}
}

// A ping is answered with its own payload, which is what keeps a connection
// alive through the middleboxes that cut idle ones.
func TestAPingIsAnswered(t *testing.T) {
	_, srv := channel(t, TopicAll)
	c, br, _ := dial(t, srv.URL, nil)
	writeFrame(t, c, opPing, []byte("still there"))
	op, payload := readFrame(t, c, br)
	if op != opPong || string(payload) != "still there" {
		t.Fatalf("op=%#x payload=%q", op, payload)
	}
}

// The same-origin check the browser does not do for a WebSocket. Without it,
// any page on the internet could open this channel in a visitor's browser.
func TestAnotherOriginIsRefused(t *testing.T) {
	_, srv := channel(t, TopicAll)
	_, _, resp := dial(t, srv.URL, http.Header{"Origin": {"https://elsewhere.example"}})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign origin got %s", resp.Status)
	}
	// Its own origin still opens, port and all.
	u, _ := url.Parse(srv.URL)
	_, _, ok := dial(t, srv.URL, http.Header{"Origin": {"http://" + u.Host}})
	if ok.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("the page's own origin got %s", ok.Status)
	}
}

// A plain GET is not an error worth a stack trace: it is a client that does
// not speak the protocol, and it is told which one to speak.
func TestAPlainRequestIsTurnedAway(t *testing.T) {
	_, srv := channel(t, TopicAll)
	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("a plain GET got %s", resp.Status)
	}
}

// A frame claiming more than we are willing to read ends the connection with
// the code that names why, instead of allocating what it asked for.
func TestAnOversizedFrameEndsTheConnection(t *testing.T) {
	_, srv := channel(t, TopicAll)
	c, br, _ := dial(t, srv.URL, nil)
	// A masked binary frame announcing a gigabyte, and no payload behind it.
	head := []byte{0x82, 0xFF}
	head = binary.BigEndian.AppendUint64(head, 1<<30)
	head = append(head, 0, 0, 0, 0)
	if _, err := c.Write(head); err != nil {
		t.Fatal(err)
	}
	op, payload := readFrame(t, c, br)
	if op != opClose {
		t.Fatalf("opcode = %#x", op)
	}
	if code := binary.BigEndian.Uint16(payload); code != closeTooBig {
		t.Fatalf("close code = %d", code)
	}
}

// A page served over HTTP/2 still gets its socket - on a SECOND connection, in
// HTTP/1.1, which is the shape Go leaves in place by keeping RFC 8441 extended
// CONNECT off. What has to hold is that one port answers both: h2 is told
// which protocol to speak rather than hanging on a hijack it cannot do, and an
// HTTP/1.1 handshake on that very port opens.
func TestASocketOpensNextToHTTP2(t *testing.T) {
	h := NewHub()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, err := h.Subscribe(TopicAll)
		if err != nil {
			http.Error(w, "full", http.StatusServiceUnavailable)
			return
		}
		_ = Serve(w, r, sub)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.Proto != "HTTP/2.0" {
		t.Fatalf("the test did not even reach h2: %s", resp.Proto)
	}
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("an h2 request got %s", resp.Status)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "HTTP/1.1") {
		t.Fatalf("the refusal must name the protocol to speak, got %q", body)
	}

	// The same port, over TLS, in HTTP/1.1: the handshake goes through. The
	// client config comes from the test server, so nothing is skipped here -
	// only the protocol list changes.
	cfg := srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	cfg.NextProtos = []string{"http/1.1"}
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := tls.Dial("tcp", u.Host, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var nonce [16]byte
	_, _ = rand.Read(nonce[:])
	key := base64.StdEncoding.EncodeToString(nonce[:])
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	if err := req.Write(c); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(c)
	ws, err := http.ReadResponse(br, req)
	if err != nil {
		t.Fatal(err)
	}
	if ws.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("the HTTP/1.1 handshake on the h2 port got %s", ws.Status)
	}
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // see ws.go.
	if got := ws.Header.Get("Sec-WebSocket-Accept"); got != base64.StdEncoding.EncodeToString(sum[:]) {
		t.Fatalf("the handshake did not prove it read our key: %q", got)
	}
	h.Publish(TopicAll, Message{Type: "hello"})
	op, payload := readFrame(t, c, br)
	if op != opText || !strings.Contains(string(payload), "hello") {
		t.Fatalf("frame %#x %q", op, payload)
	}
}
