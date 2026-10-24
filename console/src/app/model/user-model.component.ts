import { Component, computed, inject, signal, WritableSignal } from '@angular/core';
import { concat, tap } from 'rxjs';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, UserFieldDef } from '../api.service';
import { MeService } from '../me.service';
import { LiveChangesService } from '../shared/live-changes.service';
import { FieldDefsComponent } from './field-defs.component';

// Infra > Model: the SHAPE of the objects this installation keeps - the
// account, and with several organisations, the organisation (MODEL-03).
//
// Defining a field and filling it are two different acts by two different
// people: one says "this installation records a cost centre", the other says
// "Alice's is B200". So the definition lives in the infrastructure plane and
// the values on the accounts screen - which is also what makes a custom field
// safe to forward, since nobody can grant themselves an attribute a service
// trusts.
@Component({
  selector: 'app-user-model',
  imports: [FieldDefsComponent, MatButtonModule, MatIconModule],
  template: `
    <div class="banner">
      <h1 i18n="@@Model">Model</h1>
      <span class="grow"></span>
      <button matButton="filled" (click)="save()" [disabled]="saving() || !dirty()" i18n="@@Save">Save</button>
    </div>

    <div class="content">
      <h2 i18n="@@Account_fields">Account fields</h2>
      <p class="hint" i18n="@@Model_hint">
        Your own facts about a person, such as an employee number or a cost centre. Fill them per
        account under Application, Users; a route can then forward them. None is mandatory.
      </p>
      <app-field-defs [(fields)]="userFields" />

      <!-- Only where there are organisations to tell apart: a single one
           would carry facts about the installation, which is what the
           gateway's own settings are for. -->
      @if (multiTenant()) {
        <h2 class="second" i18n="@@Tenant_fields">Tenant fields</h2>
        <p class="hint" i18n="@@Tenant_model_hint">
          Your own facts about a tenant, such as its plan or its region. Root and app-admins fill them
          on the tenant's General screen; a route forwards them beside the account fields. A name
          belongs to one list only.
        </p>
        <app-field-defs [(fields)]="tenantFields" />
      }

      @if (error()) {
        <p class="warn">
          <mat-icon>report</mat-icon>
          <span>{{ error() }}</span>
        </p>
      }
    </div>
  `,
  styles: `
    :host {
      display: block;
      height: 100%;
      overflow: auto;
    }
    .banner {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 16px 24px 0;
    }
    h1 {
      margin: 0;
      font-size: 1.3rem;
      font-weight: 500;
    }
    .grow {
      flex: 1;
    }
    .content {
      padding: 8px 24px 32px;
      max-width: 1100px;
    }
    h2 {
      margin: 16px 0 4px;
      font-size: 0.78rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--mat-sys-on-surface-variant);
    }
    .second {
      margin-top: 32px;
    }
    .hint {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
      line-height: 1.45;
      max-width: 78ch;
    }
    .warn {
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--mat-sys-error);
      font-size: 0.85rem;
    }
  `,
})
export class UserModelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  protected readonly multiTenant = inject(MeService).multiTenant;

  protected readonly userFields = signal<UserFieldDef[]>([]);
  protected readonly tenantFields = signal<UserFieldDef[]>([]);
  private readonly savedUser = signal<string>('[]');
  private readonly savedTenant = signal<string>('[]');
  protected readonly saving = signal(false);
  protected readonly error = signal('');
  private readonly userDirty = computed(() => JSON.stringify(this.userFields()) !== this.savedUser());
  private readonly tenantDirty = computed(() => JSON.stringify(this.tenantFields()) !== this.savedTenant());
  protected readonly dirty = computed(() => this.userDirty() || this.tenantDirty());

  constructor() {
    this.load();
    // Somebody else's write (CONSOLE-13). OFFERED and not applied: this screen
    // is a form, and reloading it under somebody typing would throw their work
    // away for news they did not ask for. The same bargain the 409 does from
    // the other end - see stale.interceptor.ts.
    const live = inject(LiveChangesService);
    live.on('settings', (change) => live.offer(change, () => this.load()));
  }

  private load(): void {
    this.api.userFields().subscribe({
      next: (r) => this.seed(this.userFields, this.savedUser, r.fields),
      error: () => undefined,
    });
    if (this.multiTenant()) {
      this.api.tenantFields().subscribe({
        next: (r) => this.seed(this.tenantFields, this.savedTenant, r.fields),
        error: () => undefined,
      });
    }
  }

  private seed(list: WritableSignal<UserFieldDef[]>, saved: WritableSignal<string>, fields?: UserFieldDef[]): void {
    list.set(fields ?? []);
    saved.set(JSON.stringify(fields ?? []));
  }

  // One Save for both lists, each sent only when it changed. The account's
  // first: moving a name from it to the organisation's frees the name before
  // the other list claims it.
  protected save(): void {
    this.error.set('');
    this.saving.set(true);
    // The routes the change was carried to, said rather than discovered: a
    // field ticked on routes nobody opened is a change to what services get.
    const moved = new Set<string>();
    const calls = [
      ...(this.userDirty()
        ? [
            this.api.saveUserFields(this.userFields()).pipe(
              tap((r) => {
                this.seed(this.userFields, this.savedUser, r.fields);
                (r.routes ?? []).forEach((n) => moved.add(n));
              }),
            ),
          ]
        : []),
      ...(this.tenantDirty()
        ? [
            this.api.saveTenantFields(this.tenantFields()).pipe(
              tap((r) => {
                this.seed(this.tenantFields, this.savedTenant, r.fields);
                (r.routes ?? []).forEach((n) => moved.add(n));
              }),
            ),
          ]
        : []),
    ];
    concat(...calls).subscribe({
      complete: () => {
        this.saving.set(false);
        const names = [...moved].join(', ');
        this.snack.open(
          names ? $localize`:@@Saved_routes_updated:Saved. Routes updated: ${names}:NAMES:` : $localize`:@@Saved:Saved`,
          undefined,
          { duration: names ? 5000 : 2000 },
        );
      },
      error: (e: { error?: { error?: string } }) => {
        this.saving.set(false);
        // The server's own sentence: it names what is allowed, and rewriting
        // it here would be a second, worse copy of that rule.
        this.error.set(e?.error?.error ?? $localize`:@@Save_failed:Save failed`);
      },
    });
  }
}
