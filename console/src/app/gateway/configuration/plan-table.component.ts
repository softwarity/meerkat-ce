import { Component, computed, input } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { ConfigPlan } from '../../api.service';

// What a configuration would change, object by object.
//
// ONE table, three places. It was written twice - in the import preview and in
// the comparison - and a pull was about to be the third; three copies of the
// same twelve rows is three places for a new kind to be missing its label, and
// the one that gets forgotten shows an operator the word "Setting" where it
// should say "Organisation". A plan reads the same whoever asked for it:
// importing a file, comparing two saved copies, or pulling from a repository.
//
// The fields of a changed object ride along where the answer carries them: an
// object said to have moved without saying WHAT moved is an invitation to
// download both and diff them by hand.
@Component({
  selector: 'app-plan-table',
  imports: [MatIconModule],
  styles: [
    `
      :host {
        display: block;
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
      .note {
        display: flex;
        gap: 12px;
        padding: 12px 16px;
        border-radius: 8px;
        background: var(--mat-sys-surface-container);
        margin: 12px 0;
      }
      .note mat-icon {
        flex-shrink: 0;
        color: var(--mat-sys-on-surface-variant);
      }
      .note.warn mat-icon {
        color: var(--mat-sys-error);
      }
      .note p,
      .note span {
        margin: 0;
        font-size: 0.85rem;
      }
    `,
  ],
  template: `
    @if (plan(); as p) {
      @if (changes().length === 0) {
        <div class="note">
          <mat-icon>check_circle</mat-icon>
          <span>{{ nothingLabel() }}</span>
        </div>
      } @else {
        <table>
          @for (c of changes(); track c.kind + c.id + c.label) {
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
        </table>
      }
      @if (p.missing.length) {
        <div class="note warn">
          <mat-icon>vpn_key_off</mat-icon>
          <p i18n="@@Plan_missing_note">
            This configuration expects vault entries this installation has not got. They are
            created empty, and anything using one does not serve until it is filled.
          </p>
        </div>
      }
    }
  `,
})
export class PlanTableComponent {
  readonly plan = input<ConfigPlan | null>(null);
  // What to say when nothing moves. It is not the same sentence everywhere: a
  // file that matches is reassuring, two identical configurations are a fact.
  readonly nothing = input('');

  protected readonly nothingLabel = computed(
    () => this.nothing() || $localize`:@@Nothing_would_change_here:Nothing would change.`,
  );

  // The unchanged rows are dropped here rather than by each caller: a plan that
  // listed four hundred untouched objects would bury the three that moved.
  protected readonly changes = computed(
    () => this.plan()?.changes.filter((c) => c.action !== 'same') ?? [],
  );

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
