package gateway

import (
	"maps"
	"slices"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Who a session is, remembered for a few seconds.
//
// sessionIdentity answers the access rule, the forwarded headers, the signed
// JWT, the page stamp and the rate limits - several of them on the SAME
// request - and it used to read the account, the organisation and the
// session's roles from the database every time: six to eight queries for one
// authenticated call. On the embedded engine each one is a WAL read lock, and
// the benchmark (tools/bench) read that as a route checking a token and
// signing a JWT at 6,500 req/s on one core, where Kong's key-auth reads
// 39,000.
//
// What it is built from - accounts, organisations, memberships, groups, roles,
// the catalogue - is written from dozens of places, so the entries are not
// dropped one write at a time: one missed call would be a stale role nobody
// could explain. Forgetting is instead ONE increment of an epoch, done after
// every write the gateway serves outside its proxied traffic (the admin API,
// the sign-in and profile pages, the agent endpoint - wired in main), and
// relayed to the other gateways by the bus. An entry read under an older epoch
// is simply not used. The window only matters for a lost bus message, which is
// what the session cache already accepts.
//
// What depends on the clock - the account's validity window - is still judged
// on every request.

// identityTTL bounds how long an entry is served when no write came along.
const identityTTL = 5 * time.Second

// maxIdentities bounds the memory: past it, entries that aged out are swept.
const maxIdentities = 10000

// identityKey is what an identity depends on in the session: the same account
// in another organisation, or under another group, is another set of roles.
type identityKey struct{ user, tenant, group string }

type identityEntry struct {
	data identityData
	// owner is the account reduced to what a request is judged against.
	owner  store.User
	epoch  uint64
	readAt time.Time
}

// ForgetIdentities makes every remembered identity stale, on THIS gateway: an
// account, an organisation, a membership, a group or a role has just been
// written. Called locally after such a write and by the bus when another
// gateway reports one.
func (rt *Router) ForgetIdentities() {
	rt.identityEpoch.Add(1)
}

func (rt *Router) cachedIdentity(key identityKey, now time.Time) (identityEntry, bool) {
	rt.identityMu.Lock()
	defer rt.identityMu.Unlock()
	e, hit := rt.identities[key]
	if !hit || e.epoch != rt.identityEpoch.Load() || now.Sub(e.readAt) >= identityTTL {
		return identityEntry{}, false
	}
	return e, true
}

// rememberIdentity stores an entry under the epoch read BEFORE the database was
// asked. That order is the whole guarantee: the epoch is bumped after a write
// commits, so a write landing while this read was in flight leaves the entry
// under an epoch that is already over - served never, read again next time.
// Taking the epoch at storing time instead would label a pre-write answer as
// current.
func (rt *Router) rememberIdentity(key identityKey, e identityEntry, epoch uint64, now time.Time) {
	rt.identityMu.Lock()
	defer rt.identityMu.Unlock()
	if rt.identities == nil {
		rt.identities = map[identityKey]identityEntry{}
	}
	if len(rt.identities) >= maxIdentities {
		current := rt.identityEpoch.Load()
		for k, old := range rt.identities {
			if old.epoch != current || now.Sub(old.readAt) >= identityTTL {
				delete(rt.identities, k)
			}
		}
	}
	e.epoch, e.readAt = epoch, now
	rt.identities[key] = e
}

// clone hands a caller its own roles and fields: an entry is shared by every
// request of that session, and a caller that appended to the slice it was
// given would otherwise edit somebody else's identity. The tag table is the
// catalogue cache, already shared and never written.
func (d identityData) clone() identityData {
	d.Roles = slices.Clone(d.Roles)
	d.Memberships = slices.Clone(d.Memberships)
	d.Fields = maps.Clone(d.Fields)
	return d
}
