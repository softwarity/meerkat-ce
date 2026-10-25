import { booleanAttribute, Component, computed, input, model } from '@angular/core';
import { MatCheckboxModule } from '@angular/material/checkbox';
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
  imports: [MatCheckboxModule, MatIconModule],
  styles: [
    `
      :host {
        display: block;
      }
      .lines {
        max-height: 50vh;
        overflow-y: auto;
        margin: 12px 0;
      }
      table {
        width: 100%;
        border-collapse: collapse;
        font-size: 0.85rem;
      }
      td {
        padding: 4px 8px 4px 0;
        line-height: 20px;
        vertical-align: top;
      }
      /* The checkbox sits in a 40px touch target, its box centred in it: the
         text of a selectable row starts 10px lower, so its first line is
         centred on the box and a second line hangs below. */
      table.selectable td {
        padding-top: 10px;
      }
      table.selectable td.pick {
        padding: 0;
      }
      /* And its box flush with the hint above, not 11px in. */
      td.pick mat-checkbox {
        margin-left: -11px;
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
      .pick {
        width: 48px;
      }
      tr.kept td:not(.pick) {
        opacity: 0.55;
      }
      tr.kept .act {
        text-decoration: line-through;
      }
      /* The box that ticks them all stays in sight while the lines scroll,
         on the dialog's own surface so they pass under it. */
      tr.all td {
        position: sticky;
        top: 0;
        z-index: 1;
        background: var(--mat-sys-surface);
        color: var(--mat-sys-on-surface-variant);
        border-bottom: 1px solid var(--mat-sys-outline-variant);
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
        <!-- A long plan scrolls here, beside its lines, rather than taking
             the notes below and the dialog's buttons with it. -->
        <div class="lines">
          <table [class.selectable]="selectable()">
            @if (selectable()) {
              <!-- Each line can be kept out: what the gateway holds now stays as
                   it is (internal/config/keep.go). Unticked is kept; the box
                   above ticks or unticks them all. -->
              <tr class="all">
                <td class="pick">
                  <mat-checkbox
                    [checked]="kept().length === 0"
                    [indeterminate]="kept().length > 0 && kept().length < keepableCount()"
                    (change)="$event.checked ? kept.set([]) : keepAll()"
                    i18n-aria-label="@@Apply_every_line"
                    aria-label="Apply every line"
                  />
                </td>
                <td colspan="3" i18n="@@Plan_pick_hint">Untick a line to keep its current value.</td>
              </tr>
            }
            @for (c of changes(); track c.kind + c.id + c.label) {
              <tr [class.kept]="isKept(c)">
                @if (selectable()) {
                  <td class="pick">
                    @if (keepable(c)) {
                      <mat-checkbox
                        [checked]="!isKept(c)"
                        (change)="toggle(c, $event.checked)"
                        i18n-aria-label="@@Apply_this_line"
                        aria-label="Apply this line"
                      />
                    }
                  </td>
                }
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
        </div>
      }
      @if (p.missing.length) {
        <div class="note warn">
          <mat-icon>vpn_key_off</mat-icon>
          <p i18n="@@Plan_missing_note">
            This configuration uses vault entries that do not exist here. They are created empty,
            and whatever uses one does not work until it is filled.
          </p>
        </div>
      }
      @if (p.unbound?.length) {
        <div class="note warn">
          <mat-icon>key</mat-icon>
          <div>
            <p i18n="@@Plan_unbound_note">Sign-in through the OIDC provider:</p>
            <ul>
              @for (n of p.unbound; track n) {
                <li>{{ n }}</li>
              }
            </ul>
          </div>
        </div>
      }
      @if (p.notApplied?.length) {
        <div class="note warn">
          <mat-icon>workspace_premium</mat-icon>
          <div>
            <p i18n="@@Plan_not_applied_note">Left out in Community edition:</p>
            <ul>
              @for (n of p.notApplied; track n) {
                <li>{{ n }}</li>
              }
            </ul>
          </div>
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
  // Lines can be kept out (activation, import): what the gateway holds now
  // stays for them. Off for a comparison, which applies nothing.
  readonly selectable = input(false, { transform: booleanAttribute });
  // The lines kept out, as the API takes them: "kind:id".
  readonly kept = model<string[]>([]);

  // A deposited spec or file follows its route: keeping the route keeps them.
  private static readonly keepableKinds = [
    'route', 'role', 'group', 'authProvider', 'oidcClient', 'theme', 'tenant', 'mailRelay', 'setting',
  ];

  protected keepable(c: { kind: string }): boolean {
    return PlanTableComponent.keepableKinds.includes(c.kind);
  }

  private ref(c: { kind: string; id?: string }): string {
    return `${c.kind}:${c.id ?? ''}`;
  }

  // A route's spec and files go with their route.
  protected isKept(c: { kind: string; id?: string }): boolean {
    const kind = c.kind === 'route.spec' || c.kind === 'route.file' ? 'route' : c.kind;
    return this.kept().includes(this.ref({ kind, id: c.id }));
  }

  protected toggle(c: { kind: string; id?: string }, apply: boolean): void {
    const r = this.ref(c);
    this.kept.update((k) => (apply ? k.filter((x) => x !== r) : [...new Set([...k, r])]));
  }

  private readonly allRefs = computed(() => [
    ...new Set(this.changes().filter((c) => this.keepable(c)).map((c) => this.ref(c))),
  ]);
  protected readonly keepableCount = computed(() => this.allRefs().length);

  protected keepAll(): void {
    this.kept.set(this.allRefs());
  }

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
      case 'oidcClient':
        return $localize`:@@OIDC_client:OIDC client`;
      case 'theme':
        return $localize`:@@Theme:Theme`;
      case 'mailRelay':
        return $localize`:@@Mail_relay:Mail relay`;
      case 'tenant':
        return $localize`:@@Organisation:Tenant`;
      case 'group':
        return $localize`:@@Group:Group`;
      default:
        return $localize`:@@Setting:Setting`;
    }
  }
}
