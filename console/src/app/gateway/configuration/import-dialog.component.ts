import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatRadioModule } from '@angular/material/radio';
import { ApiService, ConfigMissingRef, ConfigPlan } from '../../api.service';

// Where an uploaded file lands, then what it would do, then what it left to
// fill in - one window, in that order, because that is the order the questions
// arrive in.
//
// THREE destinations, and the third is the one that must not be lost when the
// old Import tab goes away: a file may legitimately be a FRAGMENT (three routes
// lifted from a preprod), and adding it to the current configuration is a merge,
// not a switch. The old screen expressed that as a "remove what the file does
// not mention" checkbox; a choice between three sentences says the same thing
// without asking anyone to reason about pruning.
export type ImportOutcome =
  | { where: 'name'; name: string }
  | { where: 'replace' }
  | { where: 'merge' };

@Component({
  selector: 'app-import-dialog',
  imports: [
    MatButtonModule,
    MatChipsModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatRadioModule,
  ],
  styles: [
    `
      mat-dialog-content {
        max-width: min(640px, 84vw);
      }
      .file {
        font-family: var(--mk-mono);
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
        margin: 0 0 16px;
      }
      mat-radio-button {
        display: block;
      }
      .why {
        margin: 0 0 10px 32px;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-form-field {
        width: 100%;
        margin-top: 4px;
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 12px 0;
        font-size: 0.85rem;
      }
      td {
        padding: 4px 8px 4px 0;
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
      .note p {
        margin: 0;
        font-size: 0.85rem;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Import_a_file">Import a file</h2>
    <mat-dialog-content>
      <p class="file">{{ data.filename }}</p>

      <mat-radio-group [value]="where()" (change)="pick($event.value)">
        <mat-radio-button value="name" i18n="@@Save_it_under_a_name">Save it under a name</mat-radio-button>
        <p class="why" i18n="@@Under_a_name_why">
          It joins the saved configurations and is applied to nothing. An existing name is
          replaced.
        </p>
        @if (where() === 'name') {
          <mat-form-field>
            <mat-label i18n="@@Name">Name</mat-label>
            <input matInput [value]="name()" (input)="name.set($any($event.target).value)" />
          </mat-form-field>
        }

        <mat-radio-button value="replace" i18n="@@Replace_the_current_one">
          Replace the current configuration
        </mat-radio-button>
        <p class="why" i18n="@@Replace_current_why">
          After it, this gateway IS that file: what it does not carry goes.
        </p>

        <mat-radio-button value="merge" i18n="@@Add_to_the_current_one">
          Add it to the current configuration
        </mat-radio-button>
        <p class="why" i18n="@@Merge_why">
          For a file that holds a part - a few routes lifted from another gateway. What it does
          not mention is left alone.
        </p>
      </mat-radio-group>

      @if (plan(); as p) {
        @if (!touched(p)) {
          <div class="note">
            <mat-icon>check_circle</mat-icon>
            <p i18n="@@Nothing_would_change">
              This file describes what is already configured here. Importing it would change
              nothing.
            </p>
          </div>
        } @else {
          <table>
            @for (c of p.changes; track c.kind + c.id + c.label) {
              @if (c.action !== 'same') {
                <tr>
                  <td class="act" [class]="c.action">{{ label(c.action) }}</td>
                  <td class="kind">{{ kindLabel(c.kind) }}</td>
                  <td>{{ c.label || c.id }}</td>
                </tr>
              }
            }
          </table>
        }
        @if (p.missing.length) {
          <div class="note warn">
            <mat-icon>vpn_key_off</mat-icon>
            <p i18n="@@Import_missing_note">
              This configuration expects vault entries that do not exist here. They will be
              created empty, and a route that references an empty entry does not serve until it
              is filled.
            </p>
          </div>
        }
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button
        matButton="filled"
        [disabled]="where() === 'name' && !name().trim()"
        (click)="confirm()"
        cdkFocusInitial
      >
        @if (where() === 'name') {
          <ng-container i18n="@@Save">Save</ng-container>
        } @else {
          <ng-container i18n="@@Import_this_file">Import this file</ng-container>
        }
      </button>
    </mat-dialog-actions>
  `,
})
export class ImportDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<ImportDialogComponent, ImportOutcome>);
  protected readonly data = inject<{ filename: string; file: Blob; suggested: string }>(
    MAT_DIALOG_DATA,
  );

  protected readonly where = signal<'name' | 'replace' | 'merge'>('name');
  protected readonly name = signal('');
  protected readonly plan = signal<ConfigPlan | null>(null);

  constructor() {
    this.name.set(this.data.suggested);
  }

  // The plan is only meaningful for the two destinations that touch the
  // gateway, and it differs between them: replacing prunes, adding does not.
  protected pick(where: 'name' | 'replace' | 'merge'): void {
    this.where.set(where);
    this.plan.set(null);
    if (where === 'name') return;
    this.api.previewConfig(this.data.file, where === 'replace').subscribe({
      next: (p) => this.plan.set(p),
      error: () => this.plan.set(null),
    });
  }

  protected confirm(): void {
    const where = this.where();
    this.ref.close(where === 'name' ? { where, name: this.name().trim() } : { where });
  }

  protected touched(plan: ConfigPlan): boolean {
    return plan.changes.some((c) => c.action !== 'same');
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
      default:
        return $localize`:@@Setting:Setting`;
    }
  }
}

// What an import reserved empty, asked for straight away: this is the last
// moment anyone remembers what $ldap-bind was. A week later, nobody does.
@Component({
  selector: 'app-vault-holes-dialog',
  imports: [MatButtonModule, MatDialogModule, MatFormFieldModule, MatIconModule, MatInputModule],
  styles: [
    `
      .entry {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        margin-bottom: 8px;
      }
      .entry mat-form-field {
        flex: 1;
      }
      p.hint {
        margin: 0 0 12px;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Vault_entries_to_fill">Vault entries to fill</h2>
    <mat-dialog-content>
      <p class="hint" i18n="@@Vault_entries_to_fill_hint">
        The import created these empty. This is the moment to fill them: in a week nobody
        remembers what they were for.
      </p>
      @for (m of pending(); track m.name) {
        <div class="entry">
          <mat-form-field>
            <mat-label>{{ m.name }}</mat-label>
            <input
              matInput
              [type]="m.kind === 'secret' ? 'password' : 'text'"
              [value]="values()[m.name]"
              (input)="set(m.name, $any($event.target).value)"
              autocomplete="off"
              spellcheck="false"
            />
          </mat-form-field>
          <button matButton="tonal" [disabled]="!(values()[m.name] || '').trim()" (click)="fill(m)" i18n="@@Save">
            Save
          </button>
        </div>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton="filled" mat-dialog-close i18n="@@Done">Done</button>
    </mat-dialog-actions>
  `,
})
export class VaultHolesDialogComponent {
  private readonly api = inject(ApiService);
  protected readonly data = inject<{ entries: ConfigMissingRef[] }>(MAT_DIALOG_DATA);
  protected readonly pending = signal<ConfigMissingRef[]>([]);
  protected readonly values = signal<Record<string, string>>({});

  constructor() {
    this.pending.set(this.data.entries);
    // Every entry gets a key straight away: an input bound to a missing key
    // renders the string "undefined", which looks like a value and is not.
    this.values.set(Object.fromEntries(this.data.entries.map((m) => [m.name, ''])));
  }

  protected set(name: string, value: string): void {
    this.values.update((v) => ({ ...v, [name]: value }));
  }

  // Filled through the ordinary vault API: nothing special happens here, the
  // entry is simply no longer empty.
  protected fill(entry: ConfigMissingRef): void {
    const value = (this.values()[entry.name] || '').trim();
    if (!value) return;
    this.api
      .saveVaultEntry({ name: entry.name, scope: entry.scope, kind: entry.kind, value })
      .subscribe({
        next: () => this.pending.update((list) => list.filter((m) => m.name !== entry.name)),
      });
  }
}
