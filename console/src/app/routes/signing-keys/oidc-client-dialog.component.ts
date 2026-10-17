import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormField, disabled, form, readonly, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, OIDCClient } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { DurationChoice, humanIso, withCurrent } from '../../shared/iso-duration';

// Creating or editing an OIDC client: an application that signs its users in
// through Meerkat. Opened from the OIDC / JWT drawer and from a route's Auth
// forward section ("New client"), always through `editOidcClient` below.
//
// It saves itself and closes with what the server answered, so a refusal is
// said where it can be fixed.
//
// The secret is one choice of three, and one field under it whatever the
// choice, so the dialog keeps its shape:
//   - none: code in a browser or on a phone, which proves itself with PKCE;
//     the field is off;
//   - drawn by Meerkat: drawn here, in the field, to copy before saving; the
//     server keeps its hash like any other, so there is no second dialog to
//     show it once;
//   - given by the application (its chart generates it, its vendor fixed it):
//     pasted in the field.
//
// No return address is asked: an application is served behind its routes,
// and the provider sends a code back under the routes bound to the client,
// nowhere else.
export interface OidcClientDialogData {
  // The client to edit; absent for a new one.
  client?: OIDCClient;
  // The ids already taken, so a new one is refused before the server does.
  taken?: string[];
  // Open on a new secret drawn by Meerkat: the list's Regenerate.
  redraw?: boolean;
}

// The rules mirror the server's (store/oidc.go): an id is a client_id, typed
// into the application's configuration, so it is kept to one plain shape.
const CLIENT_ID = /^[a-z0-9][a-z0-9._-]{0,62}$/;

// The lifetimes offered, inside the server's bounds (PT1M to PT24H). Short:
// a refresh token renews them without the person noticing.
const TTL_CHOICES = ['PT1M', 'PT5M', 'PT10M', 'PT15M', 'PT30M', 'PT1H', 'PT2H', 'PT8H', 'PT24H'];
const DEFAULT_TTL = 'PT5M';

// The server's bounds on a secret it is given (store/oidc.go): its sha256
// travels in an exported configuration, so not a short one.
const GIVEN_SECRET_MIN = 16;
const GIVEN_SECRET_MAX = 256;

// What starts a secret Meerkat draws (store.OIDCSecretPrefix), so one found
// in a log or a repository says what it is.
const DRAWN_PREFIX = 'mko_';

// Where a client's secret comes from.
type SecretMode = 'none' | 'meerkat' | 'application';

// What an existing client's secret is, read from what the list shows of it: a
// drawn secret starts with mko_, the application's shows four characters.
function secretModeOf(c?: OIDCClient): SecretMode {
  if (!c) return 'meerkat';
  if (!c.confidential) return 'none';
  return !c.secretPrefix || c.secretPrefix.startsWith(DRAWN_PREFIX) ? 'meerkat' : 'application';
}

// A secret as the server draws one: 32 random bytes, base64url, behind mko_.
function drawSecret(): string {
  const raw = crypto.getRandomValues(new Uint8Array(32));
  const b64 = btoa(String.fromCharCode(...raw));
  return DRAWN_PREFIX + b64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

@Component({
  selector: 'app-oidc-client-dialog',
  imports: [
    FormField,
    FormFieldComponent,
    MatButtonModule,
    MatButtonToggleModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  styles: [
    `
      .note {
        margin: 0 0 8px;
        min-height: 2lh;
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
      }
      app-form-field,
      mat-form-field {
        display: block;
        width: 100%;
      }
      .problem {
        display: flex;
        align-items: center;
        gap: 6px;
        color: var(--mat-sys-error);
        font-size: 0.85rem;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>
      @if (editing()) {
        <ng-container i18n="@@Edit_OIDC_client">Edit OIDC client</ng-container>
      } @else {
        <ng-container i18n="@@New_OIDC_client">New OIDC client</ng-container>
      }
    </h2>
    <mat-dialog-content>
      <p>
        <app-form-field
          i18n-label="@@Client_id"
          label="Client id"
          i18n-hint="@@Client_id_hint"
          hint="The application's client_id. It cannot be changed later."
          [error]="idMessage()"
          [clearable]="!editing()"
        >
          <input matInput [formField]="f.id" spellcheck="false" autocapitalize="off" placeholder="my-app" />
        </app-form-field>
      </p>
      <p>
        <mat-button-toggle-group
          [value]="draft().mode"
          (change)="setMode($event.value)"
          i18n-aria-label="@@Client_secret"
          aria-label="Client secret"
        >
          <mat-button-toggle value="none" i18n="@@Oidc_secret_none">No secret</mat-button-toggle>
          <mat-button-toggle value="meerkat" i18n="@@Oidc_secret_meerkat">Drawn by Meerkat</mat-button-toggle>
          <mat-button-toggle value="application" i18n="@@Oidc_secret_application">Given by the application</mat-button-toggle>
        </mat-button-toggle-group>
      </p>
      <p class="note">
        @switch (draft().mode) {
          @case ('none') {
            <ng-container i18n="@@Oidc_public_hint"
              >Code running in the browser or on a phone cannot hide a secret: it proves itself with PKCE alone.</ng-container
            >
          }
          @case ('meerkat') {
            <ng-container i18n="@@Oidc_drawn_hint"
              >Copy it into the application's configuration: once saved, Meerkat keeps only its hash.</ng-container
            >
          }
          @case ('application') {
            <ng-container i18n="@@Oidc_given_hint"
              >Paste the secret the application imposes. Meerkat keeps only its hash.</ng-container
            >
          }
        }
      </p>
      <p>
        <app-form-field
          i18n-label="@@Client_secret"
          label="Client secret"
          floatLabel="always"
          [hint]="secretHint()"
          [error]="secretMessage()"
          [clearable]="draft().mode === 'application'"
          [copyable]="draft().mode === 'meerkat' && !!draft().secret"
          [revealable]="draft().mode === 'application'"
          [actionIcon]="draft().mode === 'meerkat' ? 'refresh' : ''"
          i18n-actionLabel="@@Draw_another_secret"
          actionLabel="Draw another"
          unlocks
          (action)="redraw()"
        >
          <input
            matInput
            [type]="draft().mode === 'application' ? 'password' : 'text'"
            [formField]="f.secret"
            [placeholder]="secretPlaceholder()"
            autocomplete="new-password"
            spellcheck="false"
            autocapitalize="off"
          />
        </app-form-field>
      </p>

      <p>
        <app-form-field
          i18n-label="@@Audience"
          label="Audience"
          i18n-hint="@@Oidc_audience_hint"
          hint="The name the application expects in its access tokens. Empty: the client id."
        >
          <input matInput [formField]="f.audience" spellcheck="false" [placeholder]="draft().id" />
        </app-form-field>
      </p>
      <p>
        <mat-form-field>
          <mat-label i18n="@@Token_lifetime">Token lifetime</mat-label>
          <mat-select [formField]="f.tokenTtl">
            @for (c of ttlChoices(); track c.key) {
              <mat-option [value]="c.key">{{ c.label }}</mat-option>
            }
          </mat-select>
          <mat-hint i18n="@@Oidc_ttl_hint">Of the ID and access tokens. A refresh renews them.</mat-hint>
        </mat-form-field>
      </p>
      @if (error(); as e) {
        <p class="problem"><mat-icon>error</mat-icon><span>{{ e }}</span></p>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="f().invalid() || saving()" (click)="submit()">
        @if (editing()) {
          <ng-container i18n="@@Save">Save</ng-container>
        } @else {
          <ng-container i18n="@@Create">Create</ng-container>
        }
      </button>
    </mat-dialog-actions>
  `,
})
export class OidcClientDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<OidcClientDialogComponent, OIDCClient>);
  private readonly data = inject<OidcClientDialogData>(MAT_DIALOG_DATA, { optional: true }) ?? {};

  protected readonly editing = signal(!!this.data.client);

  // What the client's secret was when the dialog opened, and its visible
  // start: on an edit, a secret left as it is keeps working.
  private readonly initialMode: SecretMode = this.data.redraw ? 'meerkat' : secretModeOf(this.data.client);
  private readonly currentPrefix = this.data.client?.confidential ? (this.data.client.secretPrefix ?? '') : '';

  // The application's secret already kept is kept again when nothing is typed.
  private readonly keepsGiven = this.editing() && this.initialMode === 'application' && !!this.currentPrefix;

  protected readonly draft = signal({
    id: this.data.client?.id ?? '',
    mode: this.initialMode,
    audience: this.data.client?.audience ?? '',
    tokenTtl: this.data.client?.tokenTtl || DEFAULT_TTL,
    // Drawn at once where a secret is due: a new client, one that has none
    // yet (imported without it), or the list's Regenerate.
    secret: this.initialMode === 'meerkat' && (!this.currentPrefix || this.data.redraw) ? drawSecret() : '',
  });

  protected readonly f = form(this.draft, (p) => {
    // The client_id is in the application's configuration: changing it here
    // would sign nobody in, so it is chosen once.
    disabled(p.id, () => this.editing());
    required(p.id, { message: $localize`:@@Client_id_required:A client id is required.` });
    validate(p.id, ({ value }) => {
      const v = value();
      if (this.editing() || !v) return [];
      if (!CLIENT_ID.test(v)) {
        return [{ kind: 'id', message: $localize`:@@Client_id_shape:Lower-case letters, digits, dot, dash or underscore, starting with a letter or a digit; 63 at most.` }];
      }
      if (this.data.taken?.includes(v)) {
        return [{ kind: 'id', message: $localize`:@@Client_id_taken:Another client already has this id.` }];
      }
      return [];
    });
    // Off without a secret, and not typed over when Meerkat draws it.
    disabled(p.secret, ({ valueOf }) => valueOf(p.mode) === 'none');
    readonly(p.secret, ({ valueOf }) => valueOf(p.mode) === 'meerkat');
    // Same bounds as the server, said before it does. Empty is a secret left
    // as it is, which only a client that already has one of that kind can do.
    validate(p.secret, ({ value, valueOf }) => {
      if (valueOf(p.mode) !== 'application') return [];
      const v = value();
      if (!v && this.keepsGiven) return [];
      if (!v) return [{ kind: 'secret', message: $localize`:@@Application_secret_required:Paste the secret the application uses.` }];
      if (v.length < GIVEN_SECRET_MIN || v.length > GIVEN_SECRET_MAX) {
        return [{ kind: 'secret', message: $localize`:@@Application_secret_length:From ${GIVEN_SECRET_MIN}:MIN: to ${GIVEN_SECRET_MAX}:MAX: characters.` }];
      }
      if (/\s/.test(v)) {
        return [{ kind: 'secret', message: $localize`:@@Application_secret_spaces:No spaces.` }];
      }
      return [];
    });
  });

  protected setMode(mode: SecretMode): void {
    // Back to the kind of secret the client has: as it is, nothing to type.
    // Anything else starts afresh - drawn now, or empty to paste into.
    const kept = this.editing() && mode === this.initialMode && !!this.currentPrefix && !this.data.redraw;
    const secret = mode === 'meerkat' && !kept ? drawSecret() : '';
    this.draft.update((d) => ({ ...d, mode, secret }));
  }

  protected redraw(): void {
    this.draft.update((d) => ({ ...d, secret: drawSecret() }));
  }

  protected readonly idMessage = computed(() => this.f.id().errors()[0]?.message ?? '');
  protected readonly secretMessage = computed(() => this.f.secret().errors()[0]?.message ?? '');

  // The secret the client keeps, shown by its start while it is left as it is.
  protected readonly secretPlaceholder = computed(() => {
    const d = this.draft();
    if (d.mode === 'none') return $localize`:@@Oidc_no_secret:No secret`;
    if (this.editing() && d.mode === this.initialMode && this.currentPrefix && !d.secret) return `${this.currentPrefix}...`;
    return '';
  });

  // On an existing client, a new secret replaces the current one.
  protected readonly secretHint = computed(() => {
    const d = this.draft();
    if (d.mode === 'none' || !this.editing() || !this.currentPrefix) return '';
    if (!d.secret) {
      return d.mode === 'meerkat'
        ? $localize`:@@Oidc_secret_kept_draw:Kept as it is. Draw another to replace it.`
        : $localize`:@@Oidc_secret_kept_paste:Kept as it is. Paste another to replace it.`;
    }
    return $localize`:@@Oidc_secret_replaces:The current secret stops working when you save.`;
  });

  protected readonly ttlChoices = computed<DurationChoice[]>(() =>
    withCurrent(
      TTL_CHOICES.map((k) => ({
        key: k,
        label: k === DEFAULT_TTL ? $localize`:@@Duration_default:${humanIso(k)}:DURATION: (default)` : humanIso(k),
      })),
      this.draft().tokenTtl,
    ),
  );

  protected readonly saving = signal(false);
  protected readonly error = signal('');

  protected submit(): void {
    if (this.f().invalid()) return;
    const d = this.draft();
    const client: OIDCClient = {
      id: d.id.trim(),
      confidential: d.mode !== 'none',
      audience: d.audience.trim(),
      tokenTtl: d.tokenTtl,
    };
    // Whatever its origin, the server is handed the secret and keeps its hash.
    const secret = d.mode === 'none' ? '' : d.secret;
    this.saving.set(true);
    this.error.set('');
    const done = {
      next: (c: OIDCClient) => this.ref.close(c),
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    };
    if (!this.editing()) {
      this.api.createOidcClient(secret ? { ...client, secret } : client).subscribe({ ...done, next: (out) => done.next(out.client) });
      return;
    }
    this.api.updateOidcClient(client).subscribe({
      ...done,
      next: (c) => {
        if (!secret) return done.next(c);
        // Saved, then the secret: a refusal of the secret leaves the dialog
        // open on it, the rest already kept.
        this.api.regenerateOidcClientSecret(c.id, secret).subscribe({ ...done, next: (out) => done.next(out.client) });
      },
    });
  }
}

// Opens the editor. Resolves the saved client, or undefined when nothing was
// saved.
export function editOidcClient(dialog: MatDialog, data: OidcClientDialogData): Promise<OIDCClient | undefined> {
  return firstValueFrom(
    dialog
      .open<OidcClientDialogComponent, OidcClientDialogData, OIDCClient>(OidcClientDialogComponent, {
        data,
        width: '600px',
        maxWidth: '92vw',
        restoreFocus: true,
      })
      .afterClosed(),
  );
}
