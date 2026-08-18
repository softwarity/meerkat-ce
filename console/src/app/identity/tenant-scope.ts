import { Injectable, inject, signal } from '@angular/core';
import { ApiService, Tenant } from '../api.service';

// Shared state between the tenant layout (left nav + right header) and its
// ROUTED sections: the loaded tenant (the layout owns loading, General updates
// it after a save) and the header search the matrices filter on. Provided by
// TenantPageComponent - one instance per visited tenant layout.
@Injectable()
export class TenantScope {
  readonly tenant = signal<Tenant | null>(null);
  readonly filter = signal('');
  // Groups section: the role tags available (published by the matrix) and the
  // single tag picked in the header ('' = all).
  readonly tags = signal<string[]>([]);
  readonly tagFilter = signal('');

  private readonly api = inject(ApiService);

  // How this organisation's groups combine. It sits in the layout's header,
  // above the matrix it describes, so the write lives here rather than in the
  // section - one owner for the loaded tenant, whoever changes it.
  // The whole tenant goes back: the endpoint takes one object, and a partial
  // one would blank what it left out.
  setGroupMode(mode: string): void {
    const t = this.tenant();
    if (!t || t.groupMode === mode) return;
    this.api.updateTenant({ ...t, groupMode: mode }).subscribe({
      next: (saved) => this.tenant.set(saved),
      error: () => undefined,
    });
  }
}
