package metrics

import (
	"errors"
	"sync"
	"time"
)

// Pushing the counters to an OpenTelemetry collector (OBS-05), the other way
// out beside /metrics: a stack that RECEIVES rather than scrapes.
//
// The same seam as the traces (internal/tracing): the trunk says when, the
// Enterprise package knows the format. The community binary links no pusher,
// and ApplyPush then says why instead of pretending.

// PushConfig is where the counters go and what they are called.
type PushConfig struct {
	Endpoint string
	Headers  map[string]string
	Service  string
	Version  string
	// Instance names THIS node. The counters are per node, and a collector
	// adding two nodes' cumulative totals under one identity would read every
	// switch between them as a reset.
	Instance string
	// Every is how often a push leaves. DefaultPushInterval when zero.
	Every time.Duration
}

// DefaultPushInterval is how often the counters leave. Twice the console's
// own window step: fine enough for an alert, coarse enough that a collector
// does not notice the gateway is there.
const DefaultPushInterval = 30 * time.Second

var (
	pusher      func(PushConfig, *Registry) (stop func(), err error)
	pushMu      sync.Mutex
	pushStop    func()
	errNoPusher = errors.New("pushing metrics over OTLP is part of the Enterprise edition, and this is the community image: " +
		"the built-in metrics screen works here with nothing to install")
)

// RegisterPusher is called from ee/telemetry's init().
func RegisterPusher(f func(PushConfig, *Registry) (func(), error)) { pusher = f }

// ApplyPush stops whatever pushes now and, when on, starts pushing reg where
// cfg says. Stop first, always: two pushers would send every point twice.
func ApplyPush(cfg PushConfig, reg *Registry, on bool) error {
	pushMu.Lock()
	defer pushMu.Unlock()
	if pushStop != nil {
		pushStop()
		pushStop = nil
	}
	if !on || reg == nil {
		return nil
	}
	if pusher == nil {
		return errNoPusher
	}
	if cfg.Every <= 0 {
		cfg.Every = DefaultPushInterval
	}
	stop, err := pusher(cfg, reg)
	if err != nil {
		return err
	}
	pushStop = stop
	return nil
}

// Pushing says whether a pusher runs, for the screen that shows the switch.
func Pushing() bool {
	pushMu.Lock()
	defer pushMu.Unlock()
	return pushStop != nil
}

// StopPush tears it down, at shutdown.
func StopPush() { _ = ApplyPush(PushConfig{}, nil, false) }
