package logging

import "testing"

// The access log is a live switch now (the console's, Infra, OpenTelemetry):
// on, off and on again without a restart.
func TestTheAccessLogSwitchesLive(t *testing.T) {
	t.Cleanup(DisableAccessLog)
	SetAccessLog(true)
	if !AccessLogEnabled() {
		t.Fatal("switched on, not writing")
	}
	SetAccessLog(true) // twice: still one logger, no error
	SetAccessLog(false)
	if AccessLogEnabled() {
		t.Fatal("switched off, still writing")
	}
	SetAccessLog(true)
	if !AccessLogEnabled() {
		t.Fatal("switched on again, not writing")
	}
}
