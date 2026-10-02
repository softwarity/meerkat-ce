import { Component, DestroyRef, ElementRef, computed, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { LiveTopic, LivewireClient } from '@softwarity/livewire';
import { ApiService, LogEntry, LogLevel, LogsAnswer } from '../api.service';

// How many lines the screen keeps: what the gateway keeps.
const KEPT = 5000;

const LEVELS: LogLevel[] = ['debug', 'info', 'warn', 'error'];

// The gateway's own log, live (OBS-03).
//
// What the node that answered wrote about itself, read from its buffer: the
// gateway keeps each line structured, so whatever its output format (text,
// JSON, OTel) the screen gets fields and lays them out. The socket only says
// that lines were written; the screen then asks for what came after the last
// one it holds.
//
// The level is the installation's, turned here for every node. One more
// talkative than the startup level goes back on its own after half an hour.
@Component({
  selector: 'app-logs-page',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
  ],
  styleUrl: './logs-page.component.scss',
  templateUrl: './logs-page.component.html',
})
export class LogsPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  protected readonly levels = LEVELS;
  protected readonly loading = signal(true);
  protected readonly entries = signal<LogEntry[]>([]);
  protected readonly state = signal<LogsAnswer | undefined>(undefined);

  // The filters, all on the screen's side: the gateway keeps five thousand
  // lines, which a browser filters faster than a round trip.
  protected readonly shown = signal<LogLevel[]>([...LEVELS]);
  protected readonly search = signal('');
  // Following the newest line, until somebody scrolls up to read.
  protected readonly following = signal(true);
  protected readonly open = signal<number | undefined>(undefined);

  protected readonly filtered = computed(() => {
    const levels = new Set(this.shown());
    const needle = this.search().trim().toLowerCase();
    return this.entries().filter(
      (e) => levels.has(e.level) && (!needle || this.text(e).toLowerCase().includes(needle)),
    );
  });

  // When a talkative level goes back, as "in 28 min", read again every
  // fifteen seconds.
  private readonly now = signal(Date.now());
  protected readonly revertsIn = computed(() => {
    const until = this.state()?.until;
    if (!until) return '';
    const minutes = Math.max(0, Math.round((Date.parse(until) - this.now()) / 60000));
    return $localize`:@@Logs_back_in:Back to ${this.state()?.startup}:level: in ${minutes}:minutes: min`;
  });

  private readonly scroller = viewChild<ElementRef<HTMLElement>>('scroller');
  private last = 0;
  private reading = false;
  private again = false;

  constructor() {
    this.read();
    new LiveTopic(inject(LivewireClient), 'logs')
      .open(null)
      .pipe(takeUntilDestroyed())
      .subscribe(() => this.read());
    const tick = setInterval(() => this.now.set(Date.now()), 15000);
    inject(DestroyRef).onDestroy(() => clearInterval(tick));
  }

  // What came after the last line held. One read at a time: a burst of lines
  // while one is in flight is one more read after it, not one each.
  private read(): void {
    if (this.reading) {
      this.again = true;
      return;
    }
    this.reading = true;
    this.api.logs(this.last).subscribe({
      next: (a) => {
        // A restarted gateway numbers from one again: start over.
        const restarted = a.last < this.last;
        this.state.set(a);
        this.now.set(Date.now());
        const got = a.entries ?? [];
        const fresh = restarted ? got : got.filter((e) => e.seq > this.last);
        this.last = a.last;
        if (restarted || fresh.length) {
          this.entries.update((held) => (restarted ? fresh : [...held, ...fresh]).slice(-KEPT));
          if (this.following()) queueMicrotask(() => this.toBottom());
        }
        this.done();
      },
      error: () => this.done(),
    });
  }

  private done(): void {
    this.loading.set(false);
    this.reading = false;
    if (this.again) {
      this.again = false;
      this.read();
    }
  }

  protected setLevel(level: LogLevel): void {
    this.api.setLogLevel(level).subscribe({
      next: (s) => {
        this.now.set(Date.now());
        this.state.update((a) => (a ? { ...a, ...s, until: s.until } : a));
      },
      error: (err) =>
        this.snack.open(err?.error?.error ?? $localize`:@@Request_failed:Request failed`, undefined, {
          duration: 4000,
        }),
    });
  }

  // Scrolling up to read stops the following; coming back down resumes it.
  protected scrolled(): void {
    const el = this.scroller()?.nativeElement;
    if (!el) return;
    this.following.set(el.scrollHeight - el.scrollTop - el.clientHeight < 24);
  }

  protected follow(): void {
    this.following.set(true);
    this.toBottom();
  }

  private toBottom(): void {
    const el = this.scroller()?.nativeElement;
    if (el) el.scrollTop = el.scrollHeight;
  }

  protected toggle(e: LogEntry): void {
    this.open.update((s) => (s === e.seq ? undefined : e.seq));
  }

  protected time(e: LogEntry): string {
    const d = new Date(e.time);
    const p = (n: number, w = 2) => String(n).padStart(w, '0');
    return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
  }

  protected attrs(e: LogEntry): { key: string; value: string }[] {
    return Object.entries(e.attrs ?? {}).map(([key, v]) => ({
      key,
      value: typeof v === 'string' ? v : JSON.stringify(v),
    }));
  }

  private text(e: LogEntry): string {
    return [e.message, e.traceId ?? '', ...this.attrs(e).map((a) => `${a.key}=${a.value}`)].join(' ');
  }

  // The lines shown, one JSON object each: what a collector or `jq` reads.
  protected download(): void {
    const body = this.filtered()
      .map((e) => JSON.stringify(e))
      .join('\n');
    const url = URL.createObjectURL(new Blob([body + '\n'], { type: 'application/x-ndjson' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `meerkat-${this.state()?.node ?? 'logs'}.jsonl`;
    a.click();
    URL.revokeObjectURL(url);
  }
}
