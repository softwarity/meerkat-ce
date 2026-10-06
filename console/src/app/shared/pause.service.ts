import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { LivewireClient, LiveTopic } from '@softwarity/livewire';
import { ApiService } from '../api.service';

// Whether the gateway is paused for a database move (Configuration > Snapshot).
// One state for the whole console: the shell shows it to everybody, since it
// explains every change the gateway now refuses, and the Snapshot tab turns it
// on and off. Live: the pause topic says it moved, and it is read again.
@Injectable({ providedIn: 'root' })
export class PauseService {
  private readonly api = inject(ApiService);
  readonly paused = signal(false);

  constructor() {
    this.read();
    new LiveTopic(inject(LivewireClient), 'pause')
      .open(null)
      .pipe(takeUntilDestroyed(inject(DestroyRef)))
      .subscribe(() => this.read());
  }

  set(paused: boolean): void {
    const before = this.paused();
    this.paused.set(paused);
    this.api.setPause(paused).subscribe({ error: () => this.paused.set(before) });
  }

  private read(): void {
    this.api.getPause().subscribe({ next: (p) => this.paused.set(p.paused), error: () => {} });
  }
}
