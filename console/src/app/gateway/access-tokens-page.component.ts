import { Component, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
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
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { firstValueFrom } from 'rxjs';
import { AdminToken, ApiService, TokenDomain, TokenScope } from '../api.service';
import { DialogsService } from '../shared/dialogs.service';

// Control-plane access tokens (root only, Gateway perimeter): headless access
// to the admin port. These are the FOUNDATION for a future CLI and MCP server
// driving Meerkat (PLANNED - the tooling that consumes them comes later). A
// token is minted here, shown once, and authenticates on the admin port via
// `Authorization: Bearer mk_...` with the same powers as its owner (root).
@Component({
  selector: 'app-access-tokens-page',
  imports: [MatButtonModule, MatIconModule, MatSlideToggleModule, MatTooltipModule, LoadingIndicatorComponent],
  styleUrl: './access-tokens-page.component.scss',
  templateUrl: './access-tokens-page.component.html',
})
export class AccessTokensPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);
  private readonly locale = inject(LOCALE_ID);

  protected readonly loading = signal(true);
  protected readonly tokens = signal<AdminToken[]>([]);
  constructor() {
    this.load();
  }

  private load(): void {
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
  protected readonly disableTip = $localize`:@@Disable_token:Disable this token - it stops working, and can be turned back on`;

  protected async create(): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<
          TokenCreateDialogComponent,
          void,
          { name: string; days: number; scope: TokenScope; domain: TokenDomain; from: string } | undefined
        >(TokenCreateDialogComponent, { width: '520px', restoreFocus: true })
        .afterClosed(),
    );
    if (!res) return;
    this.api.createAdminToken(res.name, res.days, res.scope, res.domain, res.from).subscribe({
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
  // scrape config becomes a metrics one without a second token and an edit in
  // another team's repository.
  protected async edit(t: AdminToken): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<
          TokenCreateDialogComponent,
          AdminToken,
          { name: string; days: number; scope: TokenScope; domain: TokenDomain; from: string } | undefined
        >(TokenCreateDialogComponent, { width: '520px', restoreFocus: true, data: t })
        .afterClosed(),
    );
    if (!res) return;
    this.api.updateAdminToken(t.id, res.name, res.days, res.scope, res.domain, res.from).subscribe({
      // No reveal dialog and no new secret: nothing was minted.
      next: () => this.load(),
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected toggle(t: AdminToken, enabled: boolean): void {
    this.api.toggleAdminToken(t.id, enabled).subscribe({
      next: () => this.tokens.update((list) => list.map((x) => (x.id === t.id ? { ...x, enabled } : x))),
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
      message: $localize`:@@Renew_token_warning:The current secret stops working immediately. Whatever is using it is refused until the new one is in place.`,
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

  protected async revoke(t: AdminToken): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Revoke_token_NAME:Revoke token "${t.name}:NAME:"?`,
      confirmLabel: $localize`:@@Revoke:Revoke`,
      danger: true,
    });
    if (!ok) return;
    this.api.revokeAdminToken(t.id).subscribe({
      next: () => this.tokens.update((list) => list.filter((x) => x.id !== t.id)),
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected day(ts: number): string {
    return DateTime.fromSeconds(ts).reconfigure({ locale: this.locale }).toLocaleString(DateTime.DATE_MED);
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

// Create dialog: a name and an expiry choice. Returns {name, days} or undefined.
@Component({
  selector: 'app-token-create-dialog',
  imports: [MatButtonModule, MatDialogModule, MatFormFieldModule, MatInputModule, MatSelectModule],
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
    `,
  ],
  template: `
    <h2 mat-dialog-title>
      @if (editing) {
        <ng-container i18n="@@Edit_token">Edit token</ng-container>
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
          cdkFocusInitial
        />
      </mat-form-field></p>
      <p><mat-form-field>
        <mat-label i18n="@@Perimeter">Perimeter</mat-label>
        <mat-select [value]="scope()" (selectionChange)="scope.set($event.value)">
          <mat-option value="metrics" i18n="@@Metrics_only">Metrics only</mat-option>
          <mat-option value="readonly" i18n="@@Read_only">Read only</mat-option>
          <mat-option value="full" i18n="@@Full_access">Full access</mat-option>
        </mat-select>
        <mat-hint>
          @switch (scope()) {
            @case ('metrics') {
              <ng-container i18n="@@Perimeter_metrics_hint">
                Opens /metrics and nothing else.
              </ng-container>
            }
            @case ('readonly') {
              <ng-container i18n="@@Perimeter_readonly_hint">
                Reads the gateway and runs the testers. Changes nothing.
              </ng-container>
            }
            @default {
              <ng-container i18n="@@Perimeter_full_hint">
                Everything you can do, without a browser.
              </ng-container>
            }
          }
        </mat-hint>
      </mat-form-field></p>
      <p><mat-form-field>
        <mat-label i18n="@@Acts_on">Acts on</mat-label>
        <mat-select [value]="domain()" (selectionChange)="domain.set($event.value)">
          <mat-option value="gateway" i18n="@@The_routing_plane">The routing plane</mat-option>
          <mat-option value="app" i18n="@@The_applications_identity">The application's identity</mat-option>
          <mat-option value="" i18n="@@Everything_you_can_do">Everything you can do</mat-option>
        </mat-select>
        <mat-hint i18n="@@Acts_on_hint">A perimeter only takes away: at most what you are.</mat-hint>
      </mat-form-field></p>
      <p><mat-form-field>
        <mat-label i18n="@@Used_from">Used from</mat-label>
        <input
          matInput
          [value]="from()"
          (input)="from.set($any($event.target).value)"
          placeholder="10.0.0.0/24, 192.168.1.7"
        />
        <mat-hint i18n="@@Used_from_hint">
          The connecting address, never a forwarded header.
        </mat-hint>
      </mat-form-field></p>
      <p><mat-form-field>
        <mat-label i18n="@@Expiry">Expiry</mat-label>
        <mat-select [value]="days()" (selectionChange)="days.set($event.value)">
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
  // The token being changed, or null when one is being minted. ONE dialog for
  // the two: the fields are the same fields, and a second component would be
  // the same form with a different title and a copy of every hint.
  protected readonly editing = inject<AdminToken | null>(MAT_DIALOG_DATA, { optional: true }) ?? null;

  protected readonly name = signal(this.editing?.name ?? '');
  protected readonly days = signal(0);
  // Read-only by default when minting: a token handed to an agent is the
  // common case, and the safe answer should be the one nobody has to think
  // about. When editing, what the token already is.
  protected readonly scope = signal<TokenScope>(this.editing?.scope ?? 'readonly');
  protected readonly domain = signal<TokenDomain>(this.editing?.domain ?? '');
  protected readonly from = signal(this.editing?.fromCidrs ?? '');

  protected confirm(): void {
    const name = this.name().trim();
    if (name) {
      this.ref.close({
        name,
        days: this.days(),
        scope: this.scope(),
        domain: this.domain(),
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
        Keep it in an environment variable rather than in a file: a configuration file is a thing
        people commit.
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
