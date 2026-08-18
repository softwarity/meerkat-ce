package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/softwarity/meerkat/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// The two halves of AUTH-10 that are about TIME rather than shape: a password
// may not come back, and it does not last forever.
//
// Both live here rather than in the store: comparing a candidate to a retired
// password is bcrypt, and bcrypt is how this product checks a password - an
// opinion the storage layer has no business holding.

// passwordReused says whether pwd is the one in use, or one of the last N the
// policy remembers. The current hash is included on purpose: "do not reuse the
// last 3" that let someone re-set the very password they already have would be
// a rule with a hole exactly where people aim.
//
// A store failure answers "not reused" and is logged: refusing every password
// change because a history table hiccupped would lock people out of the one
// action that fixes a compromised account.
func (h *Handler) passwordReused(ctx context.Context, u store.User, pwd string) bool {
	n := h.passwordPolicy(ctx).History
	if n <= 0 {
		return false
	}
	if u.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pwd)) == nil {
		return true
	}
	past, err := h.st.RecentPasswordHashes(ctx, u.ID, n)
	if err != nil {
		slog.Error("password history unreadable; reuse not checked", "user", u.ID, "err", err)
		return false
	}
	for _, hash := range past {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd)) == nil {
			return true
		}
	}
	return false
}

// passwordExpired says whether the policy's age limit has passed for u.
//
// Checked at SIGN-IN, never by a clock: a password expiring at three in the
// morning cannot sign anyone out - sessions have their own lifetime - it can
// only refuse the next login, which is exactly what this does by sending the
// person to the forced change.
//
// A password whose age is UNKNOWN (set before the column existed, or by a
// migration) never expires. Treating the zero as 1970 would expire every
// account on the installation at once, the first time someone typed a number
// into the expiry box.
func (h *Handler) passwordExpired(ctx context.Context, u store.User) bool {
	days := h.passwordPolicy(ctx).ExpiryDays
	if days <= 0 || u.PasswordChangedAt <= 0 || u.PasswordHash == "" {
		return false
	}
	age := time.Since(time.Unix(u.PasswordChangedAt, 0))
	return age > time.Duration(days)*24*time.Hour
}
