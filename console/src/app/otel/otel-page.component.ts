import { httpResource } from '@angular/common/http';
import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { ApiService, TelemetryConfig, TelemetrySetting } from '../api.service';
import { EeLockComponent } from '../shared/ee-lock.component';
import { FormFieldComponent } from '../shared/form-field.component';
import { SecretFieldComponent } from '../shared/secret-field.component';
import { SnippetComponent } from '../shared/snippet.component';
import { TRACING_PLATFORMS, tracingUrl } from './tracing-files';

// Where this gateway's traces go (OBS-04): a section of Infra, beside the other
// things this installation is WIRED to.
//
// It used to be a drawer on the Metrics screen, beside the Prometheus one, on
// the grounds that both answer an operator's question from two sides. What that
// arrangement hid is that this is a connection to a system of your own - like
// the mail relay, like TLS - and that WHICH routes are traced is decided on the
// routes themselves. The screen that configures the pipe belongs with the pipes.
//
// WHAT THIS SCREEN DOES NOT DO is choose a backend. We speak OTLP; the address
// points at whatever they already run - an OpenTelemetry Collector, Tempo,
// Jaeger, a vendor. The compose file below is for whoever has none yet, and it
// is an example rather than a dependency. It does not say which routes are
// traced either: that is the route's own OpenTelemetry section.
@Component({
  selector: 'app-otel-page',
  imports: [
    EeLockComponent,
    FormFieldComponent,
    MatButtonModule,
    MatExpansionModule,
    MatIconModule,
    MatInputModule,
    MatSlideToggleModule,
    MatTooltipModule,
    RouterLink,
    SecretFieldComponent,
    SnippetComponent,
  ],
  styleUrl: './otel-page.component.scss',
  templateUrl: './otel-page.component.html',
})
export class OtelPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly saving = signal(false);
  protected readonly loaded = signal(false);
  protected readonly enterprise = signal(true);
  // Whether spans are LEAVING, which is not whether the box is ticked: an
  // address the exporter refused leaves the setting on and the pipe shut, and
  // an operator staring at an empty Jaeger deserves to be told which it is.
  protected readonly exporting = signal(false);
  // Whether a save actually happened in this drawer. The warning below says
  // "saved, but nothing is leaving", and saying that before anybody saved is
  // the screen announcing something it did not do.
  protected readonly justSaved = signal(false);
  protected readonly error = signal('');

  protected readonly enabled = signal(false);
  protected readonly traces = signal(true);
  protected readonly metrics = signal(false);
  protected readonly pushingMetrics = signal(false);
  // Whatever is switched on and not leaving, which is what the warning says.
  protected readonly stalled = computed(
    () =>
      this.enabled() &&
      ((this.traces() && !this.exporting()) || (this.metrics() && !this.pushingMetrics())),
  );
  protected readonly endpoint = signal('');
  protected readonly sample = signal(0.1);
  protected readonly maxPerSecond = signal(200);
  protected readonly gatewayDetail = signal(false);
  // One header, which is what a collector wants: a name and a value, and the
  // value is a vault reference rather than a key. More than one is rare enough
  // that the API takes a map and this screen offers the common case.
  protected readonly headerName = signal('');
  protected readonly headerValue = signal('');
  // A literal is stored and was not sent here. Without this the field would
  // look unset, and an empty-looking credential invites somebody to retype one.
  protected readonly headerSet = signal(false);
  // Where the server finds that literal, so it can move it into the vault on
  // its own - the console never received it and could not do it otherwise.
  protected readonly headerAt = computed(() => ({
    holder: 'telemetry' as const,
    id: '',
    field: this.headerName().trim(),
  }));

  // A share is stored 0..1 and read as a percentage: nobody thinks in
  // tenths, and "10%" is the number in every other tracing product.
  //
  // ONE rate, for the gateway and for the injected bundle alike. A second one
  // for the browser read like a choice and was not: the gateway honours a
  // page's sampling decision rather than rolling again, so the browser number
  // quietly decided every request coming from an instrumented page and the
  // gateway number described a population the operator was not thinking of.
  protected readonly samplePercent = computed(() => Math.round(this.sample() * 100));

  constructor() {
    this.load();
  }

  // After a server-side move into the vault: the setting was rewritten under
  // us, so the screen reads it again rather than keeping a stale copy.
  protected reload() {
    this.load();
  }

  private load() {
    this.api
      .telemetrySetting()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((s) => this.take(s));
  }

  private take(s: TelemetrySetting) {
    this.enabled.set(s.enabled);
    this.traces.set(s.traces);
    this.metrics.set(!!s.metrics);
    this.pushingMetrics.set(!!s.pushingMetrics);
    this.endpoint.set(s.endpoint ?? '');
    this.sample.set(s.sample);
    this.maxPerSecond.set(s.maxPerSecond);
    this.gatewayDetail.set(!!s.gatewayDetail);
    this.enterprise.set(s.enterprise);
    this.exporting.set(s.exporting);
    const [name, value] = Object.entries(s.headers ?? {})[0] ?? ['', ''];
    this.headerName.set(name);
    this.headerValue.set(value);
    this.headerSet.set(!!s.headerSet);
    this.loaded.set(true);
    this.error.set('');
    this.justSaved.set(false);
  }

  protected setPercent(raw: string) {
    const n = Number(raw);
    if (Number.isNaN(n)) return;
    this.justSaved.set(false);
    this.sample.set(Math.min(100, Math.max(0, n)) / 100);
  }

  // Any local change makes the last verdict stale: the warning speaks about
  // what the gateway did with what it was given, not about what is on screen.
  protected touched() {
    this.justSaved.set(false);
  }

  protected setCeiling(raw: string) {
    const n = Number(raw);
    if (!Number.isNaN(n) && n >= 0) this.maxPerSecond.set(Math.round(n));
    this.justSaved.set(false);
  }

  // The two toggles APPLY ON CLICK, like the Prometheus switch next door: a
  // switch that needs a second button is a switch somebody leaves half-set.
  //
  // The fields keep the Save button, because saving on every keystroke of an
  // address is a gateway reconfigured a dozen times while somebody types it.
  // Applies on click, like the export switch: turning the detail on is done to
  // look at something that is happening now.
  protected toggleDetail(on: boolean) {
    this.gatewayDetail.set(on);
    this.commit();
  }

  protected toggleExport(on: boolean) {
    this.enabled.set(on);
    // Switched on with neither signal chosen, it sends the traces: that is
    // what this page was for before it carried two, and an export that
    // sends nothing is refused.
    if (on && !this.traces() && !this.metrics()) this.traces.set(true);
    this.commit();
  }

  protected toggleTraces(on: boolean) {
    this.traces.set(on);
    this.commit();
  }

  protected toggleMetrics(on: boolean) {
    this.metrics.set(on);
    this.commit();
  }

  // Put back what the gateway still holds when it refuses. A switch that stays
  // where the click left it after a refusal is a switch that lies about the
  // state of the product.
  private commit() {
    this.save({ revertOnError: true });
  }

  // What the gateway REFUSED, kept across the reload that puts the switches
  // back. take() clears the error as part of showing a fresh state, so the
  // reason has to be re-stated after it - otherwise the toggle springs back
  // and the screen says nothing at all, which is what it did.
  private revert(reason: string) {
    this.api
      .telemetrySetting()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((s) => {
        this.take(s);
        this.error.set(reason);
      });
  }

  // What the last test said, kept beside the button rather than in a snack:
  // somebody who is fixing an address tries it twice in a row and wants the
  // previous answer still on screen while they type.
  protected readonly probing = signal(false);
  protected readonly probeOk = signal('');
  protected readonly probeFailed = signal('');

  protected test() {
    this.probing.set(true);
    this.probeOk.set('');
    this.probeFailed.set('');
    const headers: Record<string, string> = {};
    if (this.headerName().trim() && this.headerValue().trim()) {
      headers[this.headerName().trim()] = this.headerValue().trim();
    }
    this.api
      .testTelemetry({ endpoint: this.endpoint().trim(), headers, metrics: this.metrics() })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (r) => {
          this.probing.set(false);
          this.probeOk.set(
            $localize`:@@Tracing_probe_ok:The collector answered ${r.status}:status: in ${r.ms}:ms:ms.`,
          );
        },
        // The gateway's own sentence: it names what answered and, on the two
        // mistakes that actually happen, what to look at.
        error: (e: { error?: { error?: string } }) => {
          this.probing.set(false);
          this.probeFailed.set(
            e.error?.error ?? $localize`:@@Tracing_probe_failed:The collector could not be reached.`,
          );
        },
      });
  }

  protected save(opts: { revertOnError?: boolean } = {}) {
    this.saving.set(true);
    this.error.set('');
    const headers: Record<string, string> = {};
    if (this.headerName().trim() && this.headerValue().trim()) {
      headers[this.headerName().trim()] = this.headerValue().trim();
    }
    const cfg: TelemetryConfig = {
      enabled: this.enabled(),
      traces: this.traces(),
      metrics: this.metrics(),
      endpoint: this.endpoint().trim(),
      headers,
      sample: this.sample(),
      maxPerSecond: this.maxPerSecond(),
      gatewayDetail: this.gatewayDetail(),
    };
    this.api
      .setTelemetrySetting(cfg)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (s) => {
          this.take(s);
          this.justSaved.set(true);
          this.saving.set(false);
          this.snack.open(
            s.exporting || s.pushingMetrics
              ? $localize`:@@Tracing_saved_on_any:Exporting to ${s.endpoint}:endpoint:`
              : $localize`:@@Tracing_saved_off:Saved. Nothing is being exported.`,
            undefined,
            { duration: 4000 },
          );
        },
        // The gateway's own sentence, not a translated guess: it names what is
        // wrong and what is allowed, which is the half a generic failure
        // message throws away.
        error: (e: { error?: { error?: string } }) => {
          this.saving.set(false);
          const reason =
            e.error?.error ?? $localize`:@@Tracing_save_failed:The setting could not be saved.`;
          if (opts.revertOnError) {
            this.revert(reason);
            return;
          }
          this.error.set(reason);
        },
      });
  }

  // For whoever has no collector yet, per platform - and as real FILES on
  // disk rather than strings here, so `curl <gateway>/tracing/swarm/
  // docker-compose.yml` gets the file with nothing in the way.
  protected readonly platforms = TRACING_PLATFORMS;

  private readonly files = new Map<string, () => string>(
    TRACING_PLATFORMS.map((p) => {
      // Fetched when its PANEL opens, not when the drawer does: two files
      // behind every visit to a screen read for its curves is two requests
      // nobody asked for. An undefined url is a request httpResource does not
      // make, and that is what a closed panel returns.
      const res = httpResource.text(() =>
        this.opened() === p.key ? tracingUrl(p.file) : undefined,
      );
      return [
        p.key,
        () => {
          // A comment rather than an empty box: a panel showing nothing at all
          // says nothing about whether there is nothing to show.
          if (res.error()) return `# ${tracingUrl(p.file)} did not answer`;
          return res.value() ?? '';
        },
      ];
    }),
  );

  // Which panel is open, because that is what decides which file is fetched.
  protected readonly opened = signal('');

  protected fileOf(key: string): string {
    return this.files.get(key)?.() ?? '';
  }
}
