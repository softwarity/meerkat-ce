package live

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	livewire "github.com/softwarity/livewire/go"
	"github.com/softwarity/meerkat/internal/store"
)

// SchedulesTopic is what the console's scheduler screen subscribes to.
const SchedulesTopic = "schedules"

// Schedules publishes the scheduled calls as a live list (SCHED-01).
//
// A screen watching this sees a run start, advance and finish without asking,
// which is the whole reason a scheduler needs a socket rather than a refresh
// button: the interesting moment lasts as long as the job does.
//
// The adapter lives HERE, like the traffic one, so the scheduler package
// counts and calls and knows nothing about who is watching.
type Schedules struct {
	st *store.Store

	// The wake channel is a TICKER rather than a signal from the scheduler,
	// and the reason is worth writing down: a run is advanced by whoever is
	// doing the work - another node in a cluster, or the service itself
	// reporting progress - so there is no local event to listen to. Asking
	// every two seconds is what a screen would do anyway, once, for everybody
	// watching.
	once sync.Once
	wake <-chan struct{}
}

// NewSchedules adapts the schedules table to the live channel.
func NewSchedules(st *store.Store) *Schedules { return &Schedules{st: st} }

// schedulesQuery is what a screen narrows by. It arrives as JSON off a
// socket, so every field is taken as a filter and nothing else.
type schedulesQuery struct {
	Tenant string `json:"tenant"`
	Route  string `json:"route"`
	// Meta is the operator's half of what a service files its schedules
	// under: `station=42`, or `station=~^st-\d+$`. One condition per entry,
	// all of them required - the screen asks one question at a time and this
	// is what it narrows with.
	Meta []string `json:"meta"`
}

// ReadQuery is the trust boundary: three strings, used as equality filters.
func (s *Schedules) ReadQuery(raw json.RawMessage) (any, error) {
	var q schedulesQuery
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &q); err != nil {
			return nil, err
		}
	}
	q.Tenant = strings.TrimSpace(q.Tenant)
	q.Route = strings.TrimSpace(q.Route)
	// Compiled here, at the boundary: an expression that does not compile is
	// an answer to the screen, not a filter that silently matches nothing.
	// The conditions are kept as written for the key below.
	kept := q.Meta[:0]
	for _, one := range q.Meta {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}
		key, value, ok := strings.Cut(one, "=")
		if !ok {
			return nil, fmt.Errorf("a metadata filter reads key=value, or key=~expression: %q", one)
		}
		if _, err := store.ParseMetaMatch(key, value); err != nil {
			return nil, err
		}
		kept = append(kept, one)
	}
	q.Meta = kept
	return q, nil
}

// conditions turns what the screen sent into what the store filters with.
// Re-parsed rather than carried: ReadQuery already refused what does not
// compile, and a compiled expression is not something to put in a cache key.
func (q schedulesQuery) conditions() []store.MetaMatch {
	out := make([]store.MetaMatch, 0, len(q.Meta))
	for _, one := range q.Meta {
		key, value, _ := strings.Cut(one, "=")
		if m, err := store.ParseMetaMatch(key, value); err == nil {
			out = append(out, m)
		}
	}
	return out
}

// Key separates one filter from another: two screens narrowing differently
// are two reads, and two screens narrowing the same way are one.
func (s *Schedules) Key(q any) string {
	f, _ := q.(schedulesQuery)
	return SchedulesTopic + "|" + f.Tenant + "|" + f.Route +
		"|" + strings.Join(f.Meta, "&")
}

// Wake fires on a ticker - see the note on the struct.
func (s *Schedules) Wake() <-chan struct{} {
	s.once.Do(func() {
		c := make(chan struct{}, 1)
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				select {
				case c <- struct{}{}:
				default:
				}
			}
		}()
		s.wake = c
	})
	return s.wake
}

// Read is the list as it stands.
func (s *Schedules) Read(ctx context.Context, q any) (livewire.Window, error) {
	f, _ := q.(schedulesQuery)
	list, err := s.st.ListSchedules(ctx, store.ScheduleFilter{
		TenantID: f.Tenant, RouteID: f.Route, Meta: f.conditions(),
	})
	if err != nil {
		return livewire.Window{}, err
	}
	rows := make([]livewire.Row, 0, len(list))
	for _, sc := range list {
		rows = append(rows, livewire.Row{
			ID: sc.ID,
			// The version, and it is NOT updated_at alone.
			//
			// updated_at is a second, and a call to a service that answers in
			// milliseconds is claimed and closed inside the same one: the row
			// would then carry the same version before and after, and a screen
			// that caught the "running" in between would keep it for ever -
			// the end having nothing new to announce. Which is exactly what it
			// did, until a schedule left running on screen turned out to have
			// finished the moment anybody pressed reload.
			//
			// So the version is what MOVES on screen: the run in flight, where
			// it stands, which attempt, how far it says it is, and when the
			// last one ended. Same purpose as
			// before - a row already on the client is only sent again when
			// something about it changed - with the seconds no longer able to
			// swallow a whole run.
			UpdatedAt: rowVersion(sc),
			Data: map[string]any{
				"id": sc.ID, "name": sc.Name, "roles": sc.Roles, "tenantId": sc.TenantID,
				"routeId": sc.RouteID, "method": sc.Method, "path": sc.Path,
				// WHEN, all three shapes: a cadence, a calendar with the zone
				// it is read in, or a single date. The screen shows one
				// column for the three.
				"every": sc.Every, "cron": sc.Cron, "at": sc.At, "timezone": sc.Timezone,
				"metadata": sc.Metadata,
				"paused":   sc.Paused, "nextAt": sc.NextAt,
				"runId": sc.RunID, "runStarted": sc.RunStarted, "claimedBy": sc.ClaimedBy,
				"runState": sc.RunState, "attempts": sc.Attempts,
				"progress": sc.Progress, "lastState": sc.LastState,
				"lastDetail": sc.LastDetail, "lastAt": sc.LastAt,
				"createdBy": sc.CreatedBy,
			},
		})
	}
	total := len(rows)
	return livewire.Window{Rows: rows, Total: &total}, nil
}

func rowVersion(sc store.Schedule) string {
	return strconv.FormatInt(sc.UpdatedAt, 10) + "|" + sc.RunID +
		"|" + sc.RunState + "|" + strconv.Itoa(sc.Attempts) +
		"|" + strconv.Itoa(sc.Progress) +
		"|" + strconv.FormatInt(sc.LastAt, 10)
}
