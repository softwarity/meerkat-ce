import { Component, model } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { UserFieldDef, UserFieldKind } from '../api.service';
import { FormFieldComponent } from '../shared/form-field.component';

// One list of field definitions, edited in place: an account's or an
// organisation's (MODEL-03). The same kinds and the same rules on both, so one
// set of widgets - the page around it says whose list it is and saves it.
@Component({
  selector: 'app-field-defs',
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
              <mat-option value="multichoice" i18n="@@Kind_multichoice">Multiple choice</mat-option>
              <mat-option value="list" i18n="@@Kind_list">List</mat-option>
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
        @if (f.kind === 'choice' || f.kind === 'multichoice') {
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
        No field yet.
      </p>
    }
    <button matButton (click)="add()">
      <mat-icon>add</mat-icon>
      <ng-container i18n="@@Add_a_field">Add a field</ng-container>
    </button>
  `,
  styles: `
    :host {
      display: block;
    }
    .grow {
      flex: 1;
    }
    .empty {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
      margin: 16px 0;
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
      width: 190px;
    }
    .choices {
      width: 100%;
    }
  `,
})
export class FieldDefsComponent {
  readonly fields = model<UserFieldDef[]>([]);

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
}
