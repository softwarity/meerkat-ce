import { Component, computed, inject, signal } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSidenavModule } from '@angular/material/sidenav';
import { ApiService, Tenant } from '../../api.service';
import { MeService } from '../../me.service';
import { MembersMatrixComponent } from '../members-matrix/members-matrix.component';
import { IdentityDrawer } from '../identity-drawer';
import { IdentityDrawerPanelComponent } from '../identity-drawer-panel.component';

// Who is in which group, seen from Application.
//
// Users and this screen answer two different questions and stay apart: Users
// is the account - who exists, what they may do across the whole gateway -
// while this is the assignment. The matrix is the same as an organisation's
// in multi mode: single mode is ONE organisation, with members and their
// administrators like any other.
@Component({
  selector: 'app-app-members',
  // The identity drawer - a member, a group - is the page's, drawn over its
  // whole height.
  providers: [IdentityDrawer],
  host: { '(document:keydown.escape)': 'drawer.escape()' },
  imports: [MatFormFieldModule, MatInputModule, MatSidenavModule, MembersMatrixComponent, IdentityDrawerPanelComponent],
  template: `
    <mat-drawer-container (backdropClick)="drawer.back()">
      <mat-drawer-content>
        <div class="banner">
          <h1 i18n="@@Members">Members</h1>
        </div>

        <div class="toolbar">
          <mat-form-field class="search" subscriptSizing="dynamic">
            <mat-label i18n="@@Search">Search</mat-label>
            <input matInput [value]="filter()" (input)="filter.set($any($event.target).value)" />
          </mat-form-field>
        </div>

        @if (tenantId(); as id) {
          <div class="panel">
            <app-members-matrix [tenantId]="id" [ownerId]="ownerId()" [filter]="filter()" />
          </div>
        }
      </mat-drawer-content>

      <mat-drawer position="end" mode="over" [opened]="drawer.opened()" disableClose>
        @if (tenantId(); as id) {
          <app-identity-drawer-panel [tenantId]="id" />
        }
      </mat-drawer>
    </mat-drawer-container>
  `,
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        min-height: 0;
      }
      mat-drawer-container {
        flex: 1 1 auto;
        min-height: 0;
        background: transparent;
      }
      mat-drawer-content {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow: hidden;
      }
      mat-drawer {
        width: min(560px, 96vw);
        border-left: 1px solid var(--mat-sys-outline-variant);
        background: var(--mat-sys-surface-container-high);
      }
      .banner {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 12px 24px;
        flex: none;
      }
      .banner h1 {
        font-size: 1.15rem;
        font-weight: 500;
        margin: 0;
        flex: 1;
      }
      .toolbar {
        flex: none;
        padding: 0 24px 12px;
      }
      .search {
        width: 100%;
        max-width: 420px;
      }
      /* The same frame the organisation page gives its sections: without it
         the matrix touched the window edges here and nowhere else. */
      .panel {
        flex: 1 1 auto;
        min-height: 0;
        min-width: 0;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        padding: 0 24px 20px;
      }
      .panel > * {
        flex: 1 1 auto;
        min-height: 0;
      }
    `,
  ],
})
export class AppMembersComponent {
  private readonly api = inject(ApiService);
  private readonly me = inject(MeService);
  protected readonly drawer = inject(IdentityDrawer);

  protected readonly filter = signal('');
  private readonly tenant = signal<Tenant | null>(null);

  protected readonly tenantId = computed(() => this.tenant()?.id ?? this.me.primaryTenant());
  protected readonly ownerId = computed(() => this.tenant()?.ownerId ?? '');

  constructor() {
    const id = this.me.primaryTenant();
    if (id) {
      this.api.getTenant(id).subscribe({ next: (t) => this.tenant.set(t), error: () => undefined });
    }
  }
}
