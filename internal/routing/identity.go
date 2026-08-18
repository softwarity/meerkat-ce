package routing

import "context"

// Identity is what a route's own answer may say about its caller (the
// "respond" filter). It is a COPY of the resolved session, not a handle on the
// store: a template sees the person making this request and nothing else - no
// other account, no configuration, no way to reach the database.
//
// It lives here rather than in the gateway package because bricks are compiled
// here and must not depend on the engine that runs them.
type Identity struct {
	Username string
	UserID   string
	Fullname string
	Email    string
	Tenant   string
	TenantID string
	Timezone string
	Roles    []string
}

// SignedIn is what a template asks with {{if .SignedIn}}: an anonymous caller
// reaching a route with no gateway rule still gets an answer, and the template
// decides what that answer says.
func (i Identity) SignedIn() bool { return i.Username != "" }

type identityKey struct{}

// WithIdentity carries the resolved caller to the route's handler.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFrom returns the caller carried by ctx, or the zero Identity - which
// is an anonymous one, and reads as such in a template.
func IdentityFrom(ctx context.Context) Identity {
	id, _ := ctx.Value(identityKey{}).(Identity)
	return id
}

// SampleIdentity is the fictional caller a template is validated against when
// a route is saved, and the one the editor's preview reads.
//
// Ordinary values on purpose. It used to carry a name full of quotes to prove
// that escaping happens, and the preview it produced was unreadable: someone
// checking the SHAPE of their answer had to first parse the demonstration.
// What escaping does is a question for the day it is asked, with a name of
// one's own; what this has to show is the answer.
var SampleIdentity = Identity{
	Username: "john",
	UserID:   "usr_123",
	Fullname: "Jane Doe",
	Email:    "jdoe@example.com",
	Tenant:   "Acme",
	TenantID: "tnt_123",
	Timezone: "Europe/Paris",
	Roles:    []string{"ROLE_A", "ROLE_B"},
}
