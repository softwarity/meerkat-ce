// Package notifier is the community side of the notifications (NOTIF-05):
// the shape the Enterprise implementation (ee/notify) fills in, and the one
// that does nothing when this binary carries none.
//
// Same arrangement as the OpenID provider (internal/oidcprovider): the trunk
// mounts what it is handed and knows nothing of what a notification is. In
// the community image every method below answers "nothing here", the paths
// fall through to the routes, and the user button shows no badge.
package notifier

import (
	"context"
	"net/http"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/events"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The data plane's paths. Under /meerkat/ like every page the gateway serves
// itself, so no route can be shadowed by them.
const (
	PagePath   = auth.NotificationsPagePath
	PathPrefix = PagePath + "/"
	ListPath   = PagePath + ".json"
	CountsPath = PathPrefix + "counts"
	WorkerPath = auth.NotificationsWorkerPath
	// WorkerScope serves nothing: the worker controls no page.
	WorkerScope = auth.NotificationsWorkerScope
)

// InternalPath is where the services publish, on the internal plane.
const InternalPath = "/api/notifications"

// Sender is who wrote a request on the internal plane, as far as the access
// mode could tell: the identity in its mode's own form, how it was known, and
// the address it came from.
type Sender struct {
	Identity string
	Via      string
	Addr     string
}

// Name is how the sender is shown and matched: its identity, or its address
// when nothing said who it was.
func (s Sender) Name() string {
	if s.Identity != "" {
		return s.Identity
	}
	return s.Addr
}

// Endpoint is one handler of the internal plane. The trunk mounts it behind
// the access the operator chose for the notifications, and hands it the
// sender that access recognised.
type Endpoint struct {
	Pattern string
	Handle  func(w http.ResponseWriter, r *http.Request, from Sender)
}

// Mux is what the handlers are mounted on.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Deps is what an implementation is built with.
type Deps struct {
	Store    *store.Store
	Sessions *session.Manager
	Router   *gateway.Router
	// Hub is the data plane's live channel.
	Hub *events.Hub
	// Pages draws a page in the frame the sign-in pages wear.
	Pages *auth.Handler
	// Signal tells the other nodes (store.TopicNotification). This node is
	// told by its own Receive, called by the implementation directly.
	Signal func(ctx context.Context, arg string)
	// Plane is the data plane's handler: a sender's service is called
	// through its route when a reader acknowledges (the scheduler's way in).
	Plane http.Handler
	// Audit writes an event to the trail and the console's live channel: an
	// acknowledgement is one.
	Audit func(ctx context.Context, ev store.AuditEvent)
}

// Badge is what the user button and the portal bar show.
type Badge struct {
	// Total is every unread notification the reader may still act on.
	Total int `json:"total"`
	// Page is where the reader reads them: the portal's notifications page
	// when they may open it, the built-in one otherwise.
	Page string `json:"page"`
	// InBar says the portal bar carries the notifications entry for them.
	InBar bool `json:"inBar,omitempty"`
}

// Published is what a publication answers: the notification, how many
// people it was given to, and whether its key was already live (then nobody
// was told again). Sent to every tenant, it has no id of its own: Tenants are
// the copies, one per tenant that received one, Skipped the tenants that did
// not and why, Recipients their sum and Existing true when every copy was
// already live.
type Published struct {
	ID         string          `json:"id,omitempty"`
	Recipients int             `json:"recipients"`
	Existing   bool            `json:"existing,omitempty"`
	Tenants    []PublishedCopy `json:"tenants,omitempty"`
	Skipped    []SkippedTenant `json:"skipped,omitempty"`
}

// PublishedCopy is one tenant's copy of a notification sent to every tenant.
type PublishedCopy struct {
	Tenant     string `json:"tenant"`
	Name       string `json:"name"`
	ID         string `json:"id"`
	Recipients int    `json:"recipients"`
	Existing   bool   `json:"existing,omitempty"`
}

// SkippedTenant is a tenant a notification sent to every tenant left out.
type SkippedTenant struct {
	Tenant string `json:"tenant"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Refusal is a publication refused: the status it answers with, what is
// wrong, and the field of the message it concerns when it concerns one
// ("title", "detail", "actions.0", "actions.0.label"...).
type Refusal struct {
	Status  int
	Field   string
	Message string
}

func (r *Refusal) Error() string { return r.Message }

// Target is one route a link of a notification may lead to, as a sender
// names it: the service its upstream is, and the path that service starts at.
type Target struct {
	Route   string `json:"route"`
	Name    string `json:"name"`
	UI      bool   `json:"ui"`
	Service string `json:"service"`
	// Base is the path the upstream starts at, "" for its root: a path given
	// with this service is as the service sees it, so it starts there.
	Base string `json:"base,omitempty"`
	// Public is the public address Base leads to: what a reader opens.
	Public string `json:"public,omitempty"`
	// Problem says why an action leading to it would be refused,
	// DetailProblem the same for a detail (which only UI routes serve, so
	// fewer routes compete for the name). Empty when it would be accepted.
	Problem       string `json:"problem,omitempty"`
	DetailProblem string `json:"detailProblem,omitempty"`
}

// Notifier is what the Enterprise implementation provides.
type Notifier interface {
	// Internal are the internal plane's handlers.
	Internal() []Endpoint
	// MountData mounts the page and its API on the data plane.
	MountData(mux Mux)
	// Receive acts on a signal from another node.
	Receive(arg string)
	// Badge is the counts for one session, ok false when the feature is off
	// for its organisation.
	Badge(ctx context.Context, sess store.Session) (b Badge, ok bool)
	// Purge deletes what expired.
	Purge(ctx context.Context, now int64) (int64, error)
	// Withdraw takes one back whoever sent it, for the console.
	Withdraw(ctx context.Context, id string) error
	// Publish is the internal plane's publication, for the console: raw is
	// the same message, checked and published the same way. tenant, when
	// set, is the only tenant it may go to (the message may name it or not).
	// A refusal is a *Refusal.
	Publish(ctx context.Context, raw []byte, from Sender, tenant string) (Published, error)
	// Targets are the enabled routes a link may lead to, named as a sender
	// names them.
	Targets(ctx context.Context) ([]Target, error)
}

// Factory builds a notifier over its dependencies.
type Factory func(Deps) Notifier

var factory Factory

// Register is called by the Enterprise implementation, at init.
func Register(f Factory) {
	if factory != nil {
		panic("notifier: a notifier is already registered")
	}
	factory = f
}

// Linked says whether this binary carries the notifications.
func Linked() bool { return factory != nil }

// New builds the notifier, or the one that does nothing.
func New(d Deps) Notifier {
	if factory == nil {
		return absent{}
	}
	return factory(d)
}

type absent struct{}

func (absent) Internal() []Endpoint { return nil }
func (absent) MountData(Mux)        {}
func (absent) Receive(string)       {}
func (absent) Badge(context.Context, store.Session) (Badge, bool) {
	return Badge{}, false
}
func (absent) Purge(context.Context, int64) (int64, error) { return 0, nil }
func (absent) Withdraw(context.Context, string) error      { return store.ErrNotificationNotFound }
func (absent) Publish(context.Context, []byte, Sender, string) (Published, error) {
	return Published{}, &Refusal{Status: http.StatusNotFound,
		Message: "notifications are part of the Enterprise edition, and this is the Community edition"}
}
func (absent) Targets(context.Context) ([]Target, error) { return nil, nil }
