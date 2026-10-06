package live

// PresenceTopic is what the Users and Members screens subscribe to, to hear
// that somebody signed in, out, or changed organisation (CONSOLE-07).
const PresenceTopic = "presence"

// NewPresence tells a screen that the set of open sessions moved.
//
// A Counter and nothing else: the screen then asks GET /api/sessions, which
// already scopes what each reader may see - root everything, an application
// administrator the applications', an organisation's administrator their
// organisations'. Nothing about a person crosses this socket, so every
// administrator may watch it.
//
// Nothing ticks: every move reaches it, from this node through the store's
// hook, and from the others through the cluster bus (store.TopicPresence).
// Expired sessions too - the minute's upkeep deletes them, and a deletion is
// a move like any other.
func NewPresence() *Counter { return NewCounter(PresenceTopic) }
