package auth

// Scheduled calls (SCHED-01) are NOT served here, and this file says why.
//
// They were, once: a service managed its schedules on the data plane, as
// itself. They moved to the control plane, whole, because a schedule is a
// service the GATEWAY provides - like the agent endpoint - not something
// an application exposes to a browser. Nobody's user agent has business
// calling it: a backend does, with a token, and hands its own users whatever
// it decides to show them.
//
// What that bought: one API on one port, one credential to mint and rotate,
// and a perimeter that is actually enforced - the control plane checks a
// token's scope in one funnel (admin.authed), the data plane checks none.
//
// See internal/admin/schedules.go. The data plane's only remaining part in
// this is the call itself: the scheduler makes it through the front door, as
// "meerkat" carrying the roles the schedule asked for, so it meets every rule
// a person meets. See internal/gateway/scheduled.go.
