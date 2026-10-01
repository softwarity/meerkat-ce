import { httpResource } from '@angular/common/http';
import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { RouterLink } from '@angular/router';
import { ApiService, Discovery, MetricsSetting } from '../api.service';
import {
  MONITORING,
  PLATFORMS,
  gatewayNetwork,
  monitoringUrl,
  stamp,
  variant,
} from '../metrics/monitoring-files';
import { EeLockComponent } from '../shared/ee-lock.component';
import { FormFieldComponent } from '../shared/form-field.component';
import { SnippetComponent } from '../shared/snippet.component';

// Where a scraper reads this gateway's counters (OBS-05). Infra, beside
// OpenTelemetry: it is wired once, like the collector, and it used to hide in
// a drawer of the Metrics screen, which is read every day for its curves.
//
// Named after what it is, an ENDPOINT, and not after one product: the format
// is read by most monitoring stacks. And it says that it is not the only way
// out - the same counters can be pushed over OTLP from the OpenTelemetry page.
@Component({
  selector: 'app-metrics-endpoint-page',
  imports: [
    EeLockComponent,
    FormFieldComponent,
    MatButtonModule,
    MatExpansionModule,
    MatInputModule,
    MatSlideToggleModule,
    RouterLink,
    SnippetComponent,
  ],
  templateUrl: './metrics-endpoint-page.component.html',
  styleUrl: './metrics-endpoint-page.component.scss',
})
export class MetricsEndpointPageComponent {
  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    this.loadExposure();
  }

  private readonly api = inject(ApiService);
  protected readonly exposed = signal(false);
  protected readonly saving = signal(false);
  private readonly path = signal('/metrics');
  protected readonly exposePath = this.path.asReadonly();
  protected readonly prometheusTip = computed(() =>
    this.exposed()
      ? $localize`:@@Metrics_tip_on:Exposed at ${this.path()}:PATH:`
      : $localize`:@@Metrics_tip_off:Not exposed - nothing scrapes this gateway`,
  );
  // The port the gateway opens for scrapers, as saved, and as being typed.
  // Chosen with the switch; once it is on, a different number is a MOVE and
  // waits for its button, since the old port closes when the new one opens.
  protected readonly metricsPort = signal(9091);
  protected readonly portDraft = signal(9091);
  protected readonly portMoved = computed(
    () => this.exposed() && this.portDraft() !== this.metricsPort(),
  );
  protected readonly requireToken = signal(false);
  // What the gateway said when it refused, kept beside the switch rather than
  // in a snack: "port 9091 is taken" is read while choosing another one.
  protected readonly exposeError = signal('');
  // The network the compose file has to join, asked of the runtime (SVC-02)
  // when the drawer opens rather than on every visit: it is a call to the
  // Docker socket with a five-second budget.
  private readonly runtime = httpResource<Discovery>(() => '/api/services');
  private readonly network = computed(() => gatewayNetwork(this.runtime.value()?.reach ?? []));
  private readonly dataOrigin = signal('');

  // Read once, when the page is built.
  private loadExposure() {
    this.api
      .metricsSetting()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((s) => {
        this.show(s);
        this.portDraft.set(s.metricsPort);
      });
  }

  private show(s: MetricsSetting) {
    this.exposed.set(s.enabled);
    this.requireToken.set(s.requireToken);
    this.metricsPort.set(s.metricsPort);
    this.path.set(s.path);
    this.dataOrigin.set(s.dataOrigin);
  }

  protected expose(enabled: boolean) {
    this.save({ enabled, requireToken: this.requireToken(), metricsPort: this.portDraft() });
  }

  protected askToken(required: boolean) {
    this.save({ enabled: this.exposed(), requireToken: required, metricsPort: this.metricsPort() });
  }

  protected movePort() {
    this.save({ enabled: true, requireToken: this.requireToken(), metricsPort: this.portDraft() });
  }

  private save(cfg: { enabled: boolean; requireToken: boolean; metricsPort: number }) {
    this.saving.set(true);
    this.exposeError.set('');
    this.api
      .setMetricsSetting(cfg)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (s) => {
          this.show(s);
          this.saving.set(false);
        },
        // Put back what the gateway still holds. A switch that stays where the
        // click left it after a refusal is a switch that lies about the state
        // of the product. The number typed stays, with the reason under it.
        error: (e: { error?: { error?: string } }) => {
          this.saving.set(false);
          this.exposeError.set(
            e.error?.error ?? $localize`:@@Metrics_save_failed:The setting could not be saved.`,
          );
          this.api
            .metricsSetting()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe((s) => this.show(s));
        },
      });
  }

  // The examples are FILES (console/public/monitoring), fetched from this
  // gateway rather than built here, and stamped with THIS installation's own
  // path and port on the way through: an example with a placeholder host in
  // it is one the reader has to translate, and the translation is where it
  // goes wrong.
  //
  // A page of its own now, visited to wire a stack up: the files are what it
  // is for, so they are fetched with it.
  private file(name: string) {
    const res = httpResource.text(() => monitoringUrl(name));
    return computed(() => {
      // A comment rather than an empty box: a panel showing nothing at all
      // says nothing about whether there is nothing to show.
      if (res.error()) return `# ${monitoringUrl(name)} did not answer`;
      return stamp(res.value() ?? '', {
        path: this.path(),
        port: String(this.metricsPort()),
        token: this.requireToken(),
        network: this.network(),
        dataOrigin: this.dataOrigin(),
      });
    });
  }
  protected readonly scrape = this.file(MONITORING.scrape);
  protected readonly swarmStack = this.file(MONITORING.swarmStack);
  protected readonly k8sMonitor = this.file(MONITORING.k8sMonitor);
  protected readonly grafanaSource = this.file(MONITORING.grafanaSource);
  protected readonly grafanaDashboards = this.file(MONITORING.grafanaDashboards);
  protected readonly grafanaDashboard = this.file(MONITORING.grafanaDashboard);
  protected readonly grafanaQueries = this.file(MONITORING.grafanaQueries);

  // Shown whole, saved per platform. The panel carries both discoveries so a
  // reader sees what the choice is; the buttons carry away a file that runs.
  protected readonly scrapeVariants = computed(() =>
    PLATFORMS.map((p) => ({
      label: p.label,
      filename: MONITORING.scrape,
      content: variant(this.scrape(), p.key),
    })),
  );
}
