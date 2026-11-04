import { Service, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService, VaultEntry } from '../api.service';

// The vault entries, loaded once and shared. Every form field offering the
// vault picker reads this signal, so opening ten fields costs one request; a
// creation refreshes it for all of them at once.
@Service()
export class VaultService {
  private readonly api = inject(ApiService);

  readonly entries = signal<VaultEntry[]>([]);
  // The entries this reader can fix that want a look: a name an import brought
  // without its value, or a value past its reminder date. What the rail's
  // badge counts. An inherited entry is not this reader's to fill.
  readonly attention = computed(() => this.entries().filter((e) => !e.readOnly && (!e.hasValue || e.expired)).length);
  private loading?: Promise<VaultEntry[]>;

  // Loads on first call, then serves the cache. Failures (a non-infra admin
  // gets 403) resolve to an empty list: the picker simply offers nothing.
  ensureLoaded(): Promise<VaultEntry[]> {
    if (!this.loading) {
      this.loading = firstValueFrom(this.api.listVault())
        .then((list) => {
          this.entries.set(list);
          return list;
        })
        .catch(() => {
          this.entries.set([]);
          return [];
        });
    }
    return this.loading;
  }

  // Re-reads from the server (after a create, an edit or a delete).
  async reload(): Promise<VaultEntry[]> {
    this.loading = undefined;
    return this.ensureLoaded();
  }
}
