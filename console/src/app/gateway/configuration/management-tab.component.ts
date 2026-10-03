import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { DatePipe } from '@angular/common';
import { httpResource } from '@angular/common/http';
import { ActivatedRoute, Router } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs/operators';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Observable, firstValueFrom } from 'rxjs';
import { RowActionsDirective } from '@softwarity/row-actions';
import {
  ApiService,
  ConfigPlan,
  ConfigRemote,
  CurrentConfiguration,
  Edition,
  SavedConfiguration,
} from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { ConfigurationYamlComponent } from './configuration-yaml.component';
import { CompareDialogComponent } from './compare-dialog.component';
import { ExportDialogComponent } from './export-dialog.component';
import { ImportDialogComponent, VaultHolesDialogComponent } from './import-dialog.component';
import {
  PickLocationDialogComponent,
  PullResultDialogComponent,
} from './pick-location-dialog.component';
import { LiveChangesService } from '../../shared/live-changes.service';

// One row per configuration, and the CURRENT one is the first of them.
//
// That is the whole design. What the gateway serves is not a card off to the
// side and not a mode: it is a configuration like the others, on the same line
// grammar, with the same file behind it - it simply happens to be the one in
// force, so it sits at the top and stays there while the shelf scrolls under
// it. Everything else follows from that: the current row saves and exports,
// the others set-as-current, duplicate, export and delete.
//
// Only "set as current" crosses from the shelf to the gateway, so it is the
// only action that asks - with the LIST OF WHAT IT WOULD CHANGE, and with a
// warning first when what is running was never saved: switching away from an
// unnamed state is the one move here with nothing to come back to.
type Row = SavedConfiguration & { current?: boolean };

@Component({
  selector: 'app-configuration-management',
  imports: [
    DatePipe,
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatIconModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    EeLockComponent,
    ConfigurationYamlComponent,
  ],
  styleUrl: './configuration-cards.scss',
  styles: [
    `
      /* Full width and full height: this tab is a table with a drawer, not a
         column of text. */
      :host {
        display: block;
        height: 100%;
        max-width: none;
        padding: 0;
      }
      mat-drawer-container {
        background: transparent;
        height: 100%;
      }
      mat-drawer-content {
        display: flex;
        flex-direction: column;
        overflow: auto;
        padding: 0 4px 24px;
      }
      mat-drawer {
        width: min(760px, 96vw);
        border-left: 1px solid var(--mat-sys-outline-variant);
        background: var(--mat-sys-surface-container-high);
      }
      .shelf {
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
        white-space: nowrap;
      }
      .shelf.full {
        color: var(--mat-sys-error);
      }
      .shelf .ee {
        margin-left: 4px;
      }
      .top {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 8px 0 12px;
        flex-wrap: wrap;
      }
      .top .grow {
        flex: 1;
      }
      mat-table {
        background: transparent;
      }
      mat-header-row {
        background: var(--mat-sys-surface);
        position: sticky;
        top: 0;
        z-index: 3;
      }
      /* The current configuration stays under the header while the shelf
         scrolls: it is the reference every other row is read against.

         The offset is the HEADER'S OWN token, not the number it happens to
         produce today: a hardcoded 56px is a copy of a Material metric that
         nobody would think to update when the theme's density changes, and it
         fails by half-covering a row rather than by breaking a build. */
      mat-row.current {
        position: sticky;
        top: var(--mat-table-header-container-height, 56px);
        z-index: 2;
        background: var(--mat-sys-surface-container);
      }
      mat-row {
        cursor: pointer;
      }
      mat-cell,
      mat-header-cell {
        padding: 0 8px;
      }
      /* Names and dates have a size; the description takes what is left. A
         name that wraps onto two lines makes the sticky row taller than the
         rows it is meant to sit above. */
      .mat-column-name {
        flex: 0 0 420px;
      }
      .mat-column-updated {
        flex: 0 0 260px;
        justify-content: flex-end;
      }
      .name {
        display: flex;
        align-items: center;
        gap: 8px;
        font-weight: 500;
      }
      .label {
        flex: none;
        white-space: nowrap;
      }
      .as {
        min-width: 0;
        color: var(--mat-sys-on-surface-variant);
        font-weight: 400;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .state.saved {
        color: var(--mat-sys-primary);
      }
      .state.drifted {
        color: var(--mat-sys-error);
      }
      .state.unsaved {
        color: var(--mat-sys-on-surface-variant);
      }
      mat-chip.stale {
        --mdc-chip-outline-color: var(--mat-sys-error);
        --mdc-chip-label-text-color: var(--mat-sys-error);
      }
      .desc {
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .when {
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.8rem;
        font-variant-numeric: tabular-nums;
      }
      input[type='file'] {
        display: none;
      }
      /* Where a row lives in git, and whether it is level with it. One icon and
         one name: the full URL belongs in the tooltip, where a line of it does
         not push the dates off the row. */
      .git {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .git mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
      }
      .git.ahead {
        color: var(--mat-sys-primary);
      }
      .git.behind,
      .git.diverged {
        color: var(--mat-sys-error);
      }
    `,
  ],
  template: `
    <mat-drawer-container>
      <mat-drawer-content>
        <div class="top">
          <p class="hint grow" style="margin: 0" i18n="@@Management_hint">
            The first row is what this gateway serves; the others are saved copies that serve
            nothing. Click a row to read its file.
          </p>
          <!-- The size of the shelf, said before it is reached: a cap found by
               being refused is a trap, a cap announced is a price.
               ONLY where there is one. Enterprise has no limit, so counting the
               rows there announced nothing - it was a number beside a list that
               already shows them, and it read as a quota to somebody who has
               none. -->
          @if (cap(); as n) {
            <span class="shelf" [class.full]="saved().length >= n">
              <ng-container i18n="@@Configurations_shelf">{{ saved().length }} of {{ n }} saved</ng-container>
              <span class="ee" i18n="@@Configurations_shelf_ee">- no limit in Enterprise</span>
            </span>
          }
          <!-- No lock: a file exported here must be able to come back in on
               the same image. What the community image does not run is taken
               out of the document by the server and named in the plan. -->
          <button matButton="outlined" [disabled]="busy()" (click)="picker.click()">
            <mat-icon>upload_file</mat-icon>
            <ng-container i18n="@@Import_a_file">Import a file</ng-container>
          </button>
          <!-- The same act as importing a file, with the bytes coming from a
               repository instead of a download folder: it SHELVES and applies
               nothing. Hence a button beside that one rather than a screen of
               its own. -->
          <button matButton="outlined" ee-feature="configurations" [disabled]="busy()" (click)="pullFromGit()">
            <mat-icon>cloud_download</mat-icon>
            <ng-container i18n="@@Import_from_git">Import from git</ng-container>
            <app-ee-lock
              feature="configurations"
              i18n-why="@@Git_ee_why"
              why="Keep your configurations in a git repository, one directory per platform."
            />
          </button>
        </div>
        <input #picker type="file" accept=".yaml,.yml,.json,.zip" (change)="pick($event)" />

        <mat-table [dataSource]="rows()">
          <ng-container matColumnDef="name">
            <mat-header-cell *matHeaderCellDef i18n="@@Name">Name</mat-header-cell>
            <mat-cell *matCellDef="let c">
              <span class="name">
                @if (c.current) {
                  <!-- One icon carries the whole answer, and it is a
                       COMPARISON: green when what runs is byte for byte a
                       saved configuration, amber when it has drifted from the
                       one it was saved as, grey when it was never saved. The
                       name is in the tooltip, where it belongs - a row that
                       shows a name it no longer matches is the lie this
                       replaces. -->
                  <mat-icon
                    [class]="'state ' + state()"
                    [matTooltip]="stateTip()"
                    matTooltipPosition="right"
                    >{{ stateIcon() }}</mat-icon
                  >
                  <span class="label" i18n="@@Current_configuration">Current configuration</span>
                  @if (savedAs(); as name) {
                    <span class="as">{{ name }}</span>
                  }
                } @else {
                  {{ c.name }}
                  @if (c.active) {
                    <!-- The mark says this is what the gateway was set from;
                         whether it still MATCHES is the same comparison the
                         first row makes, so the chip reads it too. A row
                         saying "Current" beside a gateway that has moved on is
                         the same lie in a second place - and the harder one to
                         catch, because the first row is already admitting it. -->
                    <mat-chip
                      disableRipple
                      [highlighted]="state() === 'saved'"
                      [class.stale]="state() !== 'saved'"
                      [matTooltip]="markTip()"
                    >
                      @if (state() === 'saved') {
                        <ng-container i18n="@@Current">Current</ng-container>
                      } @else {
                        <ng-container i18n="@@Current_changed">Current, changed</ng-container>
                      }
                    </mat-chip>
                  }
                }
              </span>
            </mat-cell>
          </ng-container>

          <ng-container matColumnDef="description">
            <mat-header-cell *matHeaderCellDef i18n="@@Description">Description</mat-header-cell>
            <mat-cell *matCellDef="let c">
              <span class="desc">{{ c.description }}</span>
              @if (!c.current && remoteOf(c); as r) {
                <!-- The state in words beside the location's name; where it
                     is exactly, on hover, one part per line. -->
                <span
                  class="git"
                  [class]="syncOf(c)"
                  [matTooltip]="syncTip(c, r)"
                  matTooltipClass="tooltip-lines"
                >
                  <mat-icon>{{ syncIcon(c) }}</mat-icon>
                  {{ r.name }} - {{ syncLabel(c) }}
                </span>
              }
            </mat-cell>
          </ng-container>

          <!-- The row actions live in the LAST existing column: a column of
               nothing but buttons is a column of wasted width. -->
          <ng-container matColumnDef="updated">
            <mat-header-cell *matHeaderCellDef i18n="@@Updated">Updated</mat-header-cell>
            <mat-cell *matCellDef="let c">
              @if (!c.current) {
                <span class="when">{{ c.updatedAt * 1000 | date: 'short' }}</span>
              }
              <span rowActions="tonal">
                @if (c.current) {
                  <button
                    matIconButton
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); saveCurrent()"
                    i18n-matTooltip="@@Save_under_a_name"
                    matTooltip="Save under a name"
                    i18n-aria-label="@@Save_under_a_name"
                    aria-label="Save under a name"
                  >
                    <mat-icon>bookmark_add</mat-icon>
                  </button>
                  <button
                    matIconButton
                    (click)="$event.stopPropagation(); exportCurrent()"
                    i18n-matTooltip="@@Export"
                    matTooltip="Export"
                    i18n-aria-label="@@Export"
                    aria-label="Export"
                  >
                    <mat-icon>download</mat-icon>
                  </button>
                  <!-- The running state goes to git too. A repository holds
                       NAMED configurations, so this names it on the way - the
                       alternative was a download button with no counterpart
                       beside it and an operator wondering where theirs went. -->
                  <button
                    matIconButton
                    ee-feature="configurations"
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); pushCurrent()"
                    i18n-matTooltip="@@Export_to_git"
                    matTooltip="Export to git"
                    i18n-aria-label="@@Export_to_git"
                    aria-label="Export to git"
                  >
                    <mat-icon>cloud_upload</mat-icon>
                  </button>
                } @else {
                  <button
                    matIconButton
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); setAsCurrent(c)"
                    i18n-matTooltip="@@Set_as_current"
                    matTooltip="Set as current"
                    i18n-aria-label="@@Set_as_current"
                    aria-label="Set as current"
                  >
                    <mat-icon>play_circle</mat-icon>
                  </button>
                  <button
                    matIconButton
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); duplicate(c)"
                    i18n-matTooltip="@@Duplicate"
                    matTooltip="Duplicate"
                    i18n-aria-label="@@Duplicate"
                    aria-label="Duplicate"
                  >
                    <mat-icon>content_copy</mat-icon>
                  </button>
                  @if (saved().length > 1) {
                    <button
                      matIconButton
                      (click)="$event.stopPropagation(); compare(c)"
                      i18n-matTooltip="@@Compare_with"
                      matTooltip="Compare with another saved configuration"
                      i18n-aria-label="@@Compare_with"
                      aria-label="Compare with another saved configuration"
                    >
                      <mat-icon>compare_arrows</mat-icon>
                    </button>
                  }
                  <button
                    matIconButton
                    (click)="$event.stopPropagation(); download(c)"
                    i18n-matTooltip="@@Export"
                    matTooltip="Export"
                    i18n-aria-label="@@Export"
                    aria-label="Export"
                  >
                    <mat-icon>download</mat-icon>
                  </button>
                  @if (c.remoteId) {
                    <button
                      matIconButton
                      ee-feature="configurations"
                      [disabled]="busy()"
                      (click)="$event.stopPropagation(); pullInto(c)"
                      i18n-matTooltip="@@Pull_from_git"
                      matTooltip="Pull from git"
                      i18n-aria-label="@@Pull_from_git"
                      aria-label="Pull from git"
                    >
                      <mat-icon>cloud_download</mat-icon>
                    </button>
                  }
                  <button
                    matIconButton
                    ee-feature="configurations"
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); pushToGit(c)"
                    i18n-matTooltip="@@Export_to_git"
                    matTooltip="Export to git"
                    i18n-aria-label="@@Export_to_git"
                    aria-label="Export to git"
                  >
                    <mat-icon>cloud_upload</mat-icon>
                  </button>
                  <button
                    matIconButton
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); remove(c)"
                    i18n-matTooltip="@@Delete"
                    matTooltip="Delete"
                    i18n-aria-label="@@Delete"
                    aria-label="Delete"
                  >
                    <mat-icon>delete</mat-icon>
                  </button>
                }
              </span>
            </mat-cell>
          </ng-container>

          <mat-header-row *matHeaderRowDef="columns"></mat-header-row>
          <mat-row
            *matRowDef="let row; columns: columns"
            [class.current]="row.current"
            (click)="open(row)"
          ></mat-row>
        </mat-table>
      </mat-drawer-content>

      <mat-drawer position="end" mode="over" [opened]="opened() !== null" (closedStart)="close()">
        @if (opened(); as row) {
          <app-configuration-yaml
            [title]="row.current ? (savedAs() || currentLabel()) : row.name"
            [subtitle]="row.current ? currentSubtitle() : savedSubtitle()"
            [note]="mediaNote()"
            [text]="document()"
            [canEdit]="true"
            [saveLabel]="row.current ? applyLabel() : saveLabel()"
            [busy]="busy()"
            (saved)="saveDocument(row, $event)"
            (download)="row.current ? exportCurrent() : download(row)"
            (closed)="close()"
          />
        }
      </mat-drawer>
    </mat-drawer-container>
  `,
})
export class ConfigurationManagementComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly saved = signal<SavedConfiguration[]>([]);
  protected readonly edition = httpResource<Edition>(() => '/api/edition');
  // 0 (Enterprise) reads as no limit.
  protected readonly cap = computed(() => this.edition.value()?.configurationCap ?? 0);
  protected readonly current = signal<CurrentConfiguration | null>(null);
  protected readonly busy = signal(false);
  protected readonly columns = ['name', 'description', 'updated'];
  protected readonly document = signal('');
  // The git locations, so a row can name where it lives without a call per row.
  protected readonly remotes = signal<ConfigRemote[]>([]);

  // The current configuration IS a row: the one in force.
  protected readonly rows = computed<Row[]>(() => {
    const active = this.saved().find((c) => c.active);
    const current: Row = {
      id: 'current',
      name: active?.name ?? '',
      description: active?.description ?? '',
      active: true,
      current: true,
      createdAt: 0,
      updatedAt: active?.updatedAt ?? 0,
    };
    return [current, ...this.saved()];
  });

  // Three states, and only one of them is a claim the screen can keep making:
  // saved means the fingerprints match RIGHT NOW.
  protected readonly state = computed<'saved' | 'drifted' | 'unsaved'>(() => {
    const c = this.current();
    if (!c) return 'unsaved';
    if (c.savedAs) return 'saved';
    return c.active ? 'drifted' : 'unsaved';
  });

  protected readonly savedAs = computed(() => this.current()?.savedAs?.name ?? '');

  protected readonly stateIcon = computed(() => {
    switch (this.state()) {
      case 'saved':
        return 'cloud_done';
      case 'drifted':
        return 'cloud_alert';
      default:
        return 'cloud_off';
    }
  });

  // What the mark means, said from the row that holds it. Same comparison as
  // the icon above, so the two can never disagree.
  protected readonly markTip = computed(() =>
    this.state() === 'saved'
      ? $localize`:@@Mark_matches:This is what the gateway serves.`
      : $localize`:@@Mark_stale:The gateway has changed since it was set from this one. Save the current state again to match.`,
  );

  protected readonly stateTip = computed(() => {
    const c = this.current();
    switch (this.state()) {
      case 'saved':
        return $localize`:@@Saved_as_WHAT:Saved as ${c?.savedAs?.name}:name:`;
      case 'drifted':
        return $localize`:@@Drifted_from_WHAT:Changed since ${c?.active?.name}:name: was saved. Save it again to keep these changes.`;
      default:
        return $localize`:@@Not_saved_anywhere:Not saved under any name.`;
    }
  });

  // The drawer is URL-driven (F5-proof), like every other detail view in this
  // console: /infra/configuration/management/<id>, or .../current.
  private readonly openId = toSignal(
    this.route.paramMap.pipe(map((p) => p.get('id'))),
    { initialValue: null },
  );
  protected readonly opened = computed(() => {
    const id = this.openId();
    if (!id) return null;
    return this.rows().find((r) => r.id === id) ?? null;
  });

  protected readonly currentLabel = () => $localize`:@@Current_configuration:Current configuration`;
  protected readonly currentSubtitle = () =>
    $localize`:@@Current_subtitle:What this gateway serves. Saving applies your edits.`;
  protected readonly savedSubtitle = () =>
    $localize`:@@Saved_subtitle:A saved copy. Editing it does not change what is served.`;
  protected readonly mediaNote = () =>
    $localize`:@@Media_not_edited_here:Images are left out here and kept as they are. Set them in Built-in pages, Branding.`;
  protected readonly saveLabel = () => $localize`:@@Save:Save`;
  protected readonly applyLabel = () => $localize`:@@Save_and_apply:Save and apply`;

  constructor() {
    this.reload();
    // Somebody else's write (CONSOLE-13): a configuration saved, switched,
    // imported or restored anywhere shows up here without a click.
    inject(LiveChangesService).on('configuration', () => this.reload());
    inject(LiveChangesService).on('config', () => this.reload());
    // The locations live on the page above this tab now, so a deletion there
    // (which unbinds configurations here) arrives the same way anybody else's
    // write does.
    inject(LiveChangesService).on('config-remote', () => this.reload());
    // The document follows the URL, not the click: opening the drawer from a
    // pasted link (or reloading on one) must load what clicking the row loads.
    // Fetching it in the click handler alone left a deep link showing an EMPTY
    // editor - which reads as a configuration with nothing in it.
    effect(() => {
      const row = this.opened();
      untracked(() => this.load(row));
    });
  }

  protected remoteOf(c: SavedConfiguration): ConfigRemote | undefined {
    return c.remoteId ? this.remotes().find((r) => r.id === c.remoteId) : undefined;
  }

  // Where a row stands against its repository, and it is a COMPARISON like
  // every other state on this screen - never a flag recorded once.
  //
  // What is knowable without reaching out is this: a row that has never met its
  // location (nothing pushed yet), and a row whose document has changed since it
  // last did (so the repository is behind). Whether somebody ELSE has pushed
  // since cannot be known from here - finding out is a pull, and a screen that
  // claimed to know would be guessing. So "level" means level as far as this
  // gateway last looked, and the tooltip says so.
  protected syncOf(c: SavedConfiguration): 'new' | 'ahead' | 'level' {
    if (!c.remoteRev) return 'new';
    return (c.remoteAt ?? 0) >= (c.updatedAt ?? 0) ? 'level' : 'ahead';
  }

  protected syncIcon(c: SavedConfiguration): string {
    switch (this.syncOf(c)) {
      case 'level':
        return 'cloud_done';
      case 'ahead':
        return 'cloud_upload';
      default:
        return 'cloud_off';
    }
  }

  protected syncLabel(c: SavedConfiguration): string {
    switch (this.syncOf(c)) {
      case 'level':
        return $localize`:@@Git_up_to_date:up to date`;
      case 'ahead':
        return $localize`:@@Git_changed_since_push:changed since push`;
      default:
        return $localize`:@@Git_never_pushed:never pushed`;
    }
  }

  // Where it is, one part per line - and, when it reads up to date, what
  // that can and cannot know: whether somebody else pushed since is a pull.
  protected syncTip(c: SavedConfiguration, r: ConfigRemote): string {
    const where = [r.url.replace(/^https?:\/\//, '').replace(/\.git$/, ''), $localize`:@@Git_branch_line:branch ${r.branch}:branch:`];
    if (r.dir) where.push($localize`:@@Git_dir_line:directory ${r.dir}:dir:`);
    if (this.syncOf(c) === 'level') {
      where.push('', $localize`:@@Git_level_note:As of this gateway's last push or pull: pull to see whether anyone else has changed it since.`);
    }
    return where.join('\n');
  }

  // Guarded by the id: the list is re-read after every change, so `opened()`
  // hands back a new object for the same row, and without this the effect
  // would re-fetch over a document being edited.
  private loadedId = '';

  private load(row: Row | null): void {
    if (!row) {
      this.loadedId = '';
      this.document.set('');
      return;
    }
    if (this.loadedId === row.id) return;
    this.loadedId = row.id;
    this.document.set('');
    // The DOCUMENT, not the export: same file, pictures taken out. A logo is a
    // megabyte of base64 on one line, and what nobody can read, nobody edits
    // safely. Media are set on the Branding screen; an edited document that
    // carries none keeps the ones in place.
    const source = row.current ? this.api.currentDocument() : this.api.configurationDocument(row.id);
    source.subscribe({
      next: (text) => this.document.set(text),
      error: (err: unknown) => this.fail(err),
    });
  }

  protected reload(): void {
    this.api.configurations().subscribe({
      next: (list) => this.saved.set(list),
      error: () => this.saved.set([]),
    });
    // Re-read on every change, because it is a comparison and not a flag: an
    // import, a switch, or a save all move one side of it.
    this.api.currentConfiguration().subscribe({
      next: (c) => this.current.set(c),
      error: () => this.current.set(null),
    });
    // The locations, so a row can say where it lives. A 403 is the community
    // image answering, and then no row has one - the buttons are inert anyway.
    this.api.configRemotes().subscribe({
      next: (list) => this.remotes.set(list),
      error: () => this.remotes.set([]),
    });
  }

  // Opening a row is a NAVIGATION and nothing else: the URL says which document
  // is open and the effect above loads it. One path for a click and for a
  // pasted link.
  protected open(row: Row): void {
    void this.router.navigate(['/infra/configuration/management', row.id]);
  }

  protected close(): void {
    void this.router.navigate(['/infra/configuration/management']);
  }

  // Saving from the drawer. On a saved configuration it replaces a copy and
  // applies nothing; on the CURRENT one it is an import of what was typed, so
  // it goes through the plan first - editing what a gateway serves is not
  // something to discover afterwards.
  protected saveDocument(row: Row, text: string): void {
    const file = new Blob([text], { type: 'application/yaml' });
    if (!row.current) {
      this.run(this.api.replaceConfigurationDocument(row.id, file), $localize`:@@Saved:Saved`);
      return;
    }
    this.busy.set(true);
    this.api.previewConfig(file, true).subscribe({
      next: async (plan) => {
        this.busy.set(false);
        const ok = await this.dialogs.confirm({
          title: $localize`:@@Apply_this_document:Apply this document?`,
          message: this.planMessage(plan),
          confirmLabel: $localize`:@@Save_and_apply:Save and apply`,
          danger: this.removes(plan) > 0,
        });
        if (!ok) return;
        this.run(
          this.api.importConfig(file, true),
          $localize`:@@Configuration_imported:Configuration imported`,
        );
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Save the running state under a name. An existing name REPLACES what it
  // held - "save it again" is the commonest thing an operator wants here - so
  // the collision is a confirmation, not a refusal.
  protected async saveCurrent(): Promise<void> {
    const name = await this.dialogs.prompt({
      title: $localize`:@@Save_under_a_name:Save under a name`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Save:Save`,
      initial: this.rows()[0].name,
    });
    if (!name) return;
    const existing = this.byName(name);
    if (existing) {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Replace_configuration:Replace ${name}:name:?`,
        message: $localize`:@@Replace_configuration_message:Its content is replaced by what this gateway serves now.`,
        confirmLabel: $localize`:@@Replace:Replace`,
        danger: true,
      });
      if (!ok) return;
      this.run(this.api.recaptureConfiguration(existing.id), $localize`:@@Saved:Saved`);
      return;
    }
    this.run(this.api.captureConfiguration(name), $localize`:@@Configuration_saved:Configuration saved`);
  }

  // Exporting says what leaves before it leaves - what the file carries, which
  // secrets stay behind because they are stored as values here, which vault
  // names it expects, and the choice of form. That was a whole tab; it is a
  // sentence, and its moment is this click.
  protected async exportCurrent(): Promise<void> {
    const choice = await firstValueFrom(
      this.dialog
        .open(ExportDialogComponent, {
          data: { name: this.savedAs() || this.currentLabel() },
          width: '640px',
        })
        .afterClosed(),
    );
    if (!choice) return;
    if (choice.format === 'zip') {
      this.api.exportConfigBundle().subscribe({
        next: (blob) => this.offerBlob(blob, 'meerkat-config.zip'),
        error: (err: unknown) => this.fail(err),
      });
      return;
    }
    this.api.exportConfig().subscribe({
      next: (text) => this.offerFile(text, 'meerkat-config.yaml'),
      error: (err: unknown) => this.fail(err),
    });
  }

  // One file, three destinations, and they are three different acts: onto a
  // NAME it is stored and applied to nothing; REPLACING the current one makes
  // this gateway that file; ADDING it is a merge, for a file that holds a part.
  // The third is what the old Import tab expressed as a pruning checkbox, and
  // it must not be lost with the tab.
  protected async pick(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    const outcome = await firstValueFrom(
      this.dialog
        .open(ImportDialogComponent, {
          data: {
            filename: file.name,
            file,
            suggested: file.name.replace(/\.(ya?ml|json|zip)$/i, ''),
          },
          width: '640px',
        })
        .afterClosed(),
    );
    if (!outcome) return;

    if (outcome.where === 'name') {
      const existing = this.byName(outcome.name);
      if (existing) {
        const ok = await this.dialogs.confirm({
          title: $localize`:@@Replace_configuration:Replace ${outcome.name}:name:?`,
          message: $localize`:@@Replace_configuration_file_message:Its content is replaced by this file. Nothing is applied.`,
          confirmLabel: $localize`:@@Replace:Replace`,
          danger: true,
        });
        if (!ok) return;
        this.run(this.api.replaceConfigurationDocument(existing.id, file), $localize`:@@Saved:Saved`);
        return;
      }
      this.run(
        this.api.storeConfigurationFile(outcome.name, file),
        $localize`:@@Configuration_saved:Configuration saved`,
      );
      return;
    }

    // Applied: the plan was shown in the window that just closed, so this is
    // the act itself. What it reserves empty in the vault is asked for
    // straight away - the one moment anyone still knows what a $name was.
    this.busy.set(true);
    this.api.importConfig(file, outcome.where === 'replace').subscribe({
      next: (plan) => {
        this.busy.set(false);
        this.reload();
        this.snack.open($localize`:@@Configuration_imported:Configuration imported`, undefined, {
          duration: 3000,
        });
        if (plan.missing?.length) {
          this.dialog.open(VaultHolesDialogComponent, {
            data: { entries: plan.missing },
            width: '640px',
          });
        }
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // The one act that touches what is served. Two questions, in the order they
  // matter: is there anything to lose (a running state nobody saved), and what
  // exactly would change.
  protected async setAsCurrent(c: SavedConfiguration): Promise<void> {
    // The question is not "is there a mark" but "does anything hold what is
    // running": a gateway that drifted from the configuration it was saved as
    // has just as much to lose as one that was never saved.
    if (this.state() !== 'saved') {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Nothing_saved_first:What is running is not saved`,
        message:
          this.state() === 'drifted'
            ? $localize`:@@Drifted_first_message:This gateway has changed since ${this.current()?.active?.name}:name: was saved. Switching loses those changes.`
            : $localize`:@@Nothing_saved_first_message:No saved configuration matches what this gateway serves. Save it under a name first if you may want it again.`,
        confirmLabel: $localize`:@@Continue:Continue`,
        danger: true,
      });
      if (!ok) return;
    }
    this.busy.set(true);
    this.api.configurationPlan(c.id).subscribe({
      next: async (plan) => {
        this.busy.set(false);
        const ok = await this.dialogs.confirm({
          title: $localize`:@@Set_as_current_WHAT:Serve ${c.name}:name:?`,
          message: this.planMessage(plan),
          confirmLabel: $localize`:@@Set_as_current:Set as current`,
          danger: this.removes(plan) > 0,
        });
        if (!ok) return;
        this.run(
          this.api.activateConfiguration(c.id),
          $localize`:@@Configuration_activated:Configuration activated`,
        );
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Two saved configurations against each other, what runs left out (CFG-04).
  protected compare(c: SavedConfiguration): void {
    this.dialog.open(CompareDialogComponent, {
      data: { from: c, others: this.saved().filter((o) => o.id !== c.id) },
      width: '680px',
    });
  }

  protected async duplicate(c: SavedConfiguration): Promise<void> {
    const name = await this.dialogs.prompt({
      title: $localize`:@@Duplicate:Duplicate`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Duplicate:Duplicate`,
      initial: c.name + ' (copy)',
    });
    if (!name) return;
    if (this.byName(name)) {
      this.snack.open(
        $localize`:@@Name_already_taken:A configuration called ${name}:name: already exists`,
        undefined,
        { duration: 4000 },
      );
      return;
    }
    this.run(this.api.duplicateConfiguration(c.id, name), $localize`:@@Duplicated:Duplicated`);
  }

  protected async remove(c: SavedConfiguration): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_configuration:Delete ${c.name}:name:?`,
      message: c.active
        ? $localize`:@@Delete_active_configuration_message:This is the current one. What the gateway serves does not change, it just loses its name.`
        : $localize`:@@Delete_configuration_message:The saved copy is gone. What the gateway serves does not change.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.run(this.api.deleteConfiguration(c.id), $localize`:@@Deleted:Deleted`);
  }

  protected async download(c: SavedConfiguration): Promise<void> {
    const choice = await firstValueFrom(
      this.dialog
        .open(ExportDialogComponent, { data: { name: c.name, id: c.id }, width: '640px' })
        .afterClosed(),
    );
    if (!choice) return;
    if (choice.format === 'zip') {
      this.api.exportConfigurationBundle(c.id).subscribe({
        next: (blob) => this.offerBlob(blob, `meerkat-${this.slug(c.name)}.zip`),
        error: (err: unknown) => this.fail(err),
      });
      return;
    }
    this.api.exportConfiguration(c.id).subscribe({
      next: (text) => this.offerFile(text, `meerkat-${this.slug(c.name)}.yaml`),
      error: (err: unknown) => this.fail(err),
    });
  }

  // ── git (CFG-07) ───────────────────────────────────────────────────────────

  // Reading a location into a NEW saved configuration. The same act as
  // importing a file: it shelves, and applies nothing.
  protected async pullFromGit(): Promise<void> {
    const choice = await firstValueFrom(
      this.dialog
        .open(PickLocationDialogComponent, {
          data: { way: 'pull', taken: this.saved().map((c) => c.name) },
          width: '600px',
        })
        .afterClosed(),
    );
    if (!choice?.remoteId) return;
    this.busy.set(true);
    this.api.pullNewConfiguration(choice.remoteId, choice.name).subscribe({
      next: (result) => {
        this.busy.set(false);
        this.reload();
        this.showPull(result, choice.remoteId);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Refreshing a bound row from its own location. No dialog: the destination
  // was decided when it was bound, and asking again is how a pull ends up
  // reading the wrong platform.
  protected pullInto(c: SavedConfiguration): void {
    this.busy.set(true);
    this.api.pullConfiguration(c.id).subscribe({
      next: (result) => {
        this.busy.set(false);
        this.reload();
        this.showPull(result, c.remoteId ?? '');
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  private showPull(result: { configuration: SavedConfiguration }, remoteId: string): void {
    const r = this.remotes().find((o) => o.id === remoteId);
    this.dialog.open(PullResultDialogComponent, {
      data: { result, location: r?.name ?? '' },
      width: '680px',
    });
  }

  // Pushing WHAT IS RUNNING. A repository holds named configurations, not a
  // running state, so this names it first: when a saved copy already matches
  // byte for byte, that one is pushed and nothing is asked; otherwise the name
  // is asked for and the state is saved under it on the way.
  protected async pushCurrent(): Promise<void> {
    let row = this.savedMatchingCurrent();
    if (!row) {
      const name = await this.dialogs.prompt({
        title: $localize`:@@Export_to_git:Export to git`,
        label: $localize`:@@Name:Name`,
        confirmLabel: $localize`:@@Save_and_push:Save and push`,
        initial: this.current()?.active?.name ?? '',
      });
      if (!name) return;
      const existing = this.byName(name);
      this.busy.set(true);
      try {
        row = await firstValueFrom(
          existing
            ? this.api.recaptureConfiguration(existing.id)
            : this.api.captureConfiguration(name),
        );
      } catch (err) {
        this.busy.set(false);
        this.fail(err);
        return;
      }
      this.busy.set(false);
      this.reload();
    }
    await this.pushToGit(row);
  }

  // The saved configuration that IS what the gateway runs, when there is one.
  // The digests are compared server-side, so this is a fact rather than a mark
  // somebody set once.
  private savedMatchingCurrent(): SavedConfiguration | undefined {
    const id = this.current()?.savedAs?.id;
    return id ? this.saved().find((c) => c.id === id) : undefined;
  }

  // Committing a saved copy to its location. The destination is asked for ONCE -
  // the first time, or when it changes - and remembered on the row after that.
  protected async pushToGit(c: SavedConfiguration): Promise<void> {
    let remoteId = c.remoteId ?? '';
    if (!remoteId) {
      const choice = await firstValueFrom(
        this.dialog
          .open(PickLocationDialogComponent, {
            data: { way: 'push', configuration: c.name, selected: remoteId },
            width: '600px',
          })
          .afterClosed(),
      );
      if (!choice?.remoteId) return;
      remoteId = choice.remoteId;
      this.busy.set(true);
      try {
        await firstValueFrom(this.api.bindConfiguration(c.id, remoteId));
      } catch (err) {
        this.busy.set(false);
        this.fail(err);
        return;
      }
      this.busy.set(false);
    }
    this.busy.set(true);
    this.api.pushConfiguration(c.id).subscribe({
      next: () => {
        this.busy.set(false);
        this.reload();
        this.snack.open($localize`:@@Pushed_to_git:Pushed`, undefined, { duration: 2500 });
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  private byName(name: string): SavedConfiguration | undefined {
    const wanted = name.trim().toLocaleLowerCase();
    return this.saved().find((c) => c.name.toLocaleLowerCase() === wanted);
  }

  // Built here rather than pointed at the URL: the console talks to the admin
  // port with a session cookie, and a plain link would leave the application
  // to fetch it.
  private offerFile(text: string, filename: string): void {
    const url = URL.createObjectURL(new Blob([text], { type: 'application/yaml' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  private offerBlob(blob: Blob, filename: string): void {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  private slug(name: string): string {
    return name
      .toLocaleLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '');
  }

  // "3 added, 2 updated, 1 removed" - and the removals spelled out, because
  // they are the ones nobody expects. Setting a configuration as current
  // prunes: that is what makes it a switch and not a merge.
  private planMessage(plan: ConfigPlan): string {
    const count = (action: string) => plan.changes.filter((c) => c.action === action).length;
    const added = count('add');
    const updated = count('update');
    const removed = this.removes(plan);
    if (!added && !updated && !removed) {
      return $localize`:@@Activate_no_change:This gateway already matches this configuration. Nothing changes.`;
    }
    const gone = plan.changes
      .filter((c) => c.action === 'remove')
      .map((c) => c.label)
      .join(', ');
    const head = $localize`:@@Activate_counts:${added}:added: added, ${updated}:updated: updated, ${removed}:removed: removed.`;
    return removed ? head + ' ' + $localize`:@@Activate_removed_list:What goes: ${gone}:gone:.` : head;
  }

  private removes(plan: ConfigPlan): number {
    return plan.changes.filter((c) => c.action === 'remove').length;
  }

  // Every mutation ends the same way: unlock, reload the list, say so. The list
  // is re-read rather than patched in place because setting one as current
  // moves the mark off ANOTHER row, and a local patch would leave two lit.
  private run(call: Observable<unknown>, done: string): void {
    this.busy.set(true);
    call.subscribe({
      next: () => {
        this.busy.set(false);
        this.reload();
        this.snack.open(done, undefined, { duration: 2500 });
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      undefined,
      { duration: 6000 },
    );
  }
}
