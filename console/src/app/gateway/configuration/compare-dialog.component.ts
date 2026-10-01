import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, ConfigPlan, SavedConfiguration } from '../../api.service';

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
  imports: [MatButtonModule, MatDialogModule, MatFormFieldModule, MatIconModule, MatSelectModule],
  styles: [
    `
      mat-dialog-content {
        max-width: min(680px, 84vw);
      }
      mat-form-field {
        width: 100%;
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 12px 0;
        font-size: 0.85rem;
      }
      td {
        padding: 4px 8px 4px 0;
        vertical-align: top;
      }
      .act {
        width: 90px;
        font-weight: 600;
      }
      .act.add {
        color: var(--mat-sys-primary);
      }
      .act.remove {
        color: var(--mat-sys-error);
      }
      .kind {
        width: 110px;
        color: var(--mat-sys-on-surface-variant);
      }
      .fields {
        font-family: var(--mk-mono, monospace);
        font-size: 0.75rem;
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
      @if (plan(); as p) {
        @if (differences(p) === 0) {
          <div class="same">
            <mat-icon>check_circle</mat-icon>
            <span i18n="@@Configurations_identical">These two configurations describe the same thing.</span>
          </div>
        } @else {
          <table>
            @for (c of p.changes; track c.kind + c.id) {
              @if (c.action !== 'same') {
                <tr>
                  <td class="act" [class]="c.action">{{ label(c.action) }}</td>
                  <td class="kind">{{ kindLabel(c.kind) }}</td>
                  <td>
                    {{ c.label || c.id }}
                    @if (c.fields?.length) {
                      <div class="fields">{{ c.fields!.join(', ') }}</div>
                    }
                  </td>
                </tr>
              }
            }
          </table>
        }
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

  protected differences(p: ConfigPlan): number {
    return p.changes.filter((c) => c.action !== 'same').length;
  }

  protected label(action: string): string {
    switch (action) {
      case 'add':
        return $localize`:@@Added:Added`;
      case 'update':
        return $localize`:@@Updated:Updated`;
      case 'remove':
        return $localize`:@@Removed:Removed`;
      default:
        return $localize`:@@Unchanged:Unchanged`;
    }
  }

  protected kindLabel(kind: string): string {
    switch (kind) {
      case 'route':
        return $localize`:@@Route:Route`;
      case 'role':
        return $localize`:@@Role:Role`;
      case 'authProvider':
        return $localize`:@@Authority:Authority`;
      case 'theme':
        return $localize`:@@Theme:Theme`;
      case 'mailRelay':
        return $localize`:@@Mail_relay:Mail relay`;
      case 'tenant':
        return $localize`:@@Organisation:Organisation`;
      case 'group':
        return $localize`:@@Group:Group`;
      default:
        return $localize`:@@Setting:Setting`;
    }
  }
}
