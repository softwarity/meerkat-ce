import { DestroyRef, Injectable, Injector, computed, inject, runInInjectionContext, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { LivewireClient, LiveTopic } from '@softwarity/livewire';
import { ApiService, Infrastructure } from '../api.service';
import { MeService } from '../me.service';
import { LiveChangesService } from './live-changes.service';

// What serves the installation and what it serves: what the Topology screen
// draws, and what lights its entry and the rail when something wants a look -
// more instances than the license covers, two versions running at once, an
// instance not ready, a service not answering. Read
// again when the instances topic moves (a node came, went, or changed), never
// on a timer. For who the API answers, root and infra-admin: the others never
// ask.
@Injectable({ providedIn: 'root' })
export class InfrastructureService {
  private readonly api = inject(ApiService);
  readonly state = signal<Infrastructure | null>(null);
  // What deserves the badge.
  readonly warnings = computed(() => {
    const s = this.state();
    if (!s) return 0;
    return (
      (s.over ? 1 : 0) +
      (s.versions.length > 1 ? 1 : 0) +
      (s.instances.some((i) => !i.ready) ? 1 : 0) +
      s.services.filter((v) => v.state === 'down').length
    );
  });

  constructor() {
    const me = inject(MeService);
    const client = inject(LivewireClient);
    const destroyed = inject(DestroyRef);
    const changes = inject(LiveChangesService);
    const injector = inject(Injector);
    void me.ensureLoaded().then(() => {
      if (!me.isRoot() && !me.isInfraAdmin()) return;
      this.read();
      // Somebody's route written anywhere: a service came, went or moved. In
      // the injection context on() needs, and only for a reader the channel
      // answers - anyone else would open a socket only to be refused.
      runInInjectionContext(injector, () => changes.on('route', () => this.read()));
      new LiveTopic(client, 'instances')
        .open(null)
        .pipe(takeUntilDestroyed(destroyed))
        .subscribe(() => this.read());
      // The services behind the routes: a target went up or down, or a route
      // was created, changed or deleted anywhere.
      new LiveTopic(client, 'route-health')
        .open(null)
        .pipe(takeUntilDestroyed(destroyed))
        .subscribe(() => this.read());
    });
  }

  read(): void {
    this.api.infrastructure().subscribe({ next: (s) => this.state.set(s), error: () => {} });
  }
}
