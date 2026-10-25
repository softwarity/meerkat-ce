import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, input, linkedSignal, output, signal } from '@angular/core';
import { FormField, applyEach, form, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, Role, RoleMapSave } from '../../api.service';

// The select's value for "create it": a NUL cannot be in a role name, so it
// can never be mistaken for a catalogue role.
const NEW_ROLE = '\u0000new';

// Anything a catalogue role name cannot hold becomes a hyphen, runs collapsed.
function catalogueSafe(name: string): string {
  return name.replace(/[^A-Za-z0-9_-]+/g, '-');
}

interface MapRow {
  declared: string;
  // A catalogue role name, NEW_ROLE, or '' for "not mapped".
  target: string;
}
interface MapDraft {
  prefix: string;
  tag: string;
  rows: MapRow[];
}

// The roles a route's OpenAPI spec declares, each mapped onto a catalogue role.
//
// The application names its roles the way it likes (ROLE_ADMIN), the catalogue
// holds the estate's, and two applications both saying ROLE_ADMIN do not mean
// the same person. So each declared role points at one catalogue role - a new
// one under the route's prefix, or one that already exists - and Auth forward
// then sends the declared roles whose catalogue role the person holds, under
// the APPLICATION's names. Nothing declared is forwarded until it is mapped.
//
// One explicit Save, unlike the rest of this screen: a save here may create
// roles in the catalogue, and that is not something a select should do on its
// own while somebody is still choosing.
@Component({
  selector: 'app-role-map',
  imports: [FormField, MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, MatSelectModule],
  template: `
    <p class="hint" i18n="@@Role_map_intro">
      Roles named in the application's spec. Map each to a role of the catalogue: Auth forward then sends
      the ones the person holds, under the application's names. A role left unmapped is never sent.
    </p>

    <div class="applied">
      <mat-form-field subscriptSizing="dynamic">
        <mat-label i18n="@@Role_map_prefix">Prefix of the roles created</mat-label>
        <input matInput [formField]="f.prefix" spellcheck="false" />
      </mat-form-field>
      <mat-form-field subscriptSizing="dynamic">
        <mat-label i18n="@@Role_map_tag">Tag of the roles created</mat-label>
        <input matInput [formField]="f.tag" spellcheck="false" />
      </mat-form-field>
    </div>

    <div class="rows">
      @for (row of f.rows; track $index) {
        <code class="declared">{{ row.declared().value() }}</code>
        <mat-form-field subscriptSizing="dynamic">
          <mat-label i18n="@@Role_map_maps_to">Maps to</mat-label>
          <mat-select [formField]="row.target">
            <mat-option [value]="NEW_ROLE" i18n="@@Role_map_new_role"
              >New role: {{ newRoleName(draft().prefix, row.declared().value()) }}</mat-option
            >
            @if (gone(row.target().value()); as g) {
              <mat-option [value]="g">{{ g }}</mat-option>
            }
            @for (r of catalogue(); track r) {
              <mat-option [value]="r">{{ r }}</mat-option>
            }
            <mat-option value="" i18n="@@Role_map_not_mapped">Not mapped</mat-option>
          </mat-select>
        </mat-form-field>
        @if (row.target().errors()[0]?.message; as m) {
          <p class="gone">
            <mat-icon inline>warning</mat-icon>
            {{ m }}
          </p>
        }
      }
    </div>

    @if (!rolesReadable()) {
      <p class="hint" i18n="@@Role_map_catalogue_unreadable">
        The catalogue is not readable with your rights: only new roles can be chosen.
      </p>
    }

    <div class="actions">
      <button matButton="tonal" type="button" [disabled]="saving() || f().invalid()" (click)="save()">
        <ng-container i18n="@@Save">Save</ng-container>
      </button>
      @if (creates(); as n) {
        <span class="hint" i18n="@@Role_map_creates">Creates {{ n }} role(s) in the catalogue.</span>
      }
      @if (error(); as e) {
        <span class="err">{{ e }}</span>
      } @else if (done()) {
        <span class="ok">
          <mat-icon inline>check_circle</mat-icon>
          <ng-container i18n="@@Role_map_saved">Mapping saved</ng-container>
        </span>
      }
    </div>
  `,
  styles: [
    `
      :host {
        display: block;
      }
      .hint {
        margin: 0 0 16px;
        font-size: 0.82rem;
        line-height: 1.45;
        color: var(--mat-sys-on-surface-variant);
        max-width: 72ch;
      }
      .applied {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 12px;
        margin-bottom: 20px;
      }
      .rows {
        display: grid;
        grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr);
        gap: 8px 16px;
        align-items: center;
      }
      .declared {
        font-family: var(--mk-mono);
        font-size: 0.85rem;
        word-break: break-all;
      }
      .gone {
        grid-column: 1 / -1;
        margin: -4px 0 4px;
        font-size: 0.8rem;
        color: var(--mat-sys-error);
      }
      .actions {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 12px;
        margin-top: 20px;
      }
      .actions .hint {
        margin: 0;
      }
      .err {
        font-size: 0.85rem;
        color: var(--mat-sys-error);
      }
      .ok {
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
})
export class RoleMapComponent {
  private readonly api = inject(ApiService);

  readonly routeId = input.required<string>();
  readonly routeName = input.required<string>();
  // Every distinct role the spec declares, and the saved mapping of each.
  readonly declared = input.required<string[]>();
  readonly roleMap = input<Record<string, string> | undefined>(undefined);
  // The catalogue, and whether it could be read at all: an app admin's
  // catalogue is never empty, but a caller without the right to read it gets
  // nothing, and then "existing role" is not a choice anybody can make here.
  readonly roles = input.required<Role[]>();
  readonly rolesReadable = input(true);
  // What the gateway kept, for the screen to mark its operations with.
  readonly saved = output<Record<string, string>>();

  protected readonly NEW_ROLE = NEW_ROLE;

  protected readonly catalogue = computed(() => this.roles().map((r) => r.name).sort((a, b) => a.localeCompare(b)));
  private readonly known = computed(() => new Set(this.catalogue()));

  // Reseeded whenever what it is built from changes: another route, a save
  // that came back, a catalogue reloaded with the roles just created. A
  // declared role with no saved mapping starts on "New role", the choice most
  // of them end on.
  protected readonly draft = linkedSignal<MapDraft>(() => {
    const name = this.routeName();
    const map = this.roleMap() ?? {};
    return {
      prefix: name ? `${catalogueSafe(name).replace(/^-+|-+$/g, '')}-` : '',
      tag: name,
      rows: this.declared().map((declared) => ({ declared, target: map[declared] ?? NEW_ROLE })),
    };
  });

  protected readonly f = form(this.draft, (p) => {
    applyEach(p.rows, (row) => {
      validate(row.target, ({ value }) => {
        const v = value();
        if (!v || v === NEW_ROLE || !this.rolesReadable() || this.known().has(v)) return [];
        return [{ kind: 'gone', message: $localize`:@@Role_map_target_gone:${v}:ROLE: is no longer in the catalogue: map this role again.` }];
      });
    });
  });

  // A saved target the catalogue no longer has: still offered, so the select
  // shows what was chosen rather than a blank that reads as "not mapped".
  // When the catalogue cannot be read, every saved target is one of those.
  protected gone(target: string): string {
    return target && target !== NEW_ROLE && !this.known().has(target) ? target : '';
  }

  // The roles a save would create: the "New role" choices the catalogue does
  // not hold yet. Unreadable, all of them go: the gateway skips the ones that
  // exist.
  private readonly body = computed<RoleMapSave>(() => {
    const d = this.draft();
    const prefix = d.prefix.trim();
    const tag = d.tag.trim();
    const roleMap: Record<string, string> = {};
    const create: RoleMapSave['create'] = [];
    for (const row of d.rows) {
      if (!row.target) continue;
      const name = row.target === NEW_ROLE ? this.newRoleName(prefix, row.declared) : row.target;
      roleMap[row.declared] = name;
      if (row.target === NEW_ROLE && !this.known().has(name) && !create.some((c) => c.name === name)) {
        create.push({ name, tags: tag ? [tag] : [] });
      }
    }
    return { roleMap, create };
  });
  protected readonly creates = computed(() => this.body().create.length);

  // The catalogue name a "New role" gets. A catalogue role is letters,
  // digits, - and _ only - what a session can carry - while a route's name
  // has spaces and a declared role may be a scope (orders:read): both are
  // made to fit rather than refused on save.
  protected newRoleName(prefix: string, declared: string): string {
    return catalogueSafe(prefix.trim() + declared);
  }

  protected readonly saving = signal(false);
  protected readonly error = signal('');
  // "Saved" holds while the screen still shows what was saved: the next
  // change takes it away rather than leaving a claim the draft no longer backs.
  private readonly savedAs = signal('');
  protected readonly done = computed(() => this.savedAs() === JSON.stringify(this.body().roleMap));

  protected save(): void {
    if (this.f().invalid()) return;
    this.saving.set(true);
    this.error.set('');
    this.savedAs.set('');
    const body = this.body();
    this.api.saveRoleMap(this.routeId(), body).subscribe({
      next: (res) => {
        this.saving.set(false);
        // As the gateway kept it, which is also what the reseeded draft will
        // read once the screen hands the mapping back.
        this.savedAs.set(JSON.stringify(res.roleMap ?? {}));
        this.saved.emit(res.roleMap ?? {});
      },
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(
          e.status === 403
            ? $localize`:@@Role_map_forbidden:Creating roles needs the right to write the catalogue. Map onto existing roles, or ask an application admin.`
            : (e.error?.error ?? $localize`:@@Request_failed:Request failed`),
        );
      },
    });
  }
}
