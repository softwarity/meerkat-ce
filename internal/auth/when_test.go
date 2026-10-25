package auth

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The profile's sessions and sign-in history write their times as the console
// does: with no zone chosen, the server's text is UTC with its Z and the page
// writes it again in the browser's zone (data-local, mkWhen); with one chosen,
// the server writes it in that zone and nothing is rewritten.
func TestProfileTimesFollowTheBrowserUntilAZoneIsChosen(t *testing.T) {
	mux, _, st := mfaSetup(t)
	rec := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	cookie := sessionCookieOf(rec)
	if cookie == nil {
		t.Fatalf("login: %d", rec.Code)
	}
	unchosen := regexp.MustCompile(`<time datetime="(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ)" data-local="short">(\d{4}-\d\d-\d\dT\d\d:\d\dZ)</time>`)
	for _, path := range []string{"/profile/sessions", "/profile/history"} {
		body := bodyString(do(t, mux, "GET", path, nil, cookie))
		m := unchosen.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("%s: no time to be written in the browser's zone:\n%s", path, body)
		}
		if at, _ := time.Parse(time.RFC3339, m[1]); at.UTC().Format("2006-01-02T15:04")+"Z" != m[2] {
			t.Errorf("%s: the text %q is not the instant %q in UTC", path, m[2], m[1])
		}
		for _, want := range []string{"window.mkWhen", "time[data-local]"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: the page lacks %q, the time stays in UTC", path, want)
			}
		}
	}

	if err := st.SetUserTimezone(context.Background(), "u1", "Asia/Saigon"); err != nil {
		t.Fatal(err)
	}
	chosen := regexp.MustCompile(`<time datetime="(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ)">(\d{4}-\d\d-\d\dT\d\d:\d\d)</time>`)
	saigon, err := time.LoadLocation("Asia/Saigon")
	if err != nil {
		t.Skip("no zone database on this machine")
	}
	for _, path := range []string{"/profile/sessions", "/profile/history"} {
		body := bodyString(do(t, mux, "GET", path, nil, cookie))
		if strings.Contains(body, `data-local=`) {
			t.Errorf("%s: a chosen zone is written by the server, not rewritten:\n%s", path, body)
		}
		m := chosen.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("%s: no time in the chosen zone:\n%s", path, body)
		}
		if at, _ := time.Parse(time.RFC3339, m[1]); at.In(saigon).Format("2006-01-02T15:04") != m[2] {
			t.Errorf("%s: the text %q is not the instant %q in Asia/Saigon", path, m[2], m[1])
		}
	}
}

func TestAMomentIsWrittenOneWay(t *testing.T) {
	at := time.Date(2026, 10, 25, 10, 41, 30, 0, time.UTC).Unix()
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no zone database on this machine")
	}
	for _, c := range []struct {
		m    moment
		want string
	}{
		{momentOf(at, nil), `<time datetime="2026-10-25T10:41:30Z" data-local="short">2026-10-25T10:41Z</time>`},
		{momentOf(at, time.UTC), `<time datetime="2026-10-25T10:41:30Z">2026-10-25T10:41Z</time>`},
		{momentOf(at, paris), `<time datetime="2026-10-25T10:41:30Z">2026-10-25T11:41</time>`},
		{dayOf(at, nil), `<time datetime="2026-10-25T10:41:30Z" data-local="day">2026-10-25</time>`},
		{sample("2026-09-29T08:12"), `2026-09-29T08:12`},
	} {
		if got := string(c.m.HTML()); got != c.want {
			t.Errorf("got  %s\nwant %s", got, c.want)
		}
	}
	if got := string(dayOf(at, paris).In("jusqu'au %s")); got != `jusqu&#39;au <time datetime="2026-10-25T10:41:30Z">2026-10-25</time>` {
		t.Errorf("in a sentence: %s", got)
	}
}
