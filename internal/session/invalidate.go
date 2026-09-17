package session

import (
	"log/slog"

	"github.com/softwarity/meerkat/internal/store"
)

// Dropping a cached session, here and on the other gateways (STORE-03).
//
// Resolve serves a session from memory for a few seconds, which is what keeps
// a busy page from asking the database on every request. The cost is that a
// session which CHANGES - the organisation just chosen, the login step just
// completed, the whole thing just revoked - is stale until that window passes.
// On one node that is handled precisely: every method that changes a session
// drops its entry, so only a revocation from elsewhere waits.
//
// With several gateways the same drop has to happen on all of them, and it did
// not, which is worse than a stale revocation: an operator choosing their
// organisation on node A and being served their previous one by node B for the
// next five seconds is a wrong answer, not a late one.
//
// The message that carries it is best effort, and that is the right shape
// here: losing one costs the five seconds a single gateway already accepts.

// Notify is called with what just changed so the other nodes can drop it too:
// (store.TopicSession, token hash), (store.TopicSessionUser, user id) or
// (store.TopicAPIToken, token id).
//
// Wired to the cluster bus by main; nil on a single gateway, where the local
// drop is the whole job.
func (m *Manager) Notify(fn func(topic, arg string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notify = fn
}

// Forget drops one session from THIS node's cache and says nothing to anyone.
// It is what the bus calls when another node reports a change - announcing it
// again would be this node telling the others what they just told it.
func (m *Manager) Forget(tokenHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, tokenHash)
}

// ForgetUser drops every cached session belonging to a user, and every cached
// API token they own.
//
// The caches are keyed by token hash, so this is a scan - which is fine,
// because what triggers it is a password reset or a disabled account, not a
// request.
func (m *Manager) ForgetUser(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for th, e := range m.cache {
		if e.sess.UserID == userID {
			delete(m.cache, th)
		}
	}
	for th, e := range m.tokens {
		if e.tok.UserID == userID {
			delete(m.tokens, th)
		}
	}
	m.forgotten++
}

// Revoked says a user's sessions have just been deleted from the store, so
// every node forgets what it had cached for them.
//
// Called by whoever did the deleting: the store has no idea a cache exists,
// and giving it one would be the wrong direction for that dependency.
func (m *Manager) Revoked(userID string) {
	m.ForgetUser(userID)
	m.tell(store.TopicSessionUser, userID)
}

// UserChanged says an account just changed in a way a cached session or token
// must not outlive - disabled, deleted, given another validity window - so
// every node reads it again on the next request.
func (m *Manager) UserChanged(userID string) {
	m.ForgetUser(userID)
	m.tell(store.TopicSessionUser, userID)
}

// ForgetToken drops one API token from THIS node's cache, by token id, and
// says nothing to anyone (the bus calls it). "*" drops every token: the
// gateway-wide token policy just changed.
func (m *Manager) ForgetToken(tokenID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for th, e := range m.tokens {
		if tokenID == "*" || e.tok.ID == tokenID {
			delete(m.tokens, th)
		}
	}
	m.forgotten++
}

// TokenChanged is ForgetToken plus the message: every write that changes what
// a token may do - revoke, toggle, renew, a new perimeter, the policy - ends
// here, or the token would keep its old powers for the length of the cache
// window.
func (m *Manager) TokenChanged(tokenID string) {
	m.ForgetToken(tokenID)
	m.tell(store.TopicAPIToken, tokenID)
}

// dropped is the local half plus the message: every method that CHANGES a
// session ends here.
func (m *Manager) dropped(tokenHash string) {
	m.Forget(tokenHash)
	m.tell(store.TopicSession, tokenHash)
}

func (m *Manager) tell(topic, arg string) {
	m.mu.Lock()
	fn := m.notify
	m.mu.Unlock()
	if fn == nil {
		return
	}
	defer func() {
		// A cache hint must never be able to fail a sign-in. The caller has
		// already saved what it saved.
		if r := recover(); r != nil {
			slog.Error("session invalidation panicked", "topic", topic, "panic", r)
		}
	}()
	fn(topic, arg)
}
