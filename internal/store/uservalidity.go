package store

import (
	"fmt"
	"time"
)

// A validity window on an account (RBAC-05): the contractor whose access stops
// on the 31st, the intern, the auditor in for a fortnight.
//
// Until now the only answer was `enabled`, a boolean somebody has to remember
// to turn off - which means access that outlives the reason for it, every
// time, because nobody has a reminder for a thing that works. A window is the
// same decision taken once, at the moment it is known.
//
// Checked at SIGN-IN and at every session resolution, not by a clock: an
// account that expires at three in the morning must not sign anybody out
// mid-sentence, and one that expired last night must not be let in this
// morning. Same rule the password expiry follows, for the same reason.
//
// Zero means "no bound on this side", so a window may be open-ended either
// way: a contract with a start and no end is as ordinary as the reverse.

// ValidFromUntil reports whether the window is coherent. An end before its
// start is refused at the form rather than silently locking an account out
// forever with no message that says why.
func ValidFromUntil(from, until int64) error {
	if from != 0 && until != 0 && until < from {
		return fmt.Errorf("the access ends (%s) before it starts (%s)",
			time.Unix(until, 0).UTC().Format(time.DateOnly),
			time.Unix(from, 0).UTC().Format(time.DateOnly))
	}
	return nil
}

// ValidAt reports whether this account's window contains t.
//
// The bounds are DAYS, not instants: an account valid until the 31st works all
// of the 31st. Anything else would be a trap - nobody types a date meaning
// "and it stops at midnight as the day begins".
func (u User) ValidAt(t time.Time) bool {
	if u.ValidFrom != 0 && t.Unix() < u.ValidFrom {
		return false
	}
	if u.ValidUntil != 0 && t.Unix() >= u.ValidUntil+86400 {
		return false
	}
	return true
}

// ValidityReason says why an account is out of its window, for the sign-in
// refusal and for the console. Empty when it is inside.
//
// It names the DATE. "Your access is not valid" sends somebody to support to
// ask the one question the message could have answered.
func (u User) ValidityReason(t time.Time) string {
	if u.ValidFrom != 0 && t.Unix() < u.ValidFrom {
		return "this access opens on " + time.Unix(u.ValidFrom, 0).UTC().Format(time.DateOnly)
	}
	if u.ValidUntil != 0 && t.Unix() >= u.ValidUntil+86400 {
		return "this access ended on " + time.Unix(u.ValidUntil, 0).UTC().Format(time.DateOnly)
	}
	return ""
}
