package mail

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// relay answers like an SMTP relay, and ends the data with end: a reply
// line, or "" to close the connection without one.
func relay(t *testing.T, end string) Config {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		r := bufio.NewReader(c)
		say := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
		say("220 ready")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch strings.ToUpper(strings.TrimSpace(line)) {
			case "DATA":
				say("354 go on")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
				}
				if end == "" {
					return
				}
				say(end)
			case "QUIT":
				say("221 bye")
				return
			default:
				say("250 ok")
			}
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	return Config{Host: host, Port: p, Security: "none", From: "gateway@example.com"}
}

// What the relay answers at the end of the data is what a queue reads: its
// code, or that the connection went before any answer.
func TestTheRelaysAnswerIsRead(t *testing.T) {
	msg := Message{To: []string{"alice@example.com"}, Subject: "x", Text: "x", MessageID: "ntf_1.alice@example.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Send(ctx, relay(t, "250 queued"), msg); err != nil {
		t.Fatalf("accepted: %v", err)
	}
	for end, want := range map[string]SendError{"451 4.3.0 later": {Code: 451}, "550 5.1.1 no": {Code: 550}, "": {Lost: true}} {
		err := Send(ctx, relay(t, end), msg)
		var se *SendError
		if !errors.As(err, &se) || se.Code != want.Code || se.Lost != want.Lost {
			t.Fatalf("%q: %#v", end, err)
		}
	}
	if b := string(Build("gateway@example.com", msg)); !strings.Contains(b, "Message-ID: <ntf_1.alice@example.com>\r\n") {
		t.Fatalf("no Message-ID: %s", b)
	}
}
