import { Component, computed, input, output } from '@angular/core';
import { MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSelectModule } from '@angular/material/select';
import { AccessLevel, Role, Tenant, User } from '../../api.service';

// The editable, non-optional shape of an access rule inside the editor.
export interface AccessState {
  level: AccessLevel;
  tenants: string[];
  roles: string[];
  users: string[];
}

export function emptyAccess(): AccessState {
  return { level: '', tenants: [], roles: [], users: [] };
}

// Empty means Meerkat poses no condition of its own; the upstream still
// applies whatever it applies.
export function isEmpty(a: AccessState): boolean {
  return a.level === '' && a.roles.length === 0 && a.users.length === 0;
}

// The belonging axis, in order. Each entry says what it REQUIRES - and none of
// them says what the SERVICE does, because whatever is chosen here the upstream
// still applies its own rules afterwards. Meerkat gates in addition to it,
// never instead of it, which is why the open end is "delegated" and not
// "public": a level named public would promise something Meerkat cannot grant.
export const ACCESS_LEVELS: { value: AccessLevel; label: string; hint: string }[] = [
  {
    value: '',
    label: $localize`:@@Access_delegated:Delegated`,
    hint: $localize`:@@Access_delegated_hint:Meerkat lets everyone through, signed in or not, and leaves the decision to the service.`,
  },
  {
    value: 'auth',
    label: $localize`:@@Access_authenticated:Signed in`,
    hint: $localize`:@@Access_authenticated_hint:Anyone with an account, including one that belongs to no organisation yet.`,
  },
  {
    value: 'tenant',
    label: $localize`:@@Access_in_an_organisation:In an organisation`,
    hint: $localize`:@@Access_in_an_organisation_hint:An organisation must be active on the session. Turns away an account still awaiting access.`,
  },
  {
    value: 'tenants',
    label: $localize`:@@Access_in_one_of_these:In one of these organisations`,
    hint: $localize`:@@Access_in_one_of_these_hint:The active organisation must be one of those named below.`,
  },
  {
    value: 'deny',
    label: $localize`:@@Access_nobody:Nobody`,
    hint: $localize`:@@Access_nobody_hint:Refused before the service is called. Only the users named as an exception get through.`,
  },
];

// One access rule, edited the same way for the whole route and for a single
// endpoint (RBAC-06/07). TWO AXES, ANDed: the belonging level (plus the named
// organisations when it asks for them), and a role filter evaluated in the
// ACTIVE organisation.
//
// Whatever the level, the service still applies its own rules on top: this
// screen adds conditions, it never removes any.
//
// The axes cross on purpose: the role catalogue is global while groups belong
// to an organisation, so a bare role gate means "an admin of ANY organisation" -
// a cross-org console - while naming organisations too means "an admin OF
// Acme". Named users are the exception, not a level: whoever is listed passes
// whatever the level requires (a service account, a support login, an
// application dedicated to one person).
@Component({
  selector: 'app-access-editor',
  imports: [MatChipsModule, MatFormFieldModule, MatIconModule, MatSelectModule],
  template: `
    <!-- The pivot. Four short labels, so it takes the width it needs and
         leaves the rest alone. -->
    <mat-form-field class="level" subscriptSizing="dynamic">
      <mat-label i18n="@@Who_may_call_it">Who may call it</mat-label>
      <mat-select [value]="value().level" (selectionChange)="patch({ level: $event.value })">
        @for (l of levels; track l.value) {
          <mat-option [value]="l.value">
            <span class="opt-main">{{ l.label }}</span>
          </mat-option>
        }
      </mat-select>
      <mat-hint align="end">{{ levelHint() }}</mat-hint>
    </mat-form-field>

    <!-- What only exists BECAUSE of the answer above, set apart so the two are
         not read as four independent questions. One per row: their labels are
         sentences, and side by side both got cut to the same half-word. -->
    @if (narrowable()) {
      <div class="depends">
        @if (value().level === 'tenants') {
          <mat-form-field class="field" subscriptSizing="dynamic">
            <mat-label i18n="@@Organisations_any_of">Organisations (any one grants access)</mat-label>
            <mat-select multiple [value]="value().tenants" (selectionChange)="patch({ tenants: $event.value })">
              <mat-select-trigger>
                @if (value().tenants.length) {
                <mat-chip-set class="chips">
                  @for (id of value().tenants; track id) {
                    <mat-chip (removed)="drop('tenants', id)">
                      {{ tenantName(id) }}
                      <button matChipRemove (click)="$event.stopPropagation()" [attr.aria-label]="tenantName(id)">
                        <mat-icon>cancel</mat-icon>
                      </button>
                    </mat-chip>
                  }
                </mat-chip-set>
                }
              </mat-select-trigger>
              @for (t of tenants(); track t.id) {
                <mat-option [value]="t.id">
                  <span class="opt-main">{{ t.name }}</span>
                  @if (t.description) {
                    <span class="opt-sub">{{ t.description }}</span>
                  }
                </mat-option>
              }
            </mat-select>
          </mat-form-field>
        }

        <mat-form-field class="field" subscriptSizing="dynamic">
          <mat-label i18n="@@Roles_any_of">Roles (any one grants access)</mat-label>
          <mat-select multiple [value]="value().roles" (selectionChange)="patch({ roles: $event.value })">
            <mat-select-trigger>
              @if (value().roles.length) {
              <mat-chip-set class="chips">
                @for (name of value().roles; track name) {
                  <mat-chip (removed)="drop('roles', name)">
                    {{ name }}
                    <button matChipRemove (click)="$event.stopPropagation()" [attr.aria-label]="name">
                      <mat-icon>cancel</mat-icon>
                    </button>
                  </mat-chip>
                }
              </mat-chip-set>
              }
            </mat-select-trigger>
            @for (r of roles(); track r.id) {
              <mat-option [value]="r.name">
                <span class="opt-main">{{ r.name }}</span>
                @if (r.description) {
                  <span class="opt-sub">{{ r.description }}</span>
                }
              </mat-option>
            }
          </mat-select>
          <mat-hint align="end" i18n="@@Roles_in_active_org_hint">Held in the active organisation. Left empty, any role passes.</mat-hint>
        </mat-form-field>
      </div>
    }

    <!-- Answers no question above: it overrides all of them. Which is why it
         only shows when there IS something to override - under a delegated
         route everyone is already through, and naming someone says nothing. -->
    @if (value().level !== '') {
      <div class="exception">
        <div class="exception-head" i18n="@@Exception">Exception</div>
        <mat-form-field class="field" subscriptSizing="dynamic">
          <mat-label i18n="@@Users_always_allowed">Users always allowed</mat-label>
          <mat-select multiple [value]="value().users" (selectionChange)="patch({ users: $event.value })">
            <mat-select-trigger>
              @if (value().users.length) {
              <mat-chip-set class="chips">
                @for (name of value().users; track name) {
                  <mat-chip (removed)="drop('users', name)">
                    {{ name }}
                    <button matChipRemove (click)="$event.stopPropagation()" [attr.aria-label]="name">
                      <mat-icon>cancel</mat-icon>
                    </button>
                  </mat-chip>
                }
              </mat-chip-set>
              }
            </mat-select-trigger>
            @for (u of users(); track u.id) {
              <mat-option [value]="u.username">
                <span class="opt-main">{{ u.username }}</span>
                @if (u.email) {
                  <span class="opt-sub">{{ u.email }}</span>
                }
              </mat-option>
            }
          </mat-select>
          <mat-hint align="end" i18n="@@Users_exception_hint">They pass whatever is required above.</mat-hint>
        </mat-form-field>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        align-items: stretch;
        gap: 22px;
      }
      .field {
        width: 100%;
      }
      /* The pivot answers with four short words: a full-width select for them
         reads as an empty field. */
      .level {
        width: 100%;
        max-width: 380px;
      }
      /* Set apart, and marked as belonging to the answer above: the rule is
         read as one question with its consequences, not as four questions. */
      /* One per row: their labels are sentences, and side by side both got
         truncated to say the same half-word. */
      .depends {
        display: flex;
        flex-direction: column;
        gap: 16px;
        padding: 4px 0 4px 16px;
        box-shadow: inset 3px 0 0 0 var(--mat-sys-outline-variant);
      }
      .exception {
        padding-top: 18px;
        border-top: 1px solid var(--mat-sys-outline-variant);
      }
      .exception-head {
        margin-bottom: 10px;
        font-size: 0.72rem;
        font-weight: 600;
        letter-spacing: 0.06em;
        text-transform: uppercase;
        color: var(--mat-sys-on-surface-variant);
      }
      /* Two lines per entry: the name is what is picked, the description is
         what tells them apart. Side by side they wrapped into each other. */
      .opt-main {
        display: block;
        line-height: 1.4;
      }
      .opt-sub {
        display: block;
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.78rem;
        line-height: 1.35;
      }
      /* What is chosen, as chips that can be taken back one by one - a
         comma-separated line said the same thing but could not be undone
         without reopening the list. */
      .chips {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
        padding: 4px 0;
      }
      .chips mat-chip {
        --mat-chip-container-height: 26px;
        --mat-chip-label-text-size: 0.78rem;
      }
    `,
  ],
})
export class AccessEditorComponent {
  readonly value = input.required<AccessState>();
  readonly users = input<User[]>([]);
  readonly roles = input<Role[]>([]);
  readonly tenants = input<Tenant[]>([]);
  readonly valueChange = output<AccessState>();

  protected readonly levels = ACCESS_LEVELS;
  protected readonly levelHint = computed(
    () => ACCESS_LEVELS.find((l) => l.value === this.value().level)?.hint ?? '',
  );
  // Whether the rule can be narrowed further. Delegated lets everyone through
  // and deny lets nobody: neither has a caller left to filter on roles.
  protected readonly narrowable = computed(() => {
    const l = this.value().level;
    return l !== '' && l !== 'deny';
  });
  // Ids are what travels; names are what is read.
  protected tenantName(id: string): string {
    return this.tenants().find((t) => t.id === id)?.name ?? id;
  }

  // Taking one back from the chips, without reopening the list.
  protected drop(axis: 'tenants' | 'roles' | 'users', value: string): void {
    this.patch({ [axis]: this.value()[axis].filter((v) => v !== value) });
  }

  // Leaving a level drops what only that level meant: named organisations are
  // meaningless anywhere else, and a role filter cannot be evaluated with no
  // organisation to hold it. Keeping them would save a rule the gateway reads
  // differently from the screen that wrote it.
  protected patch(p: Partial<AccessState>): void {
    const next = { ...this.value(), ...p };
    if (next.level !== 'tenants') next.tenants = [];
    // No caller left to filter: everyone is through (delegated) or nobody is
    // (deny). And under delegated a named user excepts them from nothing.
    if (next.level === '' || next.level === 'deny') next.roles = [];
    if (next.level === '') next.users = [];
    this.valueChange.emit(next);
  }
}
