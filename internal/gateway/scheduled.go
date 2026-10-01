package gateway

import "context"

// ScheduledUser is the name a scheduled call answers to, and this is the
// identity such a call carries (SCHED-01).
//
// A scheduled call has no account behind it, and that is the design rather
// than a gap: making every service own a Meerkat account - and making somebody
// keep the rights of those accounts in step with the endpoints they call - was
// the complication this replaces. What a schedule says instead is which ROLES
// its call needs; the gateway turns that into a caller named "meerkat", and
// everything downstream treats it as it treats anybody else.
//
// WHAT BOUNDS IT is the token that wrote the schedule: a control-plane token,
// minted by root, whose perimeter is the scheduled calls. Whoever holds it can
// ask for any role, the way whoever holds it could already schedule any call -
// it is an administrator's credential, traced in the audit and revocable in a
// click. Two services that must not reach the same things get two tokens.
//
// It is posed in the CONTEXT of the in-process request the scheduler makes,
// never in a header: nothing about it travels on a wire, so nothing about it
// can be forged from outside.
const ScheduledUser = "meerkat"

type scheduledKey struct{}

// scheduledCall is what the scheduler asks to be seen as.
type scheduledCall struct {
	roles  []string
	tenant string
}

// WithScheduledCaller marks ctx as a scheduled call made as ScheduledUser with
// these roles, in this organisation (empty for none).
func WithScheduledCaller(ctx context.Context, roles []string, tenantID string) context.Context {
	return context.WithValue(ctx, scheduledKey{}, scheduledCall{roles: roles, tenant: tenantID})
}

// scheduledCaller reads it back.
func scheduledCaller(ctx context.Context) (scheduledCall, bool) {
	c, ok := ctx.Value(scheduledKey{}).(scheduledCall)
	return c, ok
}

// scheduledIdentity is what sessionIdentity answers for such a call: a caller
// with a name, an organisation and roles, and no account to read - the roles
// are expanded through the catalogue's hierarchy exactly as a session's are,
// so asking for a parent role grants what it implies.
func (rt *Router) scheduledIdentity(ctx context.Context) (identityData, bool) {
	c, ok := scheduledCaller(ctx)
	if !ok {
		return identityData{}, false
	}
	d := identityData{
		UserID:   ScheduledUser,
		Username: ScheduledUser,
		TenantID: c.tenant,
		Roles:    rt.expandSimRoles(ctx, c.roles),
	}
	if c.tenant != "" {
		if t, err := rt.st.GetTenant(ctx, c.tenant); err == nil {
			d.Tenant = t.Name
		}
	}
	return d, true
}
