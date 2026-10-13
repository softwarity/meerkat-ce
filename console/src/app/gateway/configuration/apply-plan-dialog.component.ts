import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { ConfigPlan } from '../../api.service';
import { PlanTableComponent } from './plan-table.component';

export interface ApplyPlanDialogData {
  title: string;
  confirmLabel: string;
  plan: ConfigPlan;
}

// Before a plan is applied - a configuration made current, the running
// document edited - the plan itself, line by line, each one tickable: an
// operator who wants all of it but one route the configuration would delete
// unticks that line, and the gateway keeps it (internal/config/keep.go).
//
// It answers the lines kept out ("kind:id"), or nothing when cancelled. The
// confirm turns red while something would still be removed.
@Component({
  selector: 'app-apply-plan-dialog',
  imports: [MatButtonModule, MatDialogModule, PlanTableComponent],
  styles: [
    `
      mat-dialog-content {
        max-width: min(720px, 86vw);
      }
      .danger {
        --mdc-filled-button-container-color: var(--mat-sys-error);
        --mdc-filled-button-label-text-color: var(--mat-sys-on-error);
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>{{ data.title }}</h2>
    <mat-dialog-content>
      <app-plan-table [plan]="data.plan" selectable [(kept)]="kept" />
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button
        matButton="filled"
        [class.danger]="removes() > 0"
        [disabled]="moving().length > 0 && applied() === 0"
        (click)="ref.close(kept())"
      >
        {{ data.confirmLabel }}
      </button>
    </mat-dialog-actions>
  `,
})
export class ApplyPlanDialogComponent {
  protected readonly data = inject<ApplyPlanDialogData>(MAT_DIALOG_DATA);
  protected readonly ref = inject(MatDialogRef<ApplyPlanDialogComponent, string[]>);
  protected readonly kept = signal<string[]>([]);

  // The lines that would move: the plan's, the unchanged ones aside.
  protected readonly moving = computed(() => this.data.plan.changes.filter((c) => c.action !== 'same'));
  // Of those, the ones still ticked - none, and there is nothing to apply. A
  // route's spec and files go with it.
  protected readonly applied = computed(
    () =>
      this.moving().filter((c) => {
        // The server sends route.spec and route.file too, which the type omits.
        const k: string = c.kind;
        const kind = k === 'route.spec' || k === 'route.file' ? 'route' : k;
        return !this.kept().includes(`${kind}:${c.id ?? ''}`);
      }).length,
  );

  // The removals still to be made once the kept lines are left out.
  protected readonly removes = computed(
    () =>
      this.data.plan.changes.filter(
        (c) => c.action === 'remove' && !this.kept().includes(`${c.kind}:${c.id ?? ''}`),
      ).length,
  );
}
