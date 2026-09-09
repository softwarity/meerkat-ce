import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, UserFieldDef, UserFieldKind } from '../api.service';
import { FormFieldComponent } from '../shared/form-field.component';

// Infra > Model: the SHAPE of the objects this installation keeps, starting
// with the account.
//
// Defining a field and filling it are two different acts by two different
// people: one says "this installation records a cost centre", the other says
// "Alice's is B200". So the definition lives in the infrastructure plane and
// the values on the accounts screen - which is also what makes a custom field
// safe to forward, since nobody can grant themselves an attribute a service
// trusts.
@Component({
  selector: 'app-user-model',
  imports: [
    FormFieldComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  template: `
    <div class="banner">
      <h1 i18n="@@Model">Model</h1>
      <span class="grow"></span>
      <button matButton (click)="add()">
        <mat-icon>add</mat-icon>
        <ng-container i18n="@@Add_a_field">Add a field</ng-container>
      </button>
      <button matButton="filled" (click)="save()" [disabled]="saving() || !dirty()" i18n="@@Save">Save</button>
    </div>

    <div class="content">
      <h2 i18n="@@Account_fields">Account fields</h2>
      <p class="hint" i18n="@@Model_hint">
        What this installation knows about a person that this product could not have guessed - an
        employee number, a cost centre, a contract reference. Each one is filled per account under
        Application, Users, and from then on it travels like any other fact about the caller: pick
        it in a route's identity forwarding to send it to a service, or in its user info to stamp it
        on a page. None of them is ever mandatory: a field defined today is empty on every account
        that already exists, and demanding it would stop the next person who opens one of them to
        change something else.
      </p>

      @for (f of fields(); track $index) {
        <div class="field">
          <div class="row">
            <app-form-field class="name" i18n-label="@@Name" label="Name">
              <input
                matInput
                spellcheck="false"
                placeholder="employeeNumber"
                [value]="f.name"
                (input)="patch($index, { name: $any($event.target).value })"
              />
            </app-form-field>
            <app-form-field class="label" i18n-label="@@Label" label="Label">
              <input
                matInput
                [value]="f.label ?? ''"
                (input)="patch($index, { label: $any($event.target).value })"
              />
            </app-form-field>
            <mat-form-field class="kind">
              <mat-label i18n="@@Type">Type</mat-label>
              <mat-select [value]="f.kind" (selectionChange)="patch($index, { kind: $event.value })">
                <mat-option value="text" i18n="@@Kind_text">Text</mat-option>
                <mat-option value="number" i18n="@@Kind_number">Number</mat-option>
                <mat-option value="date" i18n="@@Kind_date">Date</mat-option>
                <mat-option value="choice" i18n="@@Kind_choice">Choice</mat-option>
                <mat-option value="bool" i18n="@@Kind_bool">Yes / no</mat-option>
              </mat-select>
            </mat-form-field>
            <span class="grow"></span>
            <button
              matIconButton
              (click)="remove($index)"
              i18n-matTooltip="@@Remove"
              matTooltip="Remove"
              i18n-aria-label="@@Remove"
              aria-label="Remove"
            >
              <mat-icon>close</mat-icon>
            </button>
          </div>

          <!-- The list is what earns the type: it turns a cost centre into
               data instead of three spellings of the same thing. -->
          @if (f.kind === 'choice') {
            <app-form-field class="choices" i18n-label="@@Choices" label="Choices">
              <input
                matInput
                spellcheck="false"
                placeholder="A100, B200, C300"
                [value]="(f.choices ?? []).join(', ')"
                (input)="setChoices($index, $any($event.target).value)"
              />
            </app-form-field>
          }
        </div>
      } @empty {
        <p class="empty" i18n="@@No_field_yet">
          No field yet. An account carries what this product invented - a name, an address, a
          language - and nothing of what your estate calls a person.
        </p>
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
    .hint,
    .empty {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
      line-height: 1.45;
      max-width: 78ch;
    }
    .empty {
      margin: 24px 0;
    }
    .field {
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 12px;
      padding: 12px 14px 0;
      margin-bottom: 12px;
    }
    .row {
      display: flex;
      align-items: center;
      gap: 12px;
      flex-wrap: wrap;
    }
    .name,
    .label {
      width: 220px;
    }
    .kind {
      width: 160px;
    }
    .choices {
      width: 100%;
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

  protected readonly fields = signal<UserFieldDef[]>([]);
  private readonly saved = signal<string>('[]');
  protected readonly saving = signal(false);
  protected readonly error = signal('');
  protected readonly dirty = computed(() => JSON.stringify(this.fields()) !== this.saved());

  constructor() {
    this.api.userFields().subscribe({
      next: (r) => {
        this.fields.set(r.fields ?? []);
        this.saved.set(JSON.stringify(r.fields ?? []));
      },
      error: () => undefined,
    });
  }

  protected add(): void {
    this.fields.update((f) => [...f, { name: '', kind: 'text' as UserFieldKind }]);
  }

  protected remove(i: number): void {
    this.fields.update((f) => f.filter((_, n) => n !== i));
  }

  protected patch(i: number, change: Partial<UserFieldDef>): void {
    this.fields.update((f) => f.map((x, n) => (n === i ? { ...x, ...change } : x)));
  }

  // Typed as a line rather than as a repeater: a choice list is three or four
  // short values, and a row of inputs to add one at a time is more machinery
  // than the thing it holds.
  protected setChoices(i: number, raw: string): void {
    this.patch(i, { choices: raw.split(',').map((c) => c.trim()).filter(Boolean) });
  }

  protected save(): void {
    this.error.set('');
    this.saving.set(true);
    this.api.saveUserFields(this.fields()).subscribe({
      next: (r) => {
        this.fields.set(r.fields ?? []);
        this.saved.set(JSON.stringify(r.fields ?? []));
        this.saving.set(false);
        this.snack.open($localize`:@@Saved:Saved`, undefined, { duration: 2000 });
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
