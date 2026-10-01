package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// DefaultMetricsPort is the metrics port when nobody chose one: next to the
// control plane's 9090, which is where somebody looking for it looks first.
const DefaultMetricsPort = 9091

// metricsDoor is the metrics port of THIS node: open while the exposition is
// on, on the port the console chose, and closed otherwise - no switch, no
// listener, the same rule as the developer tunnel.
//
// Opened and closed while the gateway runs, like the HTTPS listeners, and for
// the same reason: the port is chosen in the console, when the exposition is
// switched on, by the person who knows what the platform around it publishes.
// A variable read at startup would have made that a redeployment.
type metricsDoor struct {
	mu      sync.Mutex
	handler http.Handler
	// host is what the port is bound on: every interface in production, the
	// loopback in the tests.
	host string
	port int
	srv  *http.Server
}

// ServeMetricsPort gives this API a metrics port to open. Main calls it; the
// tests that do not have no listener, and saving the setting still works.
func (a *API) ServeMetricsPort() {
	a.metricsDoor = &metricsDoor{handler: a.ExpositionHandler()}
}

// ReloadMetricsPort brings this node's metrics port in line with the setting.
// Main hangs it on the change bus, so a port chosen on one node moves on all.
func (a *API) ReloadMetricsPort(ctx context.Context) error {
	if a.metricsDoor == nil {
		return nil
	}
	want := 0
	if edition.Enterprise && a.metricsEnabled(ctx) {
		want = a.metricsPortSetting(ctx)
	}
	return a.metricsDoor.open(want)
}

// StopMetricsPort closes it, at shutdown.
func (a *API) StopMetricsPort(ctx context.Context) error {
	if a.metricsDoor == nil {
		return nil
	}
	return a.metricsDoor.close(ctx)
}

func (a *API) metricsPortSetting(ctx context.Context) int {
	port := DefaultMetricsPort
	if err := a.st.GetSetting(ctx, store.SettingMetricsPort, &port); err != nil || port <= 0 {
		return DefaultMetricsPort
	}
	return port
}

// open listens on port, or closes when it is zero. The new listener is bound
// BEFORE the old one closes: a port that cannot be opened leaves the one that
// worked serving, and the error says why.
func (d *metricsDoor) open(port int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if port == d.port {
		return nil
	}
	var next *http.Server
	if port != 0 {
		ln, err := net.Listen("tcp", net.JoinHostPort(d.host, strconv.Itoa(port)))
		if err != nil {
			return fmt.Errorf("port %d cannot be opened on this gateway: %w", port, err)
		}
		next = &http.Server{Handler: d.handler, ReadHeaderTimeout: 10 * time.Second}
		go func() {
			if err := next.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("metrics port", "port", port, "err", err)
			}
		}()
		slog.Info("meerkat metrics listening", "addr", ln.Addr().String())
	}
	if d.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = d.srv.Shutdown(ctx)
		cancel()
		if port == 0 {
			slog.Info("meerkat metrics port closed", "port", d.port)
		}
	}
	d.srv, d.port = next, port
	return nil
}

func (d *metricsDoor) close(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.srv == nil {
		return nil
	}
	err := d.srv.Shutdown(ctx)
	d.srv, d.port = nil, 0
	return err
}
