import { Component, computed, inject, input, OnInit, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, SecretLocation, VaultEntry } from '../api.service';
import { FormFieldComponent } from './form-field.component';
import { VaultService } from './vault.service';

// What a vault entry is edited WITH, wherever it is edited.
//
// The same editor serves two homes and that is the point: a modal, when it is
// opened over another editor (declaring a secret without leaving a route's
// drawer - a drawer cannot host the drawer that must lie over it), and a right
// drawer on the vault page, where it matches the routes and users screens. One
// presentational component, so a field added tomorrow appears in both without
// being written twice.
export interface VaultEntryFormData {
  // An existing entry to edit; absent for a new one.
  entry?: VaultEntry;
  // Restricts the kinds offered (a field that only takes a secret).
  kinds?: ('value' | 'secret')[];
  // Prefills the name (from the field being configured).
  suggestedName?: string;
  // Restricts the scope, when the caller knows it.
  scopes?: string[];
  // Display names for tenant scopes ("tenant:<id>" -> the org name).
  tenantNames?: Record<string, string>;
  // Moving a secret that ALREADY exists into the vault rather than declaring a
  // new one - the value is never asked for here.
  stash?: {
    value?: string;
    from?: SecretLocation;
  };
}

@Component({
  selector: 'app-vault-entry-form',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatIconModule,
    MatInputModule,
    MatTooltipModule,
    FormFieldComponent,
  ],
  template: `
    <header>
      <h2>
        @if (stashing()) {
          <ng-container i18n="@@Move_into_the_vault">Move into the vault</ng-container>
        } @else if (editing()) {
          <ng-container i18n="@@Edit_vault_entry">Edit entry</ng-container>
        } @else {
          <ng-container i18n="@@New_vault_entry">New entry</ng-container>
        }
      </h2>
      <span class="grow"></span>
      <button
        matIconButton
        (click)="closed.emit()"
        i18n-matTooltip="@@Cancel"
        matTooltip="Cancel"
        i18n-aria-label="@@Cancel"
        aria-label="Cancel"
      >
        <mat-icon>close</mat-icon>
      </button>
    </header>

    <div class="body">
      <div class="rowtop">
        @if (kinds().length > 1) {
          <mat-button-toggle-group
            class="kinds"
            [value]="kind()"
            (change)="kind.set($event.value)"
            hideSingleSelectionIndicator
          >
            <mat-button-toggle value="value" i18n="@@Kind_value">Value</mat-button-toggle>
            <mat-button-toggle value="secret" i18n="@@Kind_secret">Secret</mat-button-toggle>
          </mat-button-toggle-group>
        }
        @if (scopes().length > 1) {
          <mat-button-toggle-group
            [value]="scope()"
            (change)="scope.set($event.value)"
            [disabled]="editing()"
            hideSingleSelectionIndicator
          >
            @for (sc of scopes(); track sc) {
              <mat-button-toggle [value]="sc">{{ scopeLabel(sc) }}</mat-button-toggle>
            }
          </mat-button-toggle-group>
        }
      </div>
      <p class="hint">
        @if (stashing()) {
          <ng-container i18n="@@Move_into_the_vault_hint">
            The secret stays where it is until you confirm, then only its name remains in the
            configuration. Give it a name you will recognise elsewhere.
          </ng-container>
        } @else if (kind() === 'secret') {
          <ng-container i18n="@@Kind_secret_hint">
            Encrypted at rest and never shown again. Referenced by $name wherever it is needed.
          </ng-container>
        } @else {
          <ng-container i18n="@@Kind_value_hint">
            Stored in clear and readable. Referenced by $name wherever it is needed.
          </ng-container>
        }
      </p>

      <app-form-field
        i18n-label="@@Name"
        label="Name"
        i18n-hint="@@Vault_name_hint"
        hint="A letter, then letters, digits, dot, dash or underscore"
      >
        <textarea
          matInput
          rows="1"
          class="oneline"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          [value]="name()"
          [disabled]="editing()"
          (input)="name.set($any($event.target).value)"
          (keydown.enter)="$event.preventDefault()"
          placeholder="api-host"
        ></textarea>
      </app-form-field>

      @if (!stashing()) {
        <app-form-field
          i18n-label="@@Value"
          label="Value"
          [revealable]="kind() === 'secret'"
          [masked]="kind() === 'secret'"
          [clearable]="false"
          [hint]="keepHint()"
        >
          <textarea
            matInput
            rows="1"
            class="oneline"
            spellcheck="false"
            autocapitalize="off"
            autocorrect="off"
            [value]="value()"
            (input)="value.set($any($event.target).value)"
            (keydown.enter)="$event.preventDefault()"
          ></textarea>
        </app-form-field>
      }

      <app-form-field i18n-label="@@Description" label="Description">
        <textarea
          matInput
          rows="1"
          class="oneline"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          [value]="description()"
          (input)="description.set($any($event.target).value)"
          (keydown.enter)="$event.preventDefault()"
        ></textarea>
      </app-form-field>

      @if (!stashing()) {
        <!-- A reminder, not an expiry that acts: the wording says so, because a
             field called "expires" on a secret reads as "stops working". -->
        <app-form-field
          i18n-label="@@Reminder_date"
          label="Reminder date"
          i18n-hint="@@Reminder_date_hint"
          hint="To be warned before it lapses at its source - a token, a certificate. It does NOT make the secret obsolete: the reference keeps resolving. Empty for no reminder."
          [clearable]="false"
        >
          <input matInput type="date" [value]="day()" (input)="setDay($any($event.target).value)" />
        </app-form-field>
      }

      @if (error(); as e) {
        <p class="err">{{ e }}</p>
      }
    </div>

    <div class="actions">
      <button matButton (click)="closed.emit()" i18n="@@Cancel">Cancel</button>
      @if (stashing()) {
        <button matButton="filled" [disabled]="!canSave() || saving()" (click)="save()" i18n="@@Move">
          Move
        </button>
      } @else {
        <button matButton="filled" [disabled]="!canSave() || saving()" (click)="save()" i18n="@@Save">
          Save
        </button>
      }
    </div>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      min-height: 0;
    }
    header {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 16px 12px 8px 24px;
    }
    h2 {
      margin: 0;
      font-size: 1.15rem;
      font-weight: 500;
    }
    .grow {
      flex: 1;
    }
    .body {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 4px 24px 8px;
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .rowtop {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-bottom: 4px;
    }
    .hint {
      margin: 0 0 8px;
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.82rem;
      line-height: 1.45;
    }
    .err {
      color: var(--mat-sys-error);
      font-size: 0.85rem;
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      padding: 8px 24px 20px;
    }
  `,
})
export class VaultEntryFormComponent implements OnInit {
  readonly data = input<VaultEntryFormData>({});
  readonly saved = output<VaultEntry>();
  readonly closed = output<void>();

  private readonly api = inject(ApiService);
  private readonly vault = inject(VaultService);

  // Seeded in ngOnInit, NOT in the field initialisers: an input() is not bound
  // yet while the component is constructed, so reading data() there would give
  // the default {} and lose an edit's entry or a stash. Both hosts mount a
  // fresh instance per open, so seeding once on init is the value for this edit.
  protected readonly editing = signal(false);
  protected readonly stashing = signal(false);
  protected readonly kinds = signal<('value' | 'secret')[]>(['value', 'secret']);
  protected readonly kind = signal<'value' | 'secret'>('value');
  protected readonly scopes = signal<string[]>(['infra', 'app']);
  protected readonly scope = signal<string>('infra');
  protected readonly name = signal('');
  protected readonly value = signal('');
  protected readonly description = signal('');
  protected readonly expiresAt = signal(0);
  protected readonly saving = signal(false);
  protected readonly error = signal('');

  ngOnInit(): void {
    const d = this.data();
    this.editing.set(!!d.entry);
    this.stashing.set(!!d.stash);
    this.kinds.set(this.stashing() ? ['secret'] : d.kinds?.length ? d.kinds : ['value', 'secret']);
    this.kind.set(d.entry?.kind ?? this.kinds()[0]);
    this.scopes.set(d.scopes?.length ? d.scopes : ['infra', 'app']);
    this.scope.set(d.entry?.scope ?? this.scopes()[0]);
    this.name.set(d.entry?.name ?? d.suggestedName ?? '');
    this.value.set(d.entry?.value ?? '');
    this.description.set(d.entry?.description ?? '');
    this.expiresAt.set(d.entry?.expiresAt ?? 0);
  }

  protected readonly keepHint = computed(() =>
    this.editing() && this.kind() === 'secret'
      ? $localize`:@@Secret_keep_hint:Leave empty to keep the stored secret`
      : '',
  );

  protected readonly canSave = computed(
    () =>
      /^[A-Za-z][A-Za-z0-9_.-]*$/.test(this.name().trim()) &&
      (this.editing() || this.stashing() || !!this.value()),
  );

  protected scopeLabel(scope: string): string {
    if (scope === 'infra') return $localize`:@@Scope_infra:Infra`;
    if (scope === 'app') return $localize`:@@Scope_app:Application`;
    return this.data().tenantNames?.[scope] ?? scope.replace('tenant:', '');
  }

  // A date input speaks YYYY-MM-DD; the entry keeps unix seconds. Converted at
  // UTC midnight both ways, so a date is the same day wherever it is read.
  protected day(): string {
    const u = this.expiresAt();
    return u ? new Date(u * 1000).toISOString().slice(0, 10) : '';
  }

  protected setDay(v: string): void {
    this.expiresAt.set(v ? Math.floor(Date.parse(v + 'T00:00:00Z') / 1000) : 0);
  }

  protected save(): void {
    this.saving.set(true);
    this.error.set('');
    const stash = this.data().stash;
    if (stash?.from) {
      this.api.stashSecret(stash.from, this.name().trim(), this.description().trim()).subscribe({
        next: ({ name, scope }) => {
          void this.vault.reload();
          this.saving.set(false);
          this.saved.emit({ name, scope, kind: 'secret' } as VaultEntry);
        },
        error: (err: unknown) => this.failed(err),
      });
      return;
    }
    this.api
      .saveVaultEntry({
        name: this.name().trim(),
        kind: this.kind(),
        scope: this.scope(),
        value: stash?.value ?? this.value(),
        description: this.description().trim(),
        expiresAt: this.expiresAt(),
      })
      .subscribe({
        next: (saved) => {
          void this.vault.reload();
          this.saving.set(false);
          this.saved.emit(saved);
        },
        error: (err: unknown) => this.failed(err),
      });
  }

  private failed(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.error.set(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`,
    );
    this.saving.set(false);
  }
}
