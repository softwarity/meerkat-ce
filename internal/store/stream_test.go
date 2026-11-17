package store

import "testing"

// A stream is a label value: the token and the characters a log backend takes,
// nothing else, and the error names what is allowed.
func TestAStreamTemplateHoldsOnlyWhatABackendTakes(t *testing.T) {
	for _, ok := range []string{"", "audit", " log.{servicename}.meerkat ", "audit_{servicename}-eu"} {
		v := ok
		if err := SanitizeStream("auditStream", &v); err != nil {
			t.Errorf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"audit stream", "audit/{servicename}", "{service}", "a\"b"} {
		v := bad
		if err := SanitizeStream("auditStream", &v); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for key, ok := range map[string]bool{"audit": true, "app.audit": true, "team_stream": true, "app stream": false, "{servicename}": false} {
		v := key
		if err := SanitizeStreamKey("auditStreamKey", &v); (err == nil) != ok {
			t.Errorf("%q: %v", key, err)
		}
	}
}
