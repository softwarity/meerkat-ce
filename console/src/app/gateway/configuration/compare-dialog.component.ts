import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, ConfigPlan, SavedConfiguration } from '../../api.service';
import { PlanTableComponent } from './plan-table.component';

export interface CompareDialogData {
  from: SavedConfiguration;
  others: SavedConfiguration[];
}

// Two saved configurations against each other (CFG-04), what runs left out:
// the question asked of a customer's configuration and the template it was
// made from, before touching either. The same table as an import, read as
// "going from this one to that one".
@Component({
  selector: 'app-compare-dialog',
  imports: [
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatSelectModule,
    PlanTableComponent,
  ],
  styles: [
    `
      mat-dialog-content {
        max-width: min(680px, 84vw);
      }
      mat-form-field {
        width: 100%;
      }
      .same {
        display: flex;
        gap: 12px;
        padding: 12px 16px;
        border-radius: 8px;
        background: var(--mat-sys-surface-container);
        margin: 12px 0;
        font-size: 0.85rem;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Compare_configurations">Compare configurations</h2>
    <mat-dialog-content>
      <mat-form-field subscriptSizing="dynamic">
        <mat-label i18n="@@Going_from_to">From {{ data.from.name }} to</mat-label>
        <mat-select (selectionChange)="compare($event.value)">
          @for (o of data.others; track o.id) {
            <mat-option [value]="o">{{ o.name }}</mat-option>
          }
        </mat-select>
      </mat-form-field>
      @if (error(); as e) {
        <p class="same">{{ e }}</p>
      }
      @if (plan()) {
        <app-plan-table
          [plan]="plan()"
          i18n-nothing="@@Configurations_identical"
          nothing="These two configurations describe the same thing."
        />
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Close">Close</button>
    </mat-dialog-actions>
  `,
})
export class CompareDialogComponent {
  protected readonly data = inject<CompareDialogData>(MAT_DIALOG_DATA);
  private readonly api = inject(ApiService);
  protected readonly plan = signal<ConfigPlan | null>(null);
  protected readonly error = signal('');

  protected compare(to: SavedConfiguration): void {
    this.plan.set(null);
    this.error.set('');
    this.api.compareConfigurations(this.data.from.id, to.id).subscribe({
      next: (p) => this.plan.set(p),
      error: (e) => this.error.set(e?.error?.error ?? $localize`:@@Request_failed:Request failed`),
    });
  }



}
