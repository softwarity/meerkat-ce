import { Component, DestroyRef, Injectable, computed, inject, input, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LiveTopic, LivewireClient } from '@softwarity/livewire';
import { ApiService, LiveSession } from '../api.service';

// Who is signed in, live (CONSOLE-07). The sessions themselves are read
// through GET /api/sessions, which decides what this administrator may see;
// the `presence` topic only says that they moved - a sign-in, a sign-out, an
// organisation chosen, or the half-minute tick that catches the other nodes
// of a cluster and the sessions that simply expired.
//
// Provided by the screen that shows it, so it lives and stops with the screen.
@Injectable()
export class PresenceService {
  private readonly api = inject(ApiService);

  // The open sessions, per account. A session still owed a sign-in step is
  // not somebody signed in.
  readonly byUser = signal<Map<string, LiveSession[]>>(new Map());

  constructor() {
    this.read();
    new LiveTopic(inject(LivewireClient), 'presence')
      .open(null)
      .pipe(takeUntilDestroyed(inject(DestroyRef)))
      .subscribe(() => this.read());
  }

  // Every page of the listing: an installation has tens of sessions, not
  // thousands, and a screen that showed the first hundred would call the
  // hundred-and-first person offline.
  private read(offset = 0, acc: LiveSession[] = []): void {
    this.api.listSessions({ limit: 500, offset }).subscribe({
      next: (page) => {
        const all = acc.concat(page.sessions);
        if (page.sessions.length > 0 && all.length < page.total) {
          this.read(all.length, all);
          return;
        }
        const map = new Map<string, LiveSession[]>();
        for (const s of all) {
          if (s.pending) continue;
          map.set(s.userId, [...(map.get(s.userId) ?? []), s]);
        }
        this.byUser.set(map);
      },
      // Not allowed to read sessions, or the gateway is away: nobody is shown
      // as signed in, which is what the screen can honestly say.
      error: () => this.byUser.set(new Map()),
    });
  }
}

const time = (unix: number) => new Date(unix * 1000).toLocaleString();

// The person icon at the head of a row: coloured while the account has a
// session open, and its sessions in the tooltip. `tenantId` narrows it to the
// sessions open IN that organisation - on a Members screen, connected means
// connected here, not somewhere.
@Component({
  selector: 'app-presence',
  imports: [MatIconModule, MatTooltipModule],
  template: `<mat-icon
    class="presence"
    [class.online]="sessions().length > 0"
    [matTooltip]="tip()"
    matTooltipClass="tooltip-lines"
    >person</mat-icon
  >`,
  styles: `
    .presence {
      font-size: 20px;
      width: 20px;
      height: 20px;
      margin-right: 10px;
      flex: 0 0 auto;
      color: var(--mat-sys-outline-variant);
      transition: color 200ms;
    }
    .presence.online {
      color: var(--mat-sys-primary);
    }
  `,
})
export class PresenceComponent {
  readonly userId = input.required<string>();
  readonly tenantId = input<string>('');

  private readonly presence = inject(PresenceService);

  protected readonly sessions = computed(() => {
    const all = this.presence.byUser().get(this.userId()) ?? [];
    const tenant = this.tenantId();
    return tenant ? all.filter((s) => s.tenantId === tenant) : all;
  });

  protected readonly tip = computed(() => {
    const list = this.sessions();
    if (list.length === 0) {
      return this.tenantId()
        ? $localize`:@@Presence_offline_here:Not signed in to this organisation`
        : $localize`:@@Presence_offline:Not signed in`;
    }
    const lines = list.map((s) => {
      const where = s.plane === 'admin' ? $localize`:@@Presence_console:console` : (s.tenantName ?? '');
      const parts = [s.label || $localize`:@@Presence_unknown_browser:browser`, s.ip ?? '', where];
      return `${parts.filter(Boolean).join(' - ')}\n${$localize`:@@Presence_since:since`} ${time(s.createdAt)}`;
    });
    const head =
      list.length === 1
        ? $localize`:@@Presence_one:Signed in, 1 session`
        : $localize`:@@Presence_many:Signed in, ${list.length}:count: sessions`;
    return [head, ...lines].join('\n');
  });
}
