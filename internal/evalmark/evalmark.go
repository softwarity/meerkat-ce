// Package evalmark holds the marks the evaluation image puts on what it
// serves. It is a LEAF: it imports nothing, it decides nothing, and it asks no
// question about editions.
//
// There is no boolean here, and none anywhere else either. These strings are
// empty, and they stay empty unless something fills them. The only thing that
// ever does is ee/eval, from its init(), and ee/eval reaches the binary only
// through cmd/meerkat/link_eval.go, which the `eval` build tag compiles. So
// the community and Enterprise images do not carry an evaluation mark that a
// flag turns off: the code that would draw one is not in them at all, exactly
// as ee/layouts is absent from the community image.
//
// Which also answers the question the marks exist for. Nothing in the console,
// no environment variable and no configuration can empty these: filling them
// is a compile-time act, and emptying them again means building another
// binary.
package evalmark

// The marks, written once from ee/eval's init() - before any server starts, so
// nothing reads them while they are being set - and read-only from then on.
// An empty one is drawn nowhere, which is what every other build gets.
var (
	// FlowCSS styles the banner on the pages the gateway serves itself. It is
	// appended last to those pages' stylesheet.
	FlowCSS string

	// FlowBanner is the banner itself, placed at the top of the body of every
	// page of the sign-in flow - including the console's specimen, since it is
	// the same template and nothing here knows about previews.
	FlowBanner string

	// MenuEntry is a line shown when someone opens the user button's menu on a
	// proxied application's page. It is inert: no link, no action, nothing to
	// dismiss, and nothing outside the open menu, so the application's own
	// layout is untouched.
	MenuEntry string

	// MailNote is the line added at the foot of transactional mail.
	MailNote string

	// ConsoleCSS is injected into the console's index.html and draws the
	// watermark inside the rail-nav container.
	ConsoleCSS string

	// StartupNote is appended to the line the binary logs when it starts, so
	// what is running can be read from the journal rather than guessed.
	StartupNote string
)
