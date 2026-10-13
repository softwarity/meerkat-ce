import { Component, computed, inject, signal } from '@angular/core';
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

// A saved configuration against what runs, or against another saved one
// (CFG-04). Against what runs, the answer is what serving it would change -
// the activation's own plan, read without activating, so the gateway's state
// needs no name to be compared. Against another, the question asked of a
// customer's configuration and the template it was made from, before touching
// either. The same table as an import, read as "going from this to that".
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
        max-width: min(720px, 86vw);
      }
      mat-form-field {
        width: 100%;
      }
      .direction {
        margin: 12px 0 0;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
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
        <mat-label i18n="@@Compare_NAME_with">Compare {{ data.from.name }} with</mat-label>
        <mat-select [value]="with()" (selectionChange)="compare($event.value)">
          <mat-option [value]="running" i18n="@@What_runs_now">What this gateway runs now</mat-option>
          @for (o of data.others; track o.id) {
            <mat-option [value]="o">{{ o.name }}</mat-option>
          }
        </mat-select>
      </mat-form-field>
      <p class="direction">{{ direction() }}</p>
      @if (error(); as e) {
        <p class="same">{{ e }}</p>
      }
      @if (plan()) {
        <app-plan-table
          [plan]="plan()"
          [nothing]="with() === running ? nothingRunning : nothingSaved"
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
  // What runs, as an option of the list: it has no row to point at.
  protected readonly running = 'running' as const;
  protected readonly with = signal<SavedConfiguration | 'running'>(this.running);

  protected readonly nothingRunning = $localize`:@@Running_matches_saved:This gateway already runs exactly this configuration.`;
  protected readonly nothingSaved = $localize`:@@Configurations_identical:These two configurations describe the same thing.`;

  // Which way the lines read: "Added" is added going from the left to the right.
  protected readonly direction = computed(() => {
    const w = this.with();
    return w === this.running
      ? $localize`:@@Serving_NAME_would:What serving ${this.data.from.name}:name: would change here:`
      : $localize`:@@Going_from_A_to_B:Going from ${this.data.from.name}:from: to ${w.name}:to::`;
  });

  constructor() {
    this.compare(this.running);
  }

  protected compare(to: SavedConfiguration | 'running'): void {
    this.with.set(to);
    this.plan.set(null);
    this.error.set('');
    const call =
      to === this.running
        ? this.api.configurationPlan(this.data.from.id)
        : this.api.compareConfigurations(this.data.from.id, to.id);
    call.subscribe({
      next: (p) => this.plan.set(p),
      error: (e) => this.error.set(e?.error?.error ?? $localize`:@@Request_failed:Request failed`),
    });
  }
}
