package tracing

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// The gateway's logs, PUSHED to the collector over OTLP (OBS-03) - the other
// way out, beside writing them for an agent to read. For an installation with
// no agent on its nodes: a bare Docker host, a VM, a Swarm without one.
//
// A queue of its own, never the audit's: a flood of logs must not cost the
// trail a single event. And a softer policy than the audit's: a batch the
// collector refuses three times is dropped and counted, rather than held
// while the queue fills behind it - a log line is worth less than the
// request it describes.
//
// Its own warnings go to stderr directly, never through slog: a shipper that
// logs its failures through the logger it ships would feed on itself.

const (
	logQueueSize = 20000
	logBatchSize = 500
	logFlush     = 2 * time.Second
	logAttempts  = 3
)

var (
	logMu     sync.Mutex
	logSend   func([]byte) error
	logConfig Config
	logQueue  = make(chan AuditRecord, logQueueSize)
	logOnce   sync.Once
	logLost   atomic.Int64
	logWarned atomic.Int64
)

// ApplyLogs points the logs at the collector, or stops pushing them. The
// sender is the audit's (one /v1/logs, one Enterprise code path).
func ApplyLogs(cfg Config, on bool) error {
	logMu.Lock()
	defer logMu.Unlock()
	logSend = nil
	if !on {
		return nil
	}
	if auditStarter == nil {
		return fmt.Errorf("pushing the logs to an OpenTelemetry collector is part of the Enterprise edition")
	}
	send, err := auditStarter(cfg)
	if err != nil {
		return err
	}
	logSend, logConfig = send, cfg
	logOnce.Do(func() { go logWorker() })
	return nil
}

// PushingLogs says whether the logs leave.
func PushingLogs() bool {
	logMu.Lock()
	defer logMu.Unlock()
	return logSend != nil
}

// LogsLost is how many log lines were dropped since the start: a full queue,
// or a batch the collector kept refusing.
func LogsLost() int64 { return logLost.Load() }

// ShipLog queues one line. Never blocks the caller.
func ShipLog(r AuditRecord) {
	if !PushingLogs() {
		return
	}
	select {
	case logQueue <- r:
	default:
		logDropped(1, "the queue is full")
	}
}

func logDropped(n int64, why string) {
	total := logLost.Add(n)
	now := time.Now().Unix()
	if last := logWarned.Load(); now-last >= 60 && logWarned.CompareAndSwap(last, now) {
		fmt.Fprintf(os.Stderr, "meerkat: log lines dropped (%s): %d since start\n", why, total)
	}
}

func logWorker() {
	batch := make([]AuditRecord, 0, logBatchSize)
	tick := time.NewTicker(logFlush)
	defer tick.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		logMu.Lock()
		send, cfg := logSend, logConfig
		logMu.Unlock()
		if send == nil {
			batch = batch[:0]
			return
		}
		body, err := logsPayload(cfg, "logs", "meerkat.logs", batch)
		if err == nil {
			pause := time.Second
			for attempt := 1; ; attempt++ {
				if err = send(body); err == nil || attempt == logAttempts {
					break
				}
				time.Sleep(pause)
				pause *= 2
			}
		}
		if err != nil {
			logDropped(int64(len(batch)), "the collector refused them: "+err.Error())
		}
		batch = batch[:0]
	}
	for {
		select {
		case r := <-logQueue:
			batch = append(batch, r)
			if len(batch) >= logBatchSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}
