import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { LiveTopic, LivewireClient } from '@softwarity/livewire';
import type { LiveRow } from '@softwarity/livewire';

// One interval of traffic, as the gateway measured it.
//
// The id IS the instant, and so is the version: a sample never changes once
// measured, so a row already on the screen is never sent again. What crosses
// the socket on a tick is one row, not the window.
export interface TrafficSample extends LiveRow {
  at: string;
  // How many seconds this interval covers. Sent rather than assumed: a
  // sampler that ran late must not read as a spike.
  seconds: number;
  routes: TrafficRoute[];
  inFlight: number;
  logins: number;
  refused: number;
  unmatched: number;
  // How many gateways this interval covers. One on a plain installation; on a
  // cluster it is what stops a partial curve passing for a total.
  nodes: number;
}

export interface TrafficRoute {
  id: string;
  name: string;
  // Requests by status class: index 2 is 2xx, 4 is 4xx, 5 is 5xx.
  byClass: number[];
  // The latency histogram, one count per boundary, plus a last one for what
  // fell past the final boundary.
  buckets: number[];
  // Seconds spent answering, summed. With the count, this is the mean; the
  // buckets are what a percentile is read from.
  sumSecs: number;
  // Upstream failures: connect, timeout, refused by the breaker, 5xx.
  failures: number[];
}

// One endpoint over the SAME period the table above it covers.
//
// Asked for by period rather than served as a running total, and that is what
// makes the two readable together: a route's endpoints have to add up to the
// route's own row, or the reader is left doing arithmetic that does not work.
//
// Three numbers, because a ranking is read on how much, how much of it went
// wrong, and how long it took. The endpoints keep a coarser history than the
// curves do - one point a minute - so the finer analysis is what a Prometheus
// is for.
// The histogram's upper bounds, in seconds - the same eleven the gateway
// counts with (internal/metrics.Buckets), plus the overflow past the last.
export const LATENCY_BOUNDS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10];

// A percentile, in seconds, read from bucket counts (one count per bucket, not
// cumulative) the way Prometheus's histogram_quantile reads them: find the
// bucket the rank falls in and interpolate linearly inside it. An estimate -
// the true value is somewhere in that bucket - which is why the screen says
// "p95" beside a figure and never pretends to the millisecond past it. A rank
// in the overflow answers the last bound: "slower than ten seconds" is all
// the histogram knows. Null when nothing was counted.
export function quantile(buckets: number[], q: number): number | null {
  const total = buckets.reduce((a, b) => a + b, 0);
  if (total === 0) return null;
  const rank = q * total;
  let below = 0;
  for (let i = 0; i < buckets.length; i++) {
    const n = buckets[i] ?? 0;
    if (n > 0 && below + n >= rank) {
      if (i >= LATENCY_BOUNDS.length) return LATENCY_BOUNDS[LATENCY_BOUNDS.length - 1];
      const lower = i === 0 ? 0 : LATENCY_BOUNDS[i - 1];
      return lower + (LATENCY_BOUNDS[i] - lower) * ((rank - below) / n);
    }
    below += n;
  }
  return LATENCY_BOUNDS[LATENCY_BOUNDS.length - 1];
}

// The buckets of several routes (or samples), added up.
export function sumBuckets(routes: TrafficRoute[]): number[] {
  const out = new Array<number>(LATENCY_BOUNDS.length + 1).fill(0);
  for (const r of routes) r.buckets.forEach((n, i) => (out[i] += n));
  return out;
}

export interface EndpointTotals {
  routeId: string;
  method: string;
  // The operation's TEMPLATE, in the spec's own coordinates - /delay/{delay},
  // never /delay/3. One series per order is how a monitoring stack is brought
  // down by the thing meant to watch it.
  path: string;
  requests: number;
  errors: number;
  sumSecs: number;
  // True when the gateway inferred this template from the SHAPE of the paths
  // it saw rather than reading it from something somebody wrote. Bounded, and
  // sometimes wrong - the 2024 in /files/2024/report is a year, not an id - so
  // the screen never presents it as the same kind of fact as a spec.
  deduced?: boolean;
}

export interface EndpointsAnswer {
  // Where the answer actually starts: what was asked for, or when this node
  // started counting when that is later.
  since?: string;
  endpoints?: EndpointTotals[];
}

// The live traffic window.
//
// A service and not a component field, because the topic is the screen's one
// subscription and the component is rebuilt whenever somebody navigates away
// and back. Nothing here knows about charts.
@Injectable({ providedIn: 'root' })
export class TrafficService {
  private readonly topic = new LiveTopic<TrafficSample>(inject(LivewireClient), 'traffic');

  // The window the screen watches. Bounded by what the gateway keeps rather
  // than by a scroll position - the source answers the same list whatever
  // offset is asked - but it goes through `window` all the same, because that
  // is the shape LiveWindowDataSource drives and the patch handling comes with
  // it.
  window = (offset: number, limit: number) => this.topic.window({}, offset, limit);
  resync = () => this.topic.resync();

  private readonly http = inject(HttpClient);

  // The endpoint ranking, asked for rather than pushed: it is one figure per
  // endpoint over a period, not a curve, so a socket carrying it every five
  // seconds would be sending the same numbers plus a little. `minutes=0` says
  // the window is not wanted with it - the curves already come down the socket.
  //
  // `since` is the period the screen is DRAWING, in milliseconds, so what a
  // route's endpoints add up to is what the route's own row says.
  endpoints = (since: number) =>
    this.http.get<EndpointsAnswer>('/api/metrics', {
      params: { minutes: 0, endpoints: 1, since },
    });
}
