package license

import (
	"fmt"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/evalmark"
)

// InstanceGrace is how long instances may run past a license's ceiling before
// it counts. A rolling update runs one more than the ceiling for as long as a
// new pod takes to come up, and that is not an overrun.
var InstanceGrace = graceOf(instanceGrace)

// instanceGrace is InstanceGrace as written, and it is set at LINK time only,
// by the platform that proves the overrun (e2e/editions, with -ldflags -X):
// fifteen minutes is a quarter of an hour of a CI job waiting. No flag, no
// variable and no setting reaches it - a gateway that could be told its grace
// would be told a year.
var instanceGrace = "15m"

func graceOf(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 15 * time.Minute
	}
	return d
}

// The license this node honours, and what it has seen running. In memory
// because the pages that carry the evaluation notice are drawn without a read
// of the database: the admin API puts it here at start and on every change.
var state struct {
	sync.Mutex
	current   *License
	running   int
	overSince time.Time
	overrun   bool
	served    int
}

// Apply puts a verified license in force, or nil for none.
func Apply(l *License) {
	state.Lock()
	defer state.Unlock()
	state.current = l
	state.overSince, state.overrun = time.Time{}, false
	settle()
}

// Current is the license in force, nil for none.
func Current() *License {
	state.Lock()
	defer state.Unlock()
	return state.current
}

// Observe records how many instances run. Nothing is refused and no traffic is
// touched: an overrun that lasts brings the evaluation notice back, which is
// what an installation without a license for that size shows.
func Observe(running int, now time.Time) {
	state.Lock()
	defer state.Unlock()
	state.running = running
	over := state.current != nil && state.current.Instances > 0 && running > state.current.Instances
	switch {
	case !over:
		state.overSince, state.overrun = time.Time{}, false
	case state.overSince.IsZero():
		state.overSince = now
	case now.Sub(state.overSince) >= InstanceGrace:
		state.overrun = true
	}
	settle()
}

// ObserveTenants records how many tenants this gateway serves. Past the ceiling
// the evaluation notice stays, at once: nothing here is a passing state like a
// rolling update, and a license taken for one tenant must not cover the ones
// created during an evaluation. They keep being served: a license cuts nobody.
func ObserveTenants(served int) {
	state.Lock()
	defer state.Unlock()
	state.served = served
	settle()
}

// TenantOverrun says in a sentence that more tenants are served than the license
// covers, and what ends it. "" while they are not.
func TenantOverrun() string {
	state.Lock()
	defer state.Unlock()
	if !tenantsOver() {
		return ""
	}
	return fmt.Sprintf("%d tenants are served and this %s license covers %d: the evaluation notice is back "+
		"until Several tenants is turned off, they are %d, or the license covers them",
		state.served, state.current.EditionName(), state.current.Tenants, state.current.Tenants)
}

// tenantsOver holds with the state's lock taken.
func tenantsOver() bool {
	return state.current != nil && state.current.Tenants > 0 && state.served > state.current.Tenants
}

// OverSince is when the instances first ran past the license's ceiling, and
// the zero time while they do not: the notice comes back at OverSince plus
// InstanceGrace.
func OverSince() time.Time {
	state.Lock()
	defer state.Unlock()
	return state.overSince
}

// Overrun says in a sentence that more instances run than the license covers,
// "" while they do not, grace included.
func Overrun() string {
	state.Lock()
	defer state.Unlock()
	if !state.overrun {
		return ""
	}
	return fmt.Sprintf("%d instances are running and this %s license covers %d: "+
		"the evaluation notice is back until they are %d, or the license covers them",
		state.running, state.current.EditionName(), state.current.Instances, state.current.Instances)
}

// settle holds with the state's lock taken.
func settle() { evalmark.Waive(state.current != nil && !state.overrun && !tenantsOver()) }

// TenantRefusal says why a gateway holding have tenants may not take one more,
// nil when it may. Never about the tenants already there: they keep being
// served.
func TenantRefusal(have int) error {
	l := Current()
	if l == nil || l.Tenants == 0 || have < l.Tenants {
		return nil
	}
	return fmt.Errorf("this %s license covers %d tenant(s) and this gateway holds %d: "+
		"another one takes an Enterprise license", l.EditionName(), l.Tenants, have)
}
