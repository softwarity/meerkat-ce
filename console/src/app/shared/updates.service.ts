import { DestroyRef, Injectable, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { LivewireClient, LiveTopic } from '@softwarity/livewire';
import { ApiService, Updates } from '../api.service';
import { MeService } from '../me.service';

// Whether a newer release exists (CONSOLE-16). The gateway does the looking,
// once a day and only if root turned it on: this reads what it found, and the
// updates topic says when to read it again. For who can act on an update, which
// is who the API answers: the others never ask.
@Injectable({ providedIn: 'root' })
export class UpdatesService {
  private readonly api = inject(ApiService);
  readonly state = signal<Updates | null>(null);
  // How many releases are ahead of this gateway: the indicator.
  readonly count = computed(() => this.state()?.releases.length ?? 0);

  constructor() {
    const me = inject(MeService);
    const client = inject(LivewireClient);
    const destroyed = inject(DestroyRef);
    void me.ensureLoaded().then(() => {
      if (!me.isInfraAdmin()) return;
      this.read();
      new LiveTopic(client, 'updates')
        .open(null)
        .pipe(takeUntilDestroyed(destroyed))
        .subscribe(() => this.read());
    });
  }

  // Root's. The switch moves at once: turning it on waits for the list, and the
  // answer brings what was found.
  set(enabled: boolean): void {
    const before = this.state();
    if (before) this.state.set({ ...before, enabled });
    this.api.setUpdates(enabled).subscribe({
      next: (u) => this.state.set(u),
      error: () => this.state.set(before),
    });
  }

  private read(): void {
    this.api.updates().subscribe({ next: (u) => this.state.set(u), error: () => {} });
  }
}
