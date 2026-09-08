import { Component, computed, input, model } from '@angular/core';
import { type FormValueControl } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RateLimit, Role, Tenant, User } from '../api.service';
import { ACCESS_LEVELS, AccessEditorComponent, AccessState, emptyAccess, isEmpty, levelShort } from './endpoint-security/access-editor.component';

// What a counter can be keyed on. The order is the order of the answer: the
// bound every route wants first, the narrow ones after.
//
// The hints do the work the labels cannot. "Per user" is obvious and "per
// token" is not - and the difference between bounding a person and bounding
// the integration they wired up is exactly what somebody is choosing here.
// The two labels that name the OBJECT differ by scope: in an operation's
// drawer, "the whole route" names the wrong thing. The key is the same value
// either way - one counter for everything the rule covers - and only the word
// for what is being bounded changes.
type Scope = 'route' | 'operation';

function keysFor(scope: Scope): { value: string; label: string; hint: string }[] {
  const whole =
    scope === 'operation'
      ? {
          label: $localize`:@@Per_operation:The whole operation`,
          hint: $localize`:@@Per_operation_hint:One budget for everything this operation carries, whoever is calling. The bound that protects what is behind it.`,
        }
      : {
          label: $localize`:@@Per_route:The whole route`,
          hint: $localize`:@@Per_route_hint:One budget for everything this route carries, whoever is calling. The bound that protects the service behind it.`,
        };
  const perUser =
    scope === 'operation'
      ? $localize`:@@Per_user_hint_op:One budget per signed-in account, so no single caller can take this operation's whole allowance. Anonymous callers are not covered - add one per address for them.`
      : $localize`:@@Per_user_hint:One budget per signed-in account, so no single caller can take the route's whole allowance. Anonymous callers are not covered - add one per address for them.`;
  return [
    { value: 'route', label: whole.label, hint: whole.hint },
    { value: 'user', label: $localize`:@@Per_user:Each user`, hint: perUser },
    ...KEYS,
  ];
}

const KEYS: { value: string; label: string; hint: string }[] = [
  {
    value: 'token',
    label: $localize`:@@Per_token:Each API token`,
    hint: $localize`:@@Per_token_hint:One budget per token, so an integration is bounded without bounding the person who minted it.`,
  },
  {
    value: 'tenant',
    label: $localize`:@@Per_tenant:Each organisation`,
    hint: $localize`:@@Per_tenant_hint:One budget per organisation - a quota sold to a customer rather than to a person.`,
  },
  {
    value: 'ip',
    label: $localize`:@@Per_ip:Each address`,
    hint: $localize`:@@Per_ip_hint:One budget per client address, which is the only key an anonymous caller has. Read from the address this gateway is talking to, so a forwarded header cannot mint a fresh one.`,
  },
];

// The windows worth offering. ISO 8601, like the route timeouts, because that
// is the duration this product already speaks - and a select rather than a
// text field: nobody should have to know that a minute is written PT1M.
const WINDOWS: { value: string; label: string }[] = [
  { value: 'PT1S', label: $localize`:@@Every_second:second` },
  { value: 'PT10S', label: $localize`:@@Every_ten_seconds:10 seconds` },
  { value: 'PT1M', label: $localize`:@@Every_minute:minute` },
  { value: 'PT5M', label: $localize`:@@Every_five_minutes:5 minutes` },
  { value: 'PT1H', label: $localize`:@@Every_hour:hour` },
  { value: 'P1D', label: $localize`:@@Every_day:day` },
];

// One bound in a few characters, for a table cell. The long form is the
// editor's; a list needs the shape at a glance, and a row that spells out a
// sentence per rule is a row nobody scans.
export function limitLabel(l: RateLimit, scope: Scope = 'route'): string {
  const every = WINDOWS.find((w) => w.value === l.window)?.label ?? l.window;
  const by = keysFor(scope).find((k) => k.value === l.per)?.label ?? l.per;
  return `${l.requests}/${every} - ${by.toLowerCase()}`;
}

// And WHO it is for, in the same few characters the access badges use - AUTH,
// ORG, ORG-2 - or nothing at all when the bound is for everybody.
//
// Without it two bounds differing only by their "Only for" read as the same
// line: the rule was applied, the counters were keyed apart, and the table
// said the same thing twice. A list that cannot tell two rules apart is a list
// that quietly says the second one did nothing.
export function limitScope(l: RateLimit): string {
  const a = toState(l.applies);
  return isEmpty(a) ? '' : levelShort(a);
}

// The same in a sentence, for the tooltip: the short form is for scanning, and
// a reader who stops on one needs it spelled out.
export function limitScopeTip(l: RateLimit): string {
  const a = toState(l.applies);
  if (isEmpty(a)) {
    return $localize`:@@Bound_for_everybody:For everybody calling this.`;
  }
  const level = ACCESS_LEVELS.find((x) => x.value === a.level)?.label ?? '';
  const bits = [level];
  if (a.roles.length) {
    bits.push($localize`:@@With_roles:holding ${a.roles.join(', ')}:roles:`);
  }
  if (a.tenants.length) {
    bits.push($localize`:@@In_orgs:in ${a.tenants.length}:count: organisation(s)`);
  }
  if (a.users.length) {
    bits.push($localize`:@@And_users:and ${a.users.join(', ')}:users:`);
  }
  return $localize`:@@Bound_only_for:Only for: ${bits.filter(Boolean).join(', ')}:what:`;
}

function toState(a: RateLimit['applies']): AccessState {
  if (!a) return emptyAccess();
  return {
    level: a.level ?? '',
    tenants: a.tenants ?? [],
    roles: a.roles ?? [],
    users: a.users ?? [],
  };
}

// The rate limits of a route or of one operation (ROUTE-08, QUOTA-01/05).
//
// The word on the screen is PER, and it is the whole idea: nobody writes a
// limit FOR alice - a list of names is a list nobody maintains - so a rule
// says what to count BY and the budgets make themselves, one per caller.
//
// SEVERAL AT ONCE, which is where this differs from the access rules one panel
// over: those are read first-match-wins, these are all true together. Five
// thousand a minute for the route, a hundred per user, sixty per address - and
// the first one exceeded answers 429.
//
// "Only for" reuses the access editor, and that is not a saving: it means a
// role IS a pricing tier. Two rules, one narrowed to partner and one to trial,
// and nobody has been named.
@Component({
  selector: 'app-rate-limits',
  imports: [
    AccessEditorComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  template: `
    @for (l of value(); track $index) {
      <div class="limit">
        <div class="row">
          <mat-form-field class="per" subscriptSizing="dynamic">
            <mat-label i18n="@@Count_by">Count by</mat-label>
            <mat-select [value]="l.per" (selectionChange)="patch($index, { per: $event.value })">
              @for (k of keys(); track k.value) {
                <mat-option [value]="k.value">{{ k.label }}</mat-option>
              }
            </mat-select>
          </mat-form-field>

          <mat-form-field class="requests" subscriptSizing="dynamic">
            <mat-label i18n="@@Requests">Requests</mat-label>
            <input
              matInput
              type="number"
              min="1"
              [value]="l.requests"
              (input)="patch($index, { requests: +$any($event.target).value })"
            />
          </mat-form-field>

          <mat-form-field class="window" subscriptSizing="dynamic">
            <mat-label i18n="@@Per_every">Every</mat-label>
            <mat-select [value]="l.window" (selectionChange)="patch($index, { window: $event.value })">
              @for (w of windows; track w.value) {
                <mat-option [value]="w.value">{{ w.label }}</mat-option>
              }
            </mat-select>
          </mat-form-field>

          <span class="grow"></span>
          <button
            matIconButton
            type="button"
            (click)="remove($index)"
            i18n-matTooltip="@@Remove"
            matTooltip="Remove"
            i18n-aria-label="@@Remove"
            aria-label="Remove"
          >
            <mat-icon>close</mat-icon>
          </button>
        </div>

        <p class="hint">{{ hintFor(l.per) }}</p>

        <!-- Who the rule is about, which is a separate question from what it
             counts by. Folded away until somebody wants it: most bounds are
             for everybody, and a rule that opens on an empty access editor
             reads as a decision waiting to be made. -->
        @if (narrowed($index)) {
          <div class="only">
            <div class="only-head">
              <span i18n="@@Only_for">Only for</span>
              <button matButton type="button" (click)="widen($index)" i18n="@@Everybody">Everybody</button>
            </div>
            <app-access-editor
              purpose="selects"
              [value]="appliesOf(l)"
              [users]="users()"
              [roles]="roles()"
              [tenants]="tenants()"
              (valueChange)="patch($index, { applies: fromState($event) })"
            />
          </div>
        } @else {
          <button matButton type="button" class="narrow" (click)="narrow($index)">
            <mat-icon>filter_alt</mat-icon>
            <ng-container i18n="@@Only_for_some">Only for some callers</ng-container>
          </button>
        }
      </div>
    } @empty {
      @if (scope() === 'route') {
        <p class="hint" i18n="@@No_limit_hint">
          Nothing is bounded until a bound is written. Add one for the whole route to protect the
          service behind it, one per user so a single caller cannot take it all, one per address
          for whoever has no account - they are all true at once, and the first one exceeded
          answers 429.
        </p>
      } @else {
        <p class="hint" i18n="@@No_op_limit_hint">
          This operation is bounded by whatever the route carries, and by nothing of its own. Add a
          bound here for the one operation that costs more than the others.
        </p>
      }
    }

    <!-- The question this screen could not answer on its own: a rule with an
         "Only for" bounds THOSE callers, and nobody else. Written as three
         narrow rules, a route looks bounded and is wide open to everyone the
         three do not describe - which is exactly the case somebody thinks they
         have covered. -->
    @if (noneForEverybody()) {
      <p class="warn">
        <mat-icon>report</mat-icon>
        <span i18n="@@Only_for_everyone_else">
          Every bound here is for some callers. Anyone the rules above do not describe is not
          bounded at all - add one with no "Only for" to cover everybody.
        </span>
      </p>
    }

    <!-- The number written here is what ONE node allows. The counter is a
         sliding window in memory, per node, and that is a decision: an exact
         shared counter costs a round trip to the database on every request,
         which is not a price a gateway pays on the path of all traffic. But a
         number that means something else on a cluster than it does alone has
         to say so where it is typed - reading it as an installation total is
         the mistake, and it is invisible until the day it matters. -->
    @if (value().length) {
      <p class="hint" i18n="@@Bound_is_per_node">
        Counted per node, in memory: an exact shared counter would cost a database round trip on
        every request. On a cluster of N gateways the installation therefore allows up to N times
        what is written here - divide it, or accept the multiplication.
      </p>
    }

    <button matButton="tonal" type="button" (click)="add()">
      <mat-icon>add</mat-icon>
      <ng-container i18n="@@Add_a_limit">Add a limit</ng-container>
    </button>
  `,
  styles: `
    :host {
      display: block;
    }
    .limit {
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 12px;
      padding: 12px 14px;
      margin-bottom: 12px;
    }
    .row {
      display: flex;
      align-items: flex-start;
      gap: 12px;
      flex-wrap: wrap;
    }
    .per {
      min-width: 180px;
    }
    .requests {
      width: 110px;
    }
    .window {
      width: 140px;
    }
    .grow {
      flex: 1;
    }
    .hint {
      margin: 8px 0 0;
      font-size: 0.8rem;
      line-height: 1.45;
      color: var(--mat-sys-on-surface-variant);
      max-width: 72ch;
    }
    .narrow {
      margin-top: 8px;
    }
    /* Said, not shouted: it is a state worth noticing and not a mistake - two
       tiers with no catch-all is a legitimate configuration on a route whose
       upstream does its own bounding. */
    .warn {
      display: flex;
      align-items: flex-start;
      gap: 8px;
      margin: 0 0 12px;
      padding: 10px 12px;
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 10px;
      font-size: 0.8rem;
      line-height: 1.45;
      color: var(--mat-sys-on-surface-variant);
      max-width: 72ch;
    }
    .warn mat-icon {
      color: var(--mat-sys-tertiary);
      flex: none;
    }
    .only {
      margin-top: 12px;
      padding-top: 12px;
      border-top: 1px solid var(--mat-sys-outline-variant);
    }
    .only-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-size: 0.8rem;
      color: var(--mat-sys-on-surface-variant);
      margin-bottom: 8px;
    }
  `,
})
export class RateLimitsComponent implements FormValueControl<RateLimit[]> {
  readonly value = model<RateLimit[]>([]);
  // What these bounds are ON, which only changes what the empty state says -
  // and that sentence is the one somebody reads before they have written
  // anything, so it has to be about the thing in front of them.
  readonly scope = input<Scope>('route');
  readonly users = input<User[]>([]);
  readonly roles = input<Role[]>([]);
  readonly tenants = input<Tenant[]>([]);

  protected readonly keys = computed(() => keysFor(this.scope()));
  protected readonly windows = WINDOWS;

  // Whether anything here bounds a caller the narrow rules do not describe.
  protected readonly noneForEverybody = computed(() => {
    const rules = this.value();
    return rules.length > 0 && rules.every((r) => !isEmpty(this.toState(r.applies)));
  });

  protected hintFor(per: string): string {
    return this.keys().find((k) => k.value === per)?.hint ?? '';
  }

  // A bound for everybody is the ordinary one, so that is what Add makes.
  protected add(): void {
    this.value.update((l) => [...l, { per: 'route', requests: 100, window: 'PT1M' }]);
  }

  protected remove(i: number): void {
    this.value.update((l) => l.filter((_, n) => n !== i));
  }

  protected patch(i: number, change: Partial<RateLimit>): void {
    this.value.update((l) => l.map((r, n) => (n === i ? { ...r, ...change } : r)));
  }

  // Narrowing starts from "signed in", not from nothing: an empty rule means
  // everybody, so an editor opening on it would show a narrowing that narrows
  // nothing and save a rule identical to the one before.
  protected narrow(i: number): void {
    this.patch(i, { applies: { level: 'auth' } });
  }

  protected widen(i: number): void {
    this.patch(i, { applies: undefined });
  }

  protected narrowed(i: number): boolean {
    const a = this.value()[i]?.applies;
    return !!a && !isEmpty(this.toState(a));
  }

  protected appliesOf(l: RateLimit): AccessState {
    return this.toState(l.applies);
  }

  private toState(a: RateLimit['applies']): AccessState {
    return toState(a);
  }

  // Back to the wire shape, dropping what is empty: a rule carrying four empty
  // arrays reads as four decisions in an export, and it is one.
  protected fromState(s: AccessState): RateLimit['applies'] {
    const out: NonNullable<RateLimit['applies']> = {};
    if (s.level) out.level = s.level;
    if (s.tenants.length) out.tenants = s.tenants;
    if (s.roles.length) out.roles = s.roles;
    if (s.users.length) out.users = s.users;
    return out;
  }
}
