import { Component, inject, input, model, signal } from '@angular/core';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, UserFieldDef } from '../api.service';
import { FormFieldComponent } from '../shared/form-field.component';

// The inputs, and only the inputs: the window an account is valid in, then
// this installation's own fields on it.
//
// The window first: WHEN an account works is read before WHAT it carries, and
// it is the one line on this form that can make every other one moot.
//
// Presentational on purpose - it saves nothing and knows no account. The
// editor wraps it with Save and Cancel; the creation dialog embeds it in its
// own form. Two containers, one set of widgets, so a field added tomorrow is
// drawn the same way in both places instead of the same way twice.
@Component({
  selector: 'app-user-fields-form',
  imports: [
    FormFieldComponent,
    MatCheckboxModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  template: `
    @if (withWindow()) {
      <h4 i18n="@@Access_validity">Access validity</h4>
      <div class="grid">
        <app-form-field i18n-label="@@Access_from" label="Access from">
          <input matInput type="date" [value]="day(validFrom())" (input)="setDay('from', $any($event.target).value)" />
        </app-form-field>
        <app-form-field i18n-label="@@Access_until" label="Access until">
          <input matInput type="date" [value]="day(validUntil())" (input)="setDay('until', $any($event.target).value)" />
        </app-form-field>
      </div>
      <p class="hint" i18n="@@Window_hint">
        Empty means no bound on that side. The last day counts in full, and an account outside its
        window is refused at sign-in with the date - never signed out mid-work by a clock.
      </p>
    }

    <!-- Named, because these are not fields of the product: they are this
         installation's own, and somebody reading the form has to know which
         list they came from (Infra > Model) to know who can change it. -->
    @if (defs().length) {
      <h4 i18n="@@Extra_fields">Extra fields</h4>
      <div class="grid">
        @for (f of defs(); track f.name) {
          @switch (f.kind) {
            @case ('choice') {
              <mat-form-field>
                <mat-label>{{ label(f) }}</mat-label>
                <mat-select [value]="value(f.name)" (selectionChange)="set(f.name, $event.value)">
                  <!-- Always a way back to "not set": a choice with no way out
                       is a value nobody can remove once it is wrong, and no
                       custom field is ever mandatory. -->
                  <mat-option value="" i18n="@@Not_set">Not set</mat-option>
                  @for (c of f.choices ?? []; track c) {
                    <mat-option [value]="c">{{ c }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>
            }
            @case ('bool') {
              <mat-checkbox
                [checked]="value(f.name) === 'true'"
                (change)="set(f.name, $event.checked ? 'true' : 'false')"
              >
                {{ label(f) }}
              </mat-checkbox>
            }
            @default {
              <app-form-field [label]="label(f)">
                <input
                  matInput
                  [type]="f.kind === 'date' ? 'date' : f.kind === 'number' ? 'number' : 'text'"
                  spellcheck="false"
                  [value]="value(f.name)"
                  (input)="set(f.name, $any($event.target).value)"
                />
              </app-form-field>
            }
          }
        }
      </div>
    }

  `,
  styles: `
    h4 {
      margin: 0 0 10px;
      font-size: 0.78rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--mat-sys-on-surface-variant);
    }
    .grid {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
      align-items: center;
    }
    .grid > * {
      width: 220px;
    }
    .hint {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.82rem;
      line-height: 1.45;
      max-width: 70ch;
      margin: 4px 0 18px;
    }
  `,
})
export class UserFieldsFormComponent {
  readonly fields = model<Record<string, string>>({});
  readonly validFrom = model<number>(0);
  readonly validUntil = model<number>(0);
  // The creation drawer shows the window too; a caller that has its own place
  // for it can turn this off rather than draw it twice.
  readonly withWindow = input(true);

  private readonly api = inject(ApiService);
  protected readonly defs = signal<UserFieldDef[]>([]);

  constructor() {
    this.api.userFields().subscribe({
      next: (r) => this.defs.set(r.fields ?? []),
      error: () => undefined,
    });
  }

  protected label(f: UserFieldDef): string {
    return f.label || f.name;
  }

  protected value(name: string): string {
    return this.fields()[name] ?? '';
  }

  protected set(name: string, v: string): void {
    const next = { ...this.fields() };
    if (v === '') delete next[name];
    else next[name] = v;
    this.fields.set(next);
  }

  // A date input speaks YYYY-MM-DD; an account keeps unix seconds. Converted at
  // UTC midnight both ways, so a window written in Paris and read in Tokyo
  // names the same day.
  protected day(unix: number): string {
    return unix ? new Date(unix * 1000).toISOString().slice(0, 10) : '';
  }

  protected setDay(which: 'from' | 'until', v: string): void {
    const unix = v ? Math.floor(Date.parse(v + 'T00:00:00Z') / 1000) : 0;
    if (which === 'from') this.validFrom.set(unix);
    else this.validUntil.set(unix);
  }
}
