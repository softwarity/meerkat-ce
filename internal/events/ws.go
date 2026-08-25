package events

import (
	"bufio"
	"crypto/sha1" //nolint:gosec // RFC 6455 names SHA-1 for the handshake: it is a fixed string, not a security claim.
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The WebSocket side of the channel, written here rather than pulled in.
//
// What we need is a server that pushes text frames and answers a ping: the
// browser is the only client, it sends nothing but control frames, and the
// payloads are small JSON objects. RFC 6455 for that fits in one file, and a
// gateway that authenticates people is the wrong place to add a dependency for
// what amounts to a SHA-1 of a header and a two-byte prefix. The same argument
// bought us the TOTP implementation, and for the same reason.
//
// What is deliberately NOT here: fragmentation of outgoing frames (ours are
// small), compression (permessage-deflate is not offered, so it is not
// negotiated), and reading client data (the channel is one-way by design - a
// browser that wants to say something has the rest of the HTTP API).

const (
	// The magic string RFC 6455 section 1.3 appends before hashing.
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

	opContinuation = 0x0
	opText         = 0x1
	opBinary       = 0x2
	opClose        = 0x8
	opPing         = 0x9
	opPong         = 0xA

	// closeNormal and friends: the codes we actually send.
	closeNormal        = 1000
	closeGoingAway     = 1001
	closeProtocolError = 1002
	closeTooBig        = 1009

	// maxClientFrame bounds what a client may send us. It has nothing to say,
	// so this is only here to stop a frame header from claiming a gigabyte.
	maxClientFrame = 4 << 10

	// pingEvery keeps the connection warm through proxies that cut idle ones,
	// and is how a half-open connection is discovered: the write fails, or the
	// pong never comes and the read deadline fires.
	pingEvery = 30 * time.Second
	// idleTimeout is generous relative to pingEvery: any frame refreshes it,
	// and a browser answers every ping.
	idleTimeout = 90 * time.Second
	// writeWait bounds a single frame write, so one stuck socket cannot pin a
	// goroutine forever.
	writeWait = 10 * time.Second
)

// ErrNotWebSocket says the request was not an upgrade. Kept apart because the
// caller usually wants to answer something friendlier than a protocol error.
var ErrNotWebSocket = errors.New("not a websocket upgrade")

// conn is one hijacked connection.
type conn struct {
	rwc net.Conn
	br  *bufio.Reader
	wmu sync.Mutex
}

// Serve upgrades the request and pumps the subscription until either end goes
// away. It owns the connection from the moment it returns nil error handling
// to the transport: the caller must not write to w afterwards.
//
// The subscription is closed on the way out, whichever pump ended first.
func Serve(w http.ResponseWriter, r *http.Request, sub *Subscription) error {
	defer sub.Close()

	c, err := accept(w, r)
	if err != nil {
		return err
	}
	defer func() { _ = c.rwc.Close() }()

	// The read pump exists for the control frames - a pong refreshes the
	// deadline, a close ends the connection - and to notice the browser
	// leaving. It never delivers anything upward.
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.readLoop()
	}()

	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case payload, ok := <-sub.Events():
			if !ok {
				// The hub dropped us, most likely because this page fell
				// behind. Say so properly: the browser reconnects and fetches
				// the state again, which is the whole recovery.
				c.writeClose(closeGoingAway)
				return nil
			}
			if err := c.write(opText, payload); err != nil {
				return err
			}
		case <-ping.C:
			if err := c.write(opPing, nil); err != nil {
				return err
			}
		case <-done:
			return nil
		case <-r.Context().Done():
			c.writeClose(closeNormal)
			return nil
		}
	}
}

// accept validates the handshake and takes the connection over. Every refusal
// answers on the HTTP response, since at that point there is still one.
func accept(w http.ResponseWriter, r *http.Request) (*conn, error) {
	// HTTP/2 has no hijack, and it does not need one: Go leaves RFC 8441
	// extended CONNECT off, so a browser opens a SECOND connection in
	// HTTP/1.1 for its WebSocket even when the page came over h2. Two
	// connections per tab is the expected shape, not a symptom.
	if r.ProtoMajor != 1 {
		http.Error(w, "the live channel speaks HTTP/1.1", http.StatusUpgradeRequired)
		return nil, ErrNotWebSocket
	}
	if !headerContains(r.Header, "Connection", "upgrade") ||
		!strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		http.Error(w, "this endpoint is a websocket", http.StatusUpgradeRequired)
		return nil, ErrNotWebSocket
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "only websocket version 13 is spoken here", http.StatusUpgradeRequired)
		return nil, ErrNotWebSocket
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if len(key) == 0 {
		http.Error(w, "the handshake is missing its key", http.StatusBadRequest)
		return nil, ErrNotWebSocket
	}
	// The same-origin check the browser does NOT do for us. A WebSocket is
	// exempt from CORS: without this, any page on the internet could open this
	// channel in a visitor's browser and read what it says. The cookie is
	// SameSite=Lax, which already stops most of it, but a sibling subdomain is
	// same-site, and that is exactly the neighbour worth refusing.
	if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
		http.Error(w, "this channel only opens from its own origin", http.StatusForbidden)
		return nil, ErrNotWebSocket
	}

	rwc, br, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, "the live channel is not available here", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijack: %w", err)
	}
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // see the import.
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"
	_ = rwc.SetWriteDeadline(time.Now().Add(writeWait))
	if _, err := io.WriteString(rwc, resp); err != nil {
		_ = rwc.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	_ = rwc.SetWriteDeadline(time.Time{})
	return &conn{rwc: rwc, br: br.Reader}, nil
}

// sameOrigin compares what the browser claims against the host it asked for.
// The scheme is not compared: a gateway reached in the clear and the same one
// over TLS are the same place, and the redirect already moves anyone who
// should not be in the clear.
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}

// headerContains answers for the comma-separated list headers: Connection may
// legitimately read "keep-alive, Upgrade".
func headerContains(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

// write sends one unmasked frame. Server frames are never masked (RFC 6455
// section 5.1), and ours are never fragmented.
func (c *conn) write(op byte, payload []byte) error {
	head := make([]byte, 0, 10)
	head = append(head, 0x80|op)
	switch n := len(payload); {
	case n < 126:
		head = append(head, byte(n))
	case n <= 0xFFFF:
		head = append(head, 126, byte(n>>8), byte(n))
	default:
		head = append(head, 127)
		head = binary.BigEndian.AppendUint64(head, uint64(n))
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := c.rwc.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	if _, err := c.rwc.Write(head); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.rwc.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// writeClose says goodbye properly. Failures are ignored on purpose: we are
// leaving either way, and the socket is about to be closed.
func (c *conn) writeClose(code int) {
	var body [2]byte
	binary.BigEndian.PutUint16(body[:], uint16(code))
	_ = c.write(opClose, body[:])
}

// readLoop consumes what the client sends. Nothing it sends is delivered
// upward: it answers pings, honours a close, and returns on anything it is not
// willing to read - which is what unblocks the writer.
func (c *conn) readLoop() {
	for {
		if err := c.rwc.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return
		}
		op, payload, err := c.readFrame()
		if err != nil {
			if errors.Is(err, errTooBig) {
				c.writeClose(closeTooBig)
			} else if errors.Is(err, errProtocol) {
				c.writeClose(closeProtocolError)
			}
			return
		}
		switch op {
		case opPing:
			if err := c.write(opPong, payload); err != nil {
				return
			}
		case opClose:
			c.writeClose(closeNormal)
			return
		case opPong, opText, opBinary, opContinuation:
			// A pong refreshed the deadline by arriving, which was its whole
			// job. Data frames are read and dropped: the channel is one-way,
			// and a client with something to say has the HTTP API.
		default:
			c.writeClose(closeProtocolError)
			return
		}
	}
}

var (
	errProtocol = errors.New("websocket protocol error")
	errTooBig   = errors.New("websocket frame too large")
)

// readFrame reads one frame from the client. Client frames are always masked
// (RFC 6455 section 5.1) - an unmasked one is a broken or hostile peer.
func (c *conn) readFrame() (byte, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(c.br, head[:]); err != nil {
		return 0, nil, err
	}
	op := head[0] & 0x0F
	if head[0]&0x70 != 0 { // reserved bits, and we negotiated no extension
		return 0, nil, errProtocol
	}
	if head[1]&0x80 == 0 {
		return 0, nil, errProtocol
	}
	n := int(head[1] & 0x7F)
	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		size := binary.BigEndian.Uint64(ext[:])
		if size > maxClientFrame {
			return 0, nil, errTooBig
		}
		n = int(size)
	}
	if n > maxClientFrame {
		return 0, nil, errTooBig
	}
	// A control frame carries at most 125 bytes and is never fragmented.
	if op >= opClose && (n > 125 || head[0]&0x80 == 0) {
		return 0, nil, errProtocol
	}
	var mask [4]byte
	if _, err := io.ReadFull(c.br, mask[:]); err != nil {
		return 0, nil, err
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return op, payload, nil
}
