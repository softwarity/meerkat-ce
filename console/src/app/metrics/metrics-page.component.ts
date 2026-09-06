import { DecimalPipe } from '@angular/common';
import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { LiveWindowDataSource } from '@softwarity/livewire';
import type { EChartsCoreOption } from 'echarts/core';
import { ApiService } from '../api.service';
import { EeLockComponent } from '../shared/ee-lock.component';
import { SnippetComponent } from '../shared/snippet.component';
import { ChartComponent } from './chart.component';
import {
  ExampleContext,
  grafanaDatasource,
  grafanaQueries,
  k8sPlainScrape,
  k8sSecret,
  k8sServiceMonitor,
  swarmScrapeConfig,
  swarmSecret,
  swarmStack,
} from './prometheus-examples';
import {
  EndpointsAnswer,
  TrafficRoute,
  TrafficSample,
  TrafficService,
} from './traffic.service';

// What the gateway has actually served (OBS-01).
//
// The console has always shown what is CONFIGURED. This is the other half, and
// the one somebody is looking at when they are on call: a route that is
// failing and a route nobody calls look identical in a configuration.
//
// Fed by the live channel rather than polled: the gateway pushes an interval
// every five seconds, and what crosses the socket is one row a tick because a
// sample never changes once measured.
@Component({
  selector: 'app-metrics-page',
  imports: [
    ChartComponent,
    DecimalPipe,
    EeLockComponent,
    MatButtonModule,
    MatButtonToggleModule,
    MatExpansionModule,
    MatIconModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTooltipModule,
    RouterLink,
    SnippetComponent,
  ],
  templateUrl: './metrics-page.component.html',
  styleUrl: './metrics-page.component.scss',
})
export class MetricsPageComponent {
  private readonly traffic = inject(TrafficService);

  // Built here rather than in the service, and that is the library's own rule:
  // the source repaints the view it was created in, so one built outside a
  // component has nobody to answer.
  protected readonly source = new LiveWindowDataSource<TrafficSample>(
    () => this.traffic.resync(),
    200,
  );
  private readonly rows = toSignal(this.source.changes, { initialValue: [] });

  private readonly destroyRef = inject(DestroyRef);
  private readonly perEndpoint = signal<EndpointsAnswer>({});

  constructor() {
    this.source.reset((offset, limit) => this.traffic.window(offset, limit));
    // Every fifteen seconds, whether or not a route is open.
    //
    // It used to refresh only while one WAS open, which was a deadlock the
    // moment endpoints started being deduced: a deduced template appears with
    // the traffic, so a route that went quiet at page load never grew a
    // chevron, and without a chevron there was nothing to open to trigger the
    // refresh that would have grown it. Fifteen seconds of one line per
    // endpoint is a few kilobytes; the rate is what keeps it off the socket,
    // not the condition.
    this.loadEndpoints();
    const timer = setInterval(() => this.loadEndpoints(), 15_000);
    this.destroyRef.onDestroy(() => clearInterval(timer));
    this.loadExposure();
  }

  // Asked for over the period the table is DRAWING, so a route's endpoints add
  // up to the route's own row. They used to be totals since the gateway
  // started, and a reader was left with a row saying one failure over the last
  // seventeen minutes and three failing endpoints under it.
  private loadEndpoints() {
    const from = this.samples()[0];
    this.traffic
      .endpoints(from ? new Date(from.at).getTime() : 0)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((answer) => this.perEndpoint.set(answer));
  }

  protected readonly samples = computed(() =>
    this.rows().filter((r): r is TrafficSample => !!r),
  );
  protected readonly latest = computed(() => this.samples().at(-1));
  // How much of the window is actually there, so the table below can say what
  // it ranks over rather than implying an hour it may not have.
  protected readonly coveredMinutes = computed(() => {
    const seconds = this.samples().reduce((n, s) => n + s.seconds, 0);
    return Math.max(1, Math.round(seconds / 60));
  });

  // How many gateways the curves cover. NOT announced: a cluster summing its
  // nodes is what anybody expects, and saying so is a line every reader pays
  // for. It is read only to say the one thing that is NOT expected - that an
  // endpoint is counted on the node that answered, not across the cluster.
  protected readonly nodes = computed(() => this.latest()?.nodes ?? 0);

  // ── the numbers above the charts ──────────────────────────────────────────
  //
  // Over the LAST MINUTE, not over the window. The charts tell the story; these
  // four answer "how bad is it right now", and averaging seventeen minutes
  // makes a burst that stopped a quarter of an hour ago go on dominating the
  // headline long after it ended. The period is written on the screen rather
  // than left to be guessed.
  protected readonly recentSeconds = 60;
  private readonly recent = computed(() => {
    const samples = this.samples();
    const want = Math.ceil(this.recentSeconds / Math.max(samples.at(-1)?.seconds ?? 5, 1));
    return samples.slice(-want);
  });
  private readonly totals = computed(() => {
    let requests = 0;
    let errors = 0;
    let seconds = 0;
    let spent = 0;
    for (const s of this.recent()) {
      seconds += s.seconds;
      for (const r of s.routes) {
        const all = r.byClass.reduce((a, b) => a + b, 0);
        requests += all;
        errors += (r.byClass[5] ?? 0) + (r.byClass[4] ?? 0);
        spent += r.sumSecs;
      }
    }
    return { requests, errors, seconds, spent };
  });
  protected readonly perSecond = computed(() => {
    const { requests, seconds } = this.totals();
    return seconds > 0 ? requests / seconds : 0;
  });
  protected readonly errorRate = computed(() => {
    const { requests, errors } = this.totals();
    return requests > 0 ? (errors / requests) * 100 : 0;
  });
  protected readonly meanMs = computed(() => {
    const { requests, spent } = this.totals();
    return requests > 0 ? (spent / requests) * 1000 : 0;
  });

  // ── traffic, by status class ─────────────────────────────────────────────
  protected readonly trafficOption = computed<EChartsCoreOption>(() => {
    const samples = this.samples();
    const at = samples.map((s) => new Date(s.at).getTime());
    // Stacked, because the question is "how much, and how much of it went
    // wrong" - two lines side by side make the reader do the addition.
    const classAt = (s: TrafficSample, i: number) =>
      s.routes.reduce((n, r) => n + (r.byClass[i] ?? 0), 0) / Math.max(s.seconds, 1);
    return this.lines(at, [
      { name: $localize`:@@Metrics_ok:Answered`, colour: '#4caf50', data: samples.map((s) => classAt(s, 2) + classAt(s, 3)) },
      { name: $localize`:@@Metrics_refused:Refused`, colour: '#ffa726', data: samples.map((s) => classAt(s, 4)) },
      { name: $localize`:@@Metrics_failed:Failed`, colour: '#ef5350', data: samples.map((s) => classAt(s, 5)) },
    ]);
  });

  // ── latency ──────────────────────────────────────────────────────────────
  protected readonly latencyOption = computed<EChartsCoreOption>(() => {
    const samples = this.samples();
    const at = samples.map((s) => new Date(s.at).getTime());
    const mean = samples.map((s) => {
      let n = 0;
      let spent = 0;
      for (const r of s.routes) {
        n += r.byClass.reduce((a, b) => a + b, 0);
        spent += r.sumSecs;
      }
      return n > 0 ? (spent / n) * 1000 : 0;
    });
    return this.lines(at, [
      { name: $localize`:@@Metrics_mean_latency:Mean`, colour: '#42a5f5', data: mean },
    ]);
  });

  // ── the ranking, on three axes ───────────────────────────────────────────
  //
  // Three questions, one table. The columns are the same for all three - only
  // the order changes - so three tables side by side would repeat the same
  // headers three times and put thirty rows of dense numbers in front of
  // somebody who, mid-incident, reads one.
  //
  // The axes are the ones a request-driven service is read on (the RED method:
  // rate, errors, duration), plus the one people reach for first and dashboards
  // usually leave out: the TIME SPENT. A route answering in 50 ms ten thousand
  // times costs this gateway more than one taking five seconds twice, and it is
  // neither the slowest nor the busiest.
  //
  // The selector carries the signal, which is what a selector normally fails to
  // do: the failing tab shows its count, so an eye is caught without a click.
  protected readonly axis = signal<'slow' | 'failing' | 'costly'>('slow');
  protected readonly axes = [
    { key: 'slow' as const, label: $localize`:@@Metrics_axis_slow:Slowest` },
    { key: 'failing' as const, label: $localize`:@@Metrics_axis_failing:Failing` },
    { key: 'costly' as const, label: $localize`:@@Metrics_axis_costly:Costliest` },
  ];

  protected readonly ranked = computed(() => {
    const by = new Map<string, { name: string; requests: number; errors: number; spent: number }>();
    for (const s of this.samples()) {
      for (const r of s.routes) {
        const row = by.get(r.id) ?? { name: r.name || r.id, requests: 0, errors: 0, spent: 0 };
        row.requests += r.byClass.reduce((a, b) => a + b, 0);
        row.errors += (r.byClass[5] ?? 0) + (r.byClass[4] ?? 0);
        row.spent += r.sumSecs;
        by.set(r.id, row);
      }
    }
    return [...by.entries()]
      .map(([id, r]) => ({
        id,
        ...r,
        meanMs: r.requests > 0 ? (r.spent / r.requests) * 1000 : 0,
      }))
      .filter((r) => r.requests > 0)
      .sort((a, b) => {
        switch (this.axis()) {
          case 'failing':
            return b.errors - a.errors || b.requests - a.requests;
          case 'costly':
            return b.spent - a.spent;
          default:
            return b.meanMs - a.meanMs;
        }
      })
      .slice(0, 10);
  });

  // How many requests were refused or failed over the covered period. On the
  // selector, so nobody has to switch to find out there is nothing to switch
  // for - or that there is.
  protected readonly failingCount = computed(() =>
    this.ranked().reduce((n, r) => n + r.errors, 0),
  );

  // ── which endpoint of the route ──────────────────────────────────────────
  //
  // A route is a hundred endpoints, and "this route is slow" is where the
  // question starts, not where it ends. Opening a row answers the next one.
  //
  // Opened rather than shown as a second table: an endpoint only means
  // something under the route it belongs to, and two hundred rows of
  // "GET /x on route Y" is a list nobody reads mid-incident. One route at a
  // time, and the table stays the size of the routing table.
  protected readonly open = signal<string | null>(null);
  protected toggle(id: string) {
    this.open.update((current) => (current === id ? null : id));
    // Opened: answer now rather than up to fifteen seconds from now.
    if (this.open()) this.loadEndpoints();
  }

  // The routes that have something to open. Every route with traffic does now:
  // a spec names its endpoints exactly, and without one the shape of the paths
  // is folded into templates instead. What is left out is a route this node
  // has not seen a request for since it started counting.
  protected readonly named = computed(
    () => new Set((this.perEndpoint().endpoints ?? []).map((e) => e.routeId)),
  );

  protected readonly openEndpoints = computed(() => {
    const id = this.open();
    if (!id) return [];
    return (this.perEndpoint().endpoints ?? [])
      .filter((e) => e.routeId === id)
      .map((e) => ({
        key: `${e.method} ${e.path}`,
        method: e.method,
        path: e.path,
        deduced: !!e.deduced,
        requests: e.requests,
        errors: e.errors,
        spent: e.sumSecs,
        meanMs: e.requests > 0 ? (e.sumSecs / e.requests) * 1000 : 0,
      }))
      .filter((e) => e.requests > 0)
      .sort((a, b) => {
        // The same axis as the table it hangs under. Two orders on one screen
        // would make the reader ask which one they are reading.
        switch (this.axis()) {
          case 'failing':
            return b.errors - a.errors || b.requests - a.requests;
          case 'costly':
            return b.spent - a.spent;
          default:
            return b.meanMs - a.meanMs;
        }
      })
      .slice(0, 10);
  });

  // ── the Prometheus half ──────────────────────────────────────────────────
  // A drawer off a button in the header: it is read ONCE, when somebody wires
  // a monitoring stack up, and a page carrying it at the bottom forever makes
  // every reader scroll past an explanation they have already had.
  protected readonly prometheusOpen = signal(false);

  private readonly api = inject(ApiService);
  protected readonly exposed = signal(false);
  protected readonly saving = signal(false);
  private readonly path = signal('/metrics');
  protected readonly exposePath = this.path.asReadonly();

  // Read once, when the screen is built rather than when the drawer opens: it
  // is one small call, and a switch that arrives after the drawer does flickers
  // from off to on in front of whoever opened it.
  private loadExposure() {
    this.api
      .metricsSetting()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((s) => {
        this.exposed.set(s.enabled);
        this.path.set(s.path);
      });
  }

  protected expose(enabled: boolean) {
    this.saving.set(true);
    this.api
      .setMetricsSetting(enabled)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (s) => {
          this.exposed.set(s.enabled);
          this.saving.set(false);
        },
        // Put back what the gateway still holds. A switch that stays where the
        // click left it after a refusal is a switch that lies about the state
        // of the product.
        error: () => {
          this.saving.set(false);
          this.loadExposure();
        },
      });
  }

  // The examples, carrying THIS installation's own address: one with a
  // placeholder host in it is one the reader has to translate, and the
  // translation is where it goes wrong.
  private ctx(): ExampleContext {
    return { origin: window.location.origin, path: this.path() };
  }
  protected readonly swarmSecret = swarmSecret();
  protected swarmScrape = () => swarmScrapeConfig(this.ctx());
  protected swarmStack = () => swarmStack(this.ctx());
  protected readonly k8sSecret = k8sSecret();
  protected k8sMonitor = () => k8sServiceMonitor(this.ctx());
  protected k8sScrape = () => k8sPlainScrape(this.ctx());
  protected readonly grafanaSource = grafanaDatasource();
  protected readonly grafanaQueries = grafanaQueries();

  private lines(
    at: number[],
    series: { name: string; colour: string; data: number[] }[],
  ): EChartsCoreOption {
    return {
      animation: false,
      grid: { left: 48, right: 12, top: 24, bottom: 24 },
      tooltip: { trigger: 'axis' },
      legend: { top: 0, textStyle: { color: '#9aa4c0' } },
      xAxis: {
        type: 'time',
        axisLabel: { color: '#9aa4c0' },
        splitLine: { show: false },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#9aa4c0' },
        splitLine: { lineStyle: { color: 'rgba(154,164,192,0.15)' } },
      },
      series: series.map((s) => {
        const stacked = series.length > 1;
        return {
          name: s.name,
          type: 'line',
          stack: stacked ? 'total' : undefined,
          // Stacked bands are read by their FILL, and their stroke lies. Each
          // series' line is drawn at the running total, so a series
          // contributing nothing still paints its colour along the boundary
          // below it - and being drawn last, the worst one covers the rest.
          // A chart claiming everything failed while nothing did is worse than
          // no chart. So: no stroke when stacked, and the bands say it.
          lineStyle: stacked ? { width: 0 } : { width: 2 },
          areaStyle: stacked ? { opacity: 0.55 } : undefined,
          showSymbol: false,
          smooth: 0.2,
          itemStyle: { color: s.colour },
          data: at.map((t, i) => [t, s.data[i] ?? 0]),
        };
      }),
    };
  }

  protected trackRoute = (_: number, r: { id: string }) => r.id;
  protected readonly routeOf = (r: TrafficRoute) => r.name || r.id;
}
