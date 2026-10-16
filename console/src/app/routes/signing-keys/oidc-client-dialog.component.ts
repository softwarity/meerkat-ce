import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormField, disabled, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, OIDCClient, OIDCClientSecret } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { DurationChoice, humanIso, withCurrent } from '../../shared/iso-duration';
import { OidcSecretDialogComponent } from './oidc-secret-dialog.component';

// Creating or editing an OIDC client: an application that signs its users in
// through Meerkat. Opened from the OIDC / JWT drawer and from a route's Auth
// forward section ("New client"), always through `editOidcClient` below, so
// both places end the same way - the secret shown once when there is one.
//
// It saves itself and closes with what the server answered, so a refusal is
// said where it can be fixed.
//
// No return address is asked: an application is served behind its routes,
// and the provider sends a code back under the routes bound to the client,
// nowhere else.
export interface OidcClientDialogData {
  // The client to edit; absent for a new one.
  client?: OIDCClient;
  // The ids already taken, so a new one is refused before the server does.
  taken?: string[];
}

// The rules mirror the server's (store/oidc.go): an id is a client_id, typed
// into the application's configuration, so it is kept to one plain shape.
const CLIENT_ID = /^[a-z0-9][a-z0-9._-]{0,62}$/;

// The lifetimes offered, inside the server's bounds (PT1M to PT24H). Short:
// a refresh token renews them without the person noticing.
const TTL_CHOICES = ['PT1M', 'PT5M', 'PT10M', 'PT15M', 'PT30M', 'PT1H', 'PT2H', 'PT8H', 'PT24H'];
const DEFAULT_TTL = 'PT5M';

@Component({
  selector: 'app-oidc-client-dialog',
  imports: [
    FormField,
    FormFieldComponent,
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    MatTooltipModule,
  ],
  styles: [
    `
      .note {
        margin: 0 0 8px;
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
        <mat-slide-toggle [formField]="f.confidential" i18n="@@The_application_keeps_a_secret"
          >The application keeps a secret</mat-slide-toggle
        >
      </p>
      <p class="note">
        @if (draft().confidential) {
          <ng-container i18n="@@Oidc_confidential_hint"
            >Its server holds the secret and sends it with each code exchange. Shown once, when it is made.</ng-container
          >
        } @else {
          <ng-container i18n="@@Oidc_public_hint"
            >Code running in the browser or on a phone cannot hide a secret: it proves itself with PKCE alone.</ng-container
          >
        }
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
  private readonly ref = inject(MatDialogRef<OidcClientDialogComponent, OIDCClientSecret>);
  private readonly data = inject<OidcClientDialogData>(MAT_DIALOG_DATA, { optional: true }) ?? {};

  protected readonly editing = signal(!!this.data.client);

  protected readonly draft = signal({
    id: this.data.client?.id ?? '',
    confidential: this.data.client?.confidential ?? true,
    audience: this.data.client?.audience ?? '',
    tokenTtl: this.data.client?.tokenTtl || DEFAULT_TTL,
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
  });

  protected readonly idMessage = computed(() => this.f.id().errors()[0]?.message ?? '');

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
      confidential: d.confidential,
      audience: d.audience.trim(),
      tokenTtl: d.tokenTtl,
    };
    this.saving.set(true);
    this.error.set('');
    const done = {
      next: (out: OIDCClientSecret) => this.ref.close(out),
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    };
    if (this.editing()) {
      this.api.updateOidcClient(client).subscribe({ ...done, next: (c) => done.next({ client: c }) });
    } else {
      this.api.createOidcClient(client).subscribe(done);
    }
  }
}

// Opens the editor, then the secret when the server made one. A client that
// becomes confidential on an edit has none yet: one is made at once, since a
// confidential client without a secret signs nobody in.
// Resolves the saved client, or undefined when nothing was saved.
export async function editOidcClient(
  dialog: MatDialog,
  api: ApiService,
  data: OidcClientDialogData,
): Promise<OIDCClient | undefined> {
  const out = await firstValueFrom(
    dialog
      .open<OidcClientDialogComponent, OidcClientDialogData, OIDCClientSecret>(OidcClientDialogComponent, {
        data,
        width: '600px',
        maxWidth: '92vw',
        restoreFocus: true,
      })
      .afterClosed(),
  );
  if (!out) return undefined;
  let shown = out;
  if (!shown.secret && shown.client.confidential && !shown.client.secretPrefix) {
    try {
      shown = await firstValueFrom(api.regenerateOidcClientSecret(shown.client.id));
    } catch {
      // The client is saved; its secret is one Regenerate away in the list.
    }
  }
  if (shown.secret) await showOidcSecret(dialog, shown);
  return shown.client;
}

// The secret, shown once.
export function showOidcSecret(dialog: MatDialog, out: OIDCClientSecret): Promise<unknown> {
  return firstValueFrom(
    dialog
      .open(OidcSecretDialogComponent, {
        data: { clientId: out.client.id, secret: out.secret ?? '' },
        width: '560px',
        maxWidth: '92vw',
        disableClose: true,
        restoreFocus: true,
      })
      .afterClosed(),
  );
}
