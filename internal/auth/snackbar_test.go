package auth

import (
	"strings"
	"testing"
)

// The served button carries the snackbar and the desktop path, spliced in
// whole: a placeholder left in the script is a syntax error on every page.
func TestTheUserButtonCarriesTheSnackbar(t *testing.T) {
	mux, _, _ := setupFlow(t)
	js := bodyString(do(t, mux, "GET", "/meerkat/user-button.js", nil, nil))
	for _, want := range []string{
		"function mkSnackbar(o)",
		"role', 'status'",
		"aria-live', 'polite'",
		"prefers-reduced-motion",
		"--mk-inverse-surface",
		"--mk-inverse-on-surface",
		"--mk-inverse-primary",
		// The desktop: only on a secure origin the reader allowed, one per id.
		"window.isSecureContext",
		"Notification.permission === 'granted'",
		"tag: it.group || it.id",
		// With buttons, through the gateway's worker at a scope that serves
		// nothing, and the page's own notification when it is refused.
		"register(NOTIFY_SW, { scope: NOTIFY_SW_SCOPE })",
		"'" + NotificationsWorkerPath + "'",
		"'" + NotificationsWorkerScope + "'",
		"Notification.maxActions",
		// Registered only while it can serve, unregistered when it cannot.
		"if (allowed) notifyWorker();",
		"reg.unregister()",
		".catch(() => pageNotification(it, go, lang))",
		// One that asks to be acknowledged: the button on the snackbar, and
		// first on the desktop.
		"label('notifAck')",
		"buttons.push({ action: 'ack', title: L.notifAck })",
		"'/ack', { method: 'POST', credentials: 'same-origin' }",
		// The worker calls an action with its method and its body, so they
		// ride the notification's data.
		"method: a.method || 'GET', body: a.body",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("the user button script does not carry %q", want)
		}
	}
	if strings.Contains(js, "__MK_") {
		t.Error("a placeholder is left in the user button script")
	}
	// Asking is the notifications page's, from a click: never here.
	if strings.Contains(js, "requestPermission") {
		t.Error("the user button asks for the desktop permission: only a click on the notifications page may")
	}
}

// The snackbar's words ride the payload the button reads, in the reader's
// language.
func TestTheSnackbarLabelsRideThePayload(t *testing.T) {
	labels := userButtonLabels(catalogue("fr"))
	for _, k := range []string{"notifNew", "notifOpen", "notifClose", "notifDone", "notifFailed", "notifAck", "notifAcked"} {
		if labels[k] == "" || labels[k] != EmbeddedString("fr", k) {
			t.Errorf("label %s = %q, want the French catalogue's", k, labels[k])
		}
	}
}
