package certs

import "testing"

// Every status change is told, so the TLS screen hears it on the live channel
// instead of asking every few seconds (SSL-05); a claim refused changes nothing
// and says nothing.
func TestIssueBookTellsEveryMove(t *testing.T) {
	n := 0
	b := &issueBook{moved: func() { n++ }}
	if !b.claim("a|x") || n != 1 {
		t.Fatalf("claim: told %d times, want 1", n)
	}
	if b.claim("a|x") || n != 1 {
		t.Fatalf("second claim: told %d times, want still 1", n)
	}
	b.set("a|x", IssueStatus{State: IssueFailed}, false)
	b.forget("a|x")
	if n != 3 {
		t.Fatalf("set and forget: told %d times, want 3", n)
	}
}
