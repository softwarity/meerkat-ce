import { Component, computed, inject, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { AcmeAuthority, AcmeProvider, ApiService } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { SecretFieldComponent } from '../../shared/secret-field.component';

// The ACME authorities (SSL-05), in the TLS screen's drawer: the form at the
// top, the authorities already there underneath - the same shape as the git
// locations, for the same reason: a short list somebody edits, never fifty.
//
// A PROVIDER IS PICKED, NOT DESCRIBED. Let's Encrypt, ZeroSSL, Google: the
// directory is known, and so is whether an account binding is required and
// where it is handed out. The form shows only what the chosen provider uses,
// so an operator never wonders what to put in a field that does nothing.
// Every authority saved becomes a way in of the Add certificate menu.
@Component({
  selector: 'app-authorities-panel',
  imports: [
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    FormFieldComponent,
    SecretFieldComponent,
  ],
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 8px 8px 24px;
      }
      header h2 {
        flex: 1;
        margin: 0;
        font-size: 1.1rem;
        font-weight: 500;
      }
      .body {
        flex: 1;
        min-height: 0;
        display: flex;
        flex-direction: column;
        padding: 0 24px 24px;
      }
      .list {
        flex: 1;
        min-height: 0;
        overflow: auto;
      }
      .form {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        align-items: start;
        column-gap: 12px;
      }
      .form > .full {
        grid-column: 1 / -1;
      }
      .note {
        margin: -4px 0 12px;
        font-size: 0.82rem;
        line-height: 1.45;
        color: var(--mat-sys-on-surface-variant);
      }
      .terms {
        display: flex;
        align-items: center;
        gap: 16px;
        margin: 0 0 8px;
      }
      .terms a {
        font-size: 0.82rem;
        color: var(--mat-sys-primary);
      }
      .actions {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 4px 0 8px;
      }
      .actions .grow {
        flex: 1;
      }
      .busy {
        font-size: 0.82rem;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-table {
        background: transparent;
      }
      mat-cell,
      mat-header-cell {
        padding: 0 8px;
      }
      .mat-column-name {
        flex: 0 0 200px;
      }
      .where {
        display: flex;
        flex-direction: column;
        min-width: 0;
        font-size: 0.75rem;
      }
      .where .url {
        font-family: var(--mk-mono, monospace);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .where .uses {
        color: var(--mat-sys-on-surface-variant);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .empty {
        margin: 8px 0 0;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
      textarea.pem {
        font-family: var(--mk-mono);
        font-size: 0.75rem;
      }
    `,
  ],
  template: `
    <header>
      <h2 i18n="@@Acme_authorities">ACME authorities</h2>
      <button matIconButton (click)="closed.emit()" i18n-aria-label="@@Close" aria-label="Close">
        <mat-icon>close</mat-icon>
      </button>
    </header>

    <div class="body">
      <div class="form">
        <mat-form-field class="full">
          <mat-label i18n="@@Provider">Provider</mat-label>
          <mat-select [value]="form().provider" (valueChange)="pick($event)">
            @for (p of providers(); track p.id) {
              <mat-option [value]="p.id">{{ p.label }}</mat-option>
            }
          </mat-select>
        </mat-form-field>

        <app-form-field
          class="full"
          i18n-label="@@Name"
          label="Name"
          i18n-info="@@Acme_authority_name_info"
          info="What the Add certificate menu and the pool call it."
        >
          <input matInput [value]="form().name || ''" (input)="set('name', $any($event.target).value)" />
        </app-form-field>

        @if (form().provider === 'custom') {
          <app-form-field
            class="full"
            i18n-label="@@Directory_URL"
            label="Directory URL"
            i18n-info="@@Directory_URL_info"
            info="The address of the authority's ACME service, given by whoever runs it. For step-ca: https://host/acme/provisioner-name/directory."
          >
            <input
              matInput
              [value]="form().directoryUrl || ''"
              (input)="set('directoryUrl', $any($event.target).value)"
              placeholder="https://ca.example.internal/acme/acme/directory"
              spellcheck="false"
            />
          </app-form-field>
          <app-form-field
            class="full"
            i18n-label="@@Authority_root_certificate_optional"
            label="Root certificate (PEM, optional)"
            i18n-info="@@Authority_root_info"
            info="Only when the authority's own address is served with a certificate this gateway does not trust - usually the authority's own root. For step-ca: the output of step ca root."
          >
            <textarea
              matInput
              rows="3"
              class="pem"
              [value]="form().rootCa || ''"
              (input)="set('rootCa', $any($event.target).value)"
              placeholder="-----BEGIN CERTIFICATE-----"
            ></textarea>
          </app-form-field>
        }

        @if (bindingShown()) {
          <app-form-field
            [label]="provider()?.needsEab ? keyIdLabel : keyIdOptionalLabel"
            [info]="eabInfo()"
          >
            <input
              matInput
              [value]="form().eabKeyId || ''"
              (input)="set('eabKeyId', $any($event.target).value)"
              spellcheck="false"
            />
          </app-form-field>
          <app-secret-field
            [label]="provider()?.needsEab ? hmacLabel : hmacOptionalLabel"
            [value]="form().eabHmacKey || ''"
            (valueChange)="set('eabHmacKey', $event)"
            [held]="!!form().eabSecretSet"
            [at]="editingId() ? { holder: 'tls', id: editingId(), field: 'eabHmacKey' } : undefined"
            scope="infra"
            (moved)="reload()"
          />
          <app-form-field
            class="full"
            i18n-label="@@Contact_email_optional"
            label="Contact email (optional)"
            i18n-info="@@Contact_email_info"
            info="Some authorities require one to open the account. Expiry is watched by the daily digest, not by this address."
          >
            <input matInput [value]="form().email || ''" (input)="set('email', $any($event.target).value)" />
          </app-form-field>
        }
      </div>

      @if (provider()?.note || provider()?.where; as note) {
        <p class="note">{{ note }}</p>
      }

      <div class="terms">
        <mat-checkbox [checked]="!!form().acceptTos" (change)="set('acceptTos', $event.checked)">
          <span i18n="@@Accept_terms">I accept the authority's terms of service</span>
        </mat-checkbox>
        @if (provider()?.terms; as terms) {
          <a [href]="terms" target="_blank" rel="noopener" i18n="@@Read_the_terms">Read them</a>
        }
      </div>

      <div class="actions">
        @if (busy()) {
          <span class="busy" i18n="@@Asking_the_directory">Asking the authority's directory...</span>
        }
        <div class="grow"></div>
        @if (dirty()) {
          <button matButton (click)="clear()" i18n="@@Cancel">Cancel</button>
        }
        <button matButton="filled" [disabled]="!complete() || busy()" (click)="save()">
          <mat-icon>{{ editingId() ? 'check' : 'add' }}</mat-icon>
          @if (editingId()) {
            <ng-container i18n="@@Save">Save</ng-container>
          } @else {
            <ng-container i18n="@@Add">Add</ng-container>
          }
        </button>
      </div>

      <div class="list">
        @if (authorities().length === 0) {
          <p class="empty" i18n="@@No_acme_authority">
            None yet. Pick a provider above: each one saved becomes a way in of Add certificate.
          </p>
        } @else {
          <mat-table [dataSource]="authorities()">
            <ng-container matColumnDef="name">
              <mat-header-cell *matHeaderCellDef i18n="@@Name">Name</mat-header-cell>
              <mat-cell *matCellDef="let a">{{ a.name }}</mat-cell>
            </ng-container>
            <ng-container matColumnDef="where">
              <mat-header-cell *matHeaderCellDef i18n="@@Where">Where</mat-header-cell>
              <mat-cell *matCellDef="let a">
                <span class="where">
                  <span class="url">{{ host(a.directoryUrl) }}</span>
                  @if (a.uses?.length) {
                    <span class="uses" i18n="@@Asked_for">asked for {{ a.uses.join(', ') }}</span>
                  }
                </span>
                <span rowActions="tonal">
                  <button matIconButton (click)="edit(a)" i18n-matTooltip="@@Edit" matTooltip="Edit"
                    i18n-aria-label="@@Edit" aria-label="Edit">
                    <mat-icon>edit</mat-icon>
                  </button>
                  <button matIconButton (click)="remove(a)" i18n-matTooltip="@@Delete" matTooltip="Delete"
                    i18n-aria-label="@@Delete" aria-label="Delete">
                    <mat-icon>delete</mat-icon>
                  </button>
                </span>
              </mat-cell>
            </ng-container>
            <mat-header-row *matHeaderRowDef="columns"></mat-header-row>
            <mat-row *matRowDef="let row; columns: columns"></mat-row>
          </mat-table>
        }
      </div>
    </div>
  `,
})
export class AuthoritiesPanelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);

  readonly closed = output<void>();
  // An authority added, changed or removed: the Add certificate menu follows.
  readonly changed = output<AcmeAuthority[]>();

  protected readonly columns = ['name', 'where'];
  protected readonly authorities = signal<AcmeAuthority[]>([]);
  protected readonly providers = signal<AcmeProvider[]>([]);
  protected readonly editingId = signal('');
  protected readonly form = signal<Partial<AcmeAuthority>>(blank());
  protected readonly busy = signal(false);

  protected readonly keyIdLabel = $localize`:@@Account_key_ID:Account key ID`;
  protected readonly keyIdOptionalLabel = $localize`:@@Account_key_ID_optional:Account key ID (optional)`;
  protected readonly hmacLabel = $localize`:@@Account_HMAC_key:Account HMAC key`;
  protected readonly hmacOptionalLabel = $localize`:@@Account_HMAC_key_optional:Account HMAC key (optional)`;

  protected readonly provider = computed(() =>
    this.providers().find((p) => p.id === this.form().provider),
  );
  // The binding and the contact belong to the providers that use them: one
  // that demands a binding, and any authority run by somebody else.
  protected readonly bindingShown = computed(
    () => !!this.provider()?.needsEab || this.form().provider === 'custom',
  );
  protected readonly eabInfo = computed(() => {
    const p = this.provider();
    if (p?.needsEab && p.where) return p.where;
    return $localize`:@@EAB_info:Only if the authority refuses anonymous accounts: it shows a key ID and an HMAC key in its own console, or its administrator hands them over.`;
  });

  protected readonly complete = computed(() => {
    const f = this.form();
    if (!f.acceptTos) return false;
    if (f.provider === 'custom' && !(f.directoryUrl ?? '').trim()) return false;
    if (this.provider()?.needsEab && !(f.eabKeyId ?? '').trim()) return false;
    return true;
  });

  protected readonly dirty = computed(() => !!this.editingId() || !!(this.form().name ?? '').trim());

  constructor() {
    this.reload();
  }

  reload(): void {
    this.api.listAcmeAuthorities().subscribe({
      next: (r) => {
        this.authorities.set(r.authorities);
        this.providers.set(r.providers);
        this.changed.emit(r.authorities);
      },
      error: () => this.authorities.set([]),
    });
  }

  protected host(url: string): string {
    return (url ?? '').replace(/^https?:\/\//, '');
  }

  protected pick(provider: string): void {
    const label = this.providers().find((p) => p.id === provider)?.label ?? '';
    const before = this.providers().find((p) => p.id === this.form().provider)?.label ?? '';
    this.form.update((f) => ({
      ...f,
      provider,
      // The name follows the provider until somebody types their own.
      name: !f.name || f.name === before ? label : f.name,
    }));
  }

  protected set<K extends keyof AcmeAuthority>(key: K, value: AcmeAuthority[K]): void {
    this.form.update((f) => ({ ...f, [key]: value }));
  }

  protected clear(): void {
    this.editingId.set('');
    this.form.set(blank());
  }

  protected edit(a: AcmeAuthority): void {
    this.editingId.set(a.id);
    this.form.set({ ...a });
  }

  protected save(): void {
    this.busy.set(true);
    const { id, uses, eabSecretSet, ...body } = this.form();
    void id;
    void uses;
    void eabSecretSet;
    this.api.saveAcmeAuthority(body, this.editingId() || undefined).subscribe({
      next: () => {
        this.busy.set(false);
        this.clear();
        this.reload();
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.snack.open(message(err), undefined, { duration: 8000 });
      },
    });
  }

  protected async remove(a: AcmeAuthority): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_the_authority:Delete the authority`,
      message: $localize`:@@Delete_authority_message:Delete ${a.name}:name:? Certificates are no longer asked of it.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteAcmeAuthority(a.id).subscribe({
      next: () => this.reload(),
      error: (err: unknown) => this.snack.open(message(err), undefined, { duration: 8000 }),
    });
  }
}

function blank(): Partial<AcmeAuthority> {
  return { provider: 'letsencrypt-staging', name: "Let's Encrypt (staging)", acceptTos: false };
}

function message(e: unknown): string {
  const err = e as { error?: { error?: string }; message?: string };
  return err?.error?.error ?? err?.message ?? $localize`:@@Something_went_wrong:Something went wrong`;
}
