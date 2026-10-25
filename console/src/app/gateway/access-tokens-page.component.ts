import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatTableModule } from '@angular/material/table';
import {
  MAT_DIALOG_DATA,
  MatDialog,
  MatDialogModule,
  MatDialogRef,
} from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ActivatedRoute } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { firstValueFrom } from 'rxjs';
import { AdminToken, ApiService, DataToken, TokenScope } from '../api.service';
import { MeService } from '../me.service';
import { DialogsService } from '../shared/dialogs.service';
import { LiveChangesService } from '../shared/live-changes.service';
import { zoned } from '../shared/zoned-date.pipe';

// One row of the common table: a control-plane token of the caller's, an
// internal one (perimeter schedules, which a service presents on the internal
// plane), or an application token of anyone's.
type TokenRow = (AdminToken | DataToken) & {
  plane: 'admin' | 'internal' | 'data';
  ownerName?: string;
  tenantName?: string;
  groupName?: string;
};

// Access tokens, every plane in one table (AUTH-09). The CONTROL plane's are
// the caller's own - minted here, shown once, acting with their owner's powers
// narrowed by a perimeter. The INTERNAL plane's are the same kind of token
// with the schedules perimeter: what a service presents to the scheduler, so
// they are filed apart, named after the service that holds them. The
// APPLICATIONS' are everyone's, shown to whoever administers the applications:
// seen and revoked here, and minted only by their owner, on the profile - a
// credential is never created in somebody else's name. Details and actions
// live in the drawer, like every other list.
@Component({
  selector: 'app-access-tokens-page',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatIconModule,
    MatInputModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
  ],
  styleUrl: './access-tokens-page.component.scss',
  templateUrl: './access-tokens-page.component.html',
})
export class AccessTokensPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);
  private readonly locale = inject(LOCALE_ID);

  protected readonly me = inject(MeService);
  protected readonly loading = signal(true);
  protected readonly tokens = signal<AdminToken[]>([]);
  // Every account's application tokens (AUTH-09), for those who administer
  // the applications: seen and revoked here, minted by their owner only.
  protected readonly dataTokens = signal<DataToken[]>([]);
  // Opened on one plane when a screen sends here for it (?view=internal from
  // the Scheduler's Access drawer), on all of them otherwise.
  protected readonly view = signal<'' | 'admin' | 'internal' | 'data'>(
    (['admin', 'internal', 'data'] as const).find(
      (v) => v === inject(ActivatedRoute).snapshot.queryParamMap.get('view'),
    ) ?? '',
  );
  protected readonly query = signal('');
  protected readonly columns = ['name', 'owner', 'plane', 'scope', 'used'];
  protected readonly rows = computed<TokenRow[]>(() => {
    const q = this.query().trim().toLowerCase();
    const view = this.view();
    // A schedules token is a control-plane token in the store - which is why
    // every one minted before keeps working - and an internal one here: it is
    // what a service presents on the internal plane, never on the console's.
    const mine: TokenRow[] = this.tokens().map((t) => ({
      ...t,
      plane:
        t.scope === 'schedules' || t.scope === 'notifications' ? ('internal' as const) : ('admin' as const),
    }));
    const theirs: TokenRow[] = this.dataTokens().map((t) => ({ ...t, plane: 'data' as const }));
    return [...mine, ...theirs].filter(
      (t) =>
        (!view || t.plane === view) &&
        (!q || t.name.toLowerCase().includes(q) || (t.ownerName ?? '').toLowerCase().includes(q)),
    );
  });
  protected readonly selected = signal<TokenRow | null>(null);
  constructor() {
    this.load();
    // Somebody else's write (CONSOLE-13): this list follows what other
    // operators do, quietly - a spinner replacing a list nobody asked to
    // reload takes the screen away from whoever is reading it.
    inject(LiveChangesService).on('token', () => this.load());
  }

  private load(): void {
    if (this.me.isAppAdmin()) {
      this.api.listDataTokens().subscribe({ next: (list) => this.dataTokens.set(list) });
    }
    this.api.listAdminTokens().subscribe({
      next: (tokens) => {
        // A token belonging to a registered agent is a CONNECTION, and it is
        // seen and cut off under MCP. What is listed here is what somebody
        // minted by hand, for the REST API.
        this.tokens.set(tokens.filter((t) => !t.clientId));
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected readonly enableTip = $localize`:@@Enable_token:Enable this token`;
  protected readonly disableTip = $localize`:@@Disable_token:Disable this token (it can be turned back on)`;

  // Under the Internal filter, a token for a service to call the scheduler:
  // the perimeter is not a choice there, it is what makes it internal.
  protected async create(): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<
          TokenCreateDialogComponent,
          TokenDialogData,
          { name: string; days: number; scope: TokenScope; from: string } | undefined
        >(TokenCreateDialogComponent, {
          width: '520px',
          restoreFocus: true,
          data: { internal: this.view() === 'internal' },
        })
        .afterClosed(),
    );
    if (!res) return;
    this.api.createAdminToken(res.name, res.days, res.scope, res.from).subscribe({
      next: (created) => {
        this.dialog.open(TokenRevealDialogComponent, { data: { token: created.token }, width: '560px' });
        this.load();
      },
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  // Change what a token may do, without touching the token: the secret is a
  // hash and encodes none of this, so whoever holds the key keeps holding the
  // same key. Which is what makes narrowing one cheap - a read-only token in a
  // service's manifest becomes a schedules one without a second token and an
  // edit in another team's repository.
  protected async edit(t: AdminToken): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<
          TokenCreateDialogComponent,
          TokenDialogData,
          { name: string; days: number | null; scope: TokenScope; from: string } | undefined
        >(TokenCreateDialogComponent, { width: '520px', restoreFocus: true, data: { token: t } })
        .afterClosed(),
    );
    if (!res) return;
    this.api.updateAdminToken(t.id, res.name, res.days, res.scope, res.from).subscribe({
      // No reveal dialog and no new secret: nothing was minted.
      next: () => this.load(),
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected toggle(t: AdminToken, enabled: boolean): void {
    this.api.toggleAdminToken(t.id, enabled).subscribe({
      next: () => {
        this.tokens.update((list) => list.map((x) => (x.id === t.id ? { ...x, enabled } : x)));
        const open = this.selected();
        if (open?.id === t.id) this.selected.set({ ...open, enabled });
      },
      error: (err) => {
        this.snack.open(errMsg(err), undefined, { duration: 4000 });
        this.load();
      },
    });
  }

  // A new secret for the same token. Confirmed first, and the sentence says
  // the one thing that matters: the key somebody is using stops working now,
  // not when they get round to swapping it.
  protected async renew(t: AdminToken): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Renew_token_NAME:New secret for "${t.name}:NAME:"?`,
      message: $localize`:@@Renew_token_warning:The current secret stops working immediately.`,
      confirmLabel: $localize`:@@Renew:New secret`,
      danger: true,
    });
    if (!ok) return;
    this.api.renewAdminToken(t.id).subscribe({
      next: (created) => {
        // Shown once, in the same dialog a freshly minted one uses: it IS a
        // freshly minted secret, on a token that already existed.
        this.dialog.open(TokenRevealDialogComponent, { data: { token: created.token }, width: '560px' });
        this.load();
      },
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected open(t: TokenRow): void {
    this.selected.set(t);
  }

  protected close(): void {
    this.selected.set(null);
  }

  // An application token, ended by an administrator: the owner's script is
  // refused at its next call, and the trail says who did it.
  protected async revokeData(t: TokenRow): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Revoke_token_NAME:Revoke token "${t.name}:NAME:"?`,
      message: $localize`:@@Revoke_data_token_warning:It belongs to ${t.ownerName}:OWNER:. It is refused at its next call.`,
      confirmLabel: $localize`:@@Revoke:Revoke`,
      danger: true,
    });
    if (!ok) return;
    this.api.revokeDataToken(t.id).subscribe({
      next: () => {
        this.dataTokens.update((list) => list.filter((x) => x.id !== t.id));
        this.close();
      },
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected async revoke(t: AdminToken): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Revoke_token_NAME:Revoke token "${t.name}:NAME:"?`,
      confirmLabel: $localize`:@@Revoke:Revoke`,
      danger: true,
    });
    if (!ok) return;
    this.api.revokeAdminToken(t.id).subscribe({
      next: () => {
        this.tokens.update((list) => list.filter((x) => x.id !== t.id));
        this.close();
      },
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected scopeLabel(t: TokenRow): string {
    switch (t.scope) {
      case 'schedules':
        return $localize`:@@Scheduled_calls_only:Scheduled calls only`;
      case 'notifications':
        return $localize`:@@Notifications_only:Notifications only`;
      case 'readonly':
        return $localize`:@@Read_only:Read only`;
      default:
        return $localize`:@@Full_access:Full access`;
    }
  }

  protected readonly neverLabel = $localize`:@@Never:Never`;

  protected relative(ts: number): string {
    return DateTime.fromSeconds(ts).reconfigure({ locale: this.locale }).toRelative() ?? '';
  }

  protected day(ts: number): string {
    return zoned(ts, this.me.timezone(), 'day');
  }

  protected expiryLabel(ts: number): string {
    if (!ts) return $localize`:@@never_expires:never expires`;
    return $localize`:@@expires_DATE:expires ${this.day(ts)}:DATE:`;
  }

  protected lastUsedLabel(ts: number): string {
    if (!ts) return $localize`:@@never_used:never used`;
    const rel = DateTime.fromSeconds(ts).reconfigure({ locale: this.locale }).toRelative() ?? '';
    return $localize`:@@last_used_REL:last used ${rel}:REL:`;
  }
}

// What the dialog is opened on: a token to edit, or a new one - internal or
// for the console.
interface TokenDialogData {
  token?: AdminToken;
  internal?: boolean;
}

// Create dialog: a name and an expiry choice. Returns {name, days} or undefined.
@Component({
  selector: 'app-token-create-dialog',
  imports: [
    MatAutocompleteModule,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  styles: [
    `
      /* One rule. The dialog's WIDTH is set where a dialog's width is set -
         in the open() config - and a width here only fought it, pushing the
         fields out past the container. The SPACING is the paragraph's own
         margin, which is how the Material documentation stacks form fields.
         What is left is the only thing neither of them says. */
      mat-form-field {
        width: 100%;
      }
      /* Which plane this key opens, said once at the top: the other kind is
         minted from an account's own profile, on the other port. */
      .plane {
        margin: -4px 0 16px;
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.82rem;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>
      @if (editing) {
        <ng-container i18n="@@Edit_token">Edit token</ng-container>
      } @else if (internal) {
        <ng-container i18n="@@New_internal_token">New internal token</ng-container>
      } @else {
        <ng-container i18n="@@New_token">New token</ng-container>
      }
    </h2>
    <mat-dialog-content>
      <p><mat-form-field>
        <mat-label i18n="@@Token_name">Token name</mat-label>
        <input
          matInput
          [value]="name()"
          (input)="name.set($any($event.target).value)"
          (keydown.enter)="confirm()"
          [matAutocomplete]="services"
          [matAutocompleteDisabled]="!internal"
          cdkFocusInitial
        />
        <mat-autocomplete #services (optionSelected)="name.set($event.option.value)">
          @for (s of suggestions(); track s) {
            <mat-option [value]="s">{{ s }}</mat-option>
          }
        </mat-autocomplete>
        <mat-hint>
          @if (internal) {
            <ng-container i18n="@@Internal_token_name_hint">
              The service that uses it, or global for a token all services share.
            </ng-container>
          }
        </mat-hint>
      </mat-form-field></p>
      @if (internal) {
        <p class="plane" i18n="@@Token_for_the_internal_plane">
          For the internal plane: a backend service uses it, within its perimeter only.
        </p>
      } @else {
        <p class="plane" i18n="@@Token_for_the_control_plane">
          For the control plane API.
        </p>
      }
      @if (internal) {
      <!-- An internal token opens ONE door of the internal plane: the
           scheduler's, or the notifications' (NOTIF-05). -->
      <p><mat-form-field>
        <mat-label i18n="@@Perimeter">Perimeter</mat-label>
        <mat-select [value]="scope()" (selectionChange)="scope.set($event.value)">
          <mat-option value="schedules" i18n="@@Scheduled_calls_only">Scheduled calls only</mat-option>
          <mat-option value="notifications" i18n="@@Notifications_only">Notifications only</mat-option>
        </mat-select>
        <mat-hint>
          @if (scope() === 'notifications') {
            <ng-container i18n="@@Perimeter_notifications_hint">
              Publishes on /api/notifications on the internal plane, nothing else. For a backend service.
            </ng-container>
          } @else {
            <ng-container i18n="@@Perimeter_schedules_hint">
              Calls /api/schedules on the internal plane, nothing else. For a backend service.
            </ng-container>
          }
        </mat-hint>
      </mat-form-field></p>
      }
      @if (!internal) {
      <p><mat-form-field>
        <mat-label i18n="@@Perimeter">Perimeter</mat-label>
        <mat-select [value]="scope()" (selectionChange)="scope.set($event.value)">
          <mat-option value="schedules" i18n="@@Scheduled_calls_only">Scheduled calls only</mat-option>
          <mat-option value="readonly" i18n="@@Read_only">Read only</mat-option>
          <mat-option value="full" i18n="@@Full_access">Full access</mat-option>
        </mat-select>
        <mat-hint>
          @switch (scope()) {
            @case ('schedules') {
              <ng-container i18n="@@Perimeter_schedules_hint">
                Calls /api/schedules on the internal plane, nothing else. For a backend service.
              </ng-container>
            }
            @case ('readonly') {
              <ng-container i18n="@@Perimeter_readonly_hint">
                Reads the gateway and runs the testers. Changes nothing.
              </ng-container>
            }
            @default {
              <ng-container i18n="@@Perimeter_full_hint">
                Everything you can do in the console.
              </ng-container>
            }
          }
        </mat-hint>
      </mat-form-field></p>
      }
      <p><mat-form-field>
        <mat-label i18n="@@Used_from">Used from</mat-label>
        <input
          matInput
          [value]="from()"
          (input)="from.set($any($event.target).value)"
          placeholder="10.0.0.0/24, 192.168.1.7"
        />
        <mat-hint i18n="@@Used_from_hint">
          Allowed addresses or ranges, as seen on the connection (forwarded headers are ignored).
        </mat-hint>
      </mat-form-field></p>
      <p><mat-form-field>
        <mat-label i18n="@@Expiry">Expiry</mat-label>
        <mat-select [value]="days()" (selectionChange)="days.set($event.value)">
          @if (editing) {
            <mat-option [value]="keep" i18n="@@Unchanged">Unchanged</mat-option>
          }
          <mat-option [value]="0" i18n="@@never_expires">never expires</mat-option>
          <mat-option [value]="30" i18n="@@in_30_days">30 days</mat-option>
          <mat-option [value]="90" i18n="@@in_90_days">90 days</mat-option>
          <mat-option [value]="365" i18n="@@in_1_year">1 year</mat-option>
        </mat-select>
      </mat-form-field></p>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!name().trim()" (click)="confirm()">
        @if (editing) {
          <ng-container i18n="@@Save">Save</ng-container>
        } @else {
          <ng-container i18n="@@Create">Create</ng-container>
        }
      </button>
    </mat-dialog-actions>
  `,
})
export class TokenCreateDialogComponent {
  private readonly ref = inject(MatDialogRef<TokenCreateDialogComponent>);
  private readonly data = inject<TokenDialogData | null>(MAT_DIALOG_DATA, { optional: true });
  // The token being changed, or null when one is being minted. ONE dialog for
  // the two: the fields are the same fields, and a second component would be
  // the same form with a different title and a copy of every hint.
  protected readonly editing = this.data?.token ?? null;
  // A new internal token: its perimeter is the scheduler, not a choice.
  protected readonly internal = !this.editing && !!this.data?.internal;

  protected readonly name = signal(this.editing?.name ?? '');
  // Editing starts on the expiry the token has: a fresh "never expires" here
  // turned every rename of a 30-day token into one that never expires. A
  // number rather than null, which mat-select reads as no selection.
  protected readonly keep = -1;
  protected readonly days = signal(this.editing ? this.keep : 0);
  // Read-only by default when minting: a token handed to an agent is the
  // common case, and the safe answer should be the one nobody has to think
  // about. When editing, what the token already is.
  protected readonly scope = signal<TokenScope>(
    this.editing?.scope ?? (this.internal ? 'schedules' : 'readonly'),
  );

  // The names offered for an internal token: global, and the services the
  // runtime knows, by their own name - the workload's last segment.
  private readonly candidates = httpResource<{ candidates: string[] }>(() =>
    this.internal ? '/api/settings/internal-access/candidates?mode=none' : undefined,
  );
  protected readonly suggestions = computed(() => {
    const typed = this.name().trim().toLowerCase();
    const names = (this.candidates.value()?.candidates ?? []).map((c) => c.slice(c.lastIndexOf('/') + 1));
    return ['global', ...new Set(names)].filter((n) => !typed || n.toLowerCase().includes(typed)).slice(0, 50);
  });
  protected readonly from = signal(this.editing?.fromCidrs ?? '');

  protected confirm(): void {
    const name = this.name().trim();
    if (name) {
      this.ref.close({
        name,
        days: this.days() === this.keep ? null : this.days(),
        scope: this.scope(),
        from: this.from().trim(),
      });
    }
  }
}

// Reveal dialog: the clear token, shown once, with a copy button.
@Component({
  selector: 'app-token-reveal-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule],
  styles: [
    `
      .secret {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 12px 16px;
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 8px;
        background: var(--mat-sys-surface-container-highest);
      }
      code {
        font-family: var(--mk-mono);
        font-size: 0.95rem;
        flex: 1;
        word-break: break-all;
        user-select: all;
      }
      .hint {
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
        line-height: 1.4;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Token_created">Token created</h2>
    <mat-dialog-content>
      <p class="hint" i18n="@@Shown_once_copy_it_now">Shown once: copy it now, it cannot be retrieved later.</p>
      <div class="secret">
        <code>{{ data.token }}</code>
        <button matIconButton (click)="copy()" i18n-aria-label="@@Copy" aria-label="Copy">
          <mat-icon>{{ copied() ? 'check' : 'content_copy' }}</mat-icon>
        </button>
      </div>
      <p class="hint" i18n="@@Keep_it_in_an_env_var">
        Keep it in an environment variable, not in a committed file.
      </p>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton="filled" mat-dialog-close i18n="@@Done">Done</button>
    </mat-dialog-actions>
  `,
})
export class TokenRevealDialogComponent {
  protected readonly data = inject<{ token: string }>(MAT_DIALOG_DATA);
  protected readonly copied = signal(false);

  protected copy(): void {
    void navigator.clipboard.writeText(this.data.token).then(() => this.copied.set(true));
  }
}

function errMsg(err: unknown): string {
  const e = err as { error?: { error?: string } };
  return typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`;
}
