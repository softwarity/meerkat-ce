package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// meerkat health: the probe a container runtime runs INSIDE the container.
//
// The image has no shell and no wget (distroless): a Swarm or compose
// healthcheck that used to call `wget -qO- http://127.0.0.1:8080/readyz` calls
// this binary instead. Kubernetes never needs it - its probes are httpGet,
// asked by the kubelet from outside.
//
//	meerkat health                                  readiness on the default port
//	meerkat health http://127.0.0.1:8080/healthz    any address, liveness for one
const healthUsage = `usage:
  meerkat health [URL]   (default: http://127.0.0.1:8080/readyz)

  Exits 0 when the address answers 2xx within three seconds, 1 otherwise.`

const defaultProbe = "http://127.0.0.1:8080/readyz"

func runHealth(args []string, stdout, stderr io.Writer) int {
	target := defaultProbe
	switch {
	case len(args) > 1:
		_, _ = fmt.Fprintln(stderr, healthUsage)
		return 2
	case len(args) == 1:
		target = args[0]
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		_, _ = fmt.Fprintf(stderr, "meerkat health: %q is not an http or https address\n%s\n", target, healthUsage)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "meerkat health:", err)
		return 1
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "meerkat health:", err)
		return 1
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(stdout, io.LimitReader(res.Body, 4<<10))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		_, _ = fmt.Fprintf(stderr, "\nmeerkat health: %s answered %s\n", target, res.Status)
		return 1
	}
	return 0
}
