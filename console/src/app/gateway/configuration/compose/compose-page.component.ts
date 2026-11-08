import { Component, DestroyRef, ElementRef, afterRenderEffect, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router } from '@angular/router';
import { MatDialog } from '@angular/material/dialog';
import { Subject, catchError, debounceTime, firstValueFrom, of, switchMap } from 'rxjs';
import { ApiService, ComposeAnswer, ComposeBody, ComposeSource, ConfigRemote, SavedConfiguration } from '../../../api.service';
import { MeService } from '../../../me.service';
import { DialogsService } from '../../../shared/dialogs.service';
import { LiveChangesService } from '../../../shared/live-changes.service';
import { FileDropDirective, pickFiles } from '../../../shared/file-pick';
import { ComposeBlockComponent, Media, PickEvent } from './compose-block.component';
import { Entity, KINDS, Recipe, entities } from './compose.model';

// One configuration composed from two (CFG-08): the loom.
//
// Three columns. The left is where the result starts (what runs, by
// default), the right is what it is compared with - a saved configuration, a
// git location, a file picked or dropped on the page - and the result stands
// between them. Every difference is a block to decide: a choice between the
// two sides, on a radio, object by object, value by value, element by
// element. The server composes at each click (one implementation, the one
// that applies); this page reads its answer and keeps the recipe.
//
// What the result would silently lack is said out loud, above the loom and
// counted in the result's header: a vault entry the document names and this
// gateway does not hold, a spec or files a plain file never carries, the
// Enterprise parts a community image leaves out, a reference nobody declares.
//
// Nothing here polls: the page asks the gateway when the operator clicks.
type SourceOption = { value: string; label: string; src: ComposeSource };

// What a side reads as a file: a document, or a package with its assets.
const FILES = '.yaml,.yml,.json,.zip';

function base64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(String(r.result).split(',')[1] ?? '');
    r.onerror = () => reject(r.error);
    r.readAsDataURL(file);
  });
}

@Component({
  selector: 'app-compose-page',
  imports: [
    ComposeBlockComponent,
    FileDropDirective,
    MatButtonModule,
    MatChipsModule,
    MatIconModule,
    MatMenuModule,
    MatProgressBarModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  styles: [
    `
      /* The tab panel above is the one that scrolls (the page has no height
         of its own in it): the loom does not scroll, so the headers stick to
         the panel, and the ribbons are drawn in the loom's own coordinates,
         which do not move with the scroll. */
      :host {
        display: block;
        /* Three hues, one per column: blue for what runs (the primary),
           green for the result, orange for the other side. */
        --mk-left: var(--mat-sys-primary);
        --mk-center: #2e6b3a;
        --mk-right: #9a4a00;
        --mk-mono: 'Roboto Mono', ui-monospace, Menlo, monospace;
      }
      :host-context(.dark) {
        --mk-center: #8fd39b;
        --mk-right: #ffb787;
      }
      .filters {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
        padding: 4px 0 8px;
      }
      .filters .grow {
        flex: 1;
      }
      .progress {
        height: 4px;
        margin: 0 16px 6px;
      }
      /* What the result would lack, said before anything is applied. */
      .alerts {
        margin: 0 16px 10px;
        padding: 10px 14px;
        border-radius: 12px;
        border: 1px solid var(--mat-sys-error);
        background: color-mix(in srgb, var(--mat-sys-error) 8%, var(--mat-sys-surface));
        display: flex;
        align-items: flex-start;
        gap: 12px;
        font-size: 0.8rem;
      }
      .alerts mat-icon {
        color: var(--mat-sys-error);
        flex: none;
      }
      .alerts .lines {
        display: flex;
        flex-direction: column;
        gap: 4px;
        min-width: 0;
      }
      /* 16px on each side: a block's frame overflows the columns by that
         much, and the tab panel above clips what leaves its box. */
      .loom {
        position: relative;
        padding: 0 16px 48px;
        border-radius: 12px;
        outline: 2px dashed transparent;
        outline-offset: -2px;
        transition: outline-color 0.12s;
      }
      .loom.dragging {
        outline-color: var(--mk-right);
      }
      .loom svg.ribbons {
        position: absolute;
        top: 0;
        left: 0;
        pointer-events: none;
        overflow: visible;
      }
      .grid {
        position: relative;
        display: grid;
        grid-template-columns: 1fr 1fr 1fr;
        column-gap: 56px;
        row-gap: 10px;
      }
      .colband {
        position: absolute;
        top: 0;
        bottom: -16px;
        z-index: 0;
        border-radius: 12px;
        pointer-events: none;
      }
      .colband.left {
        background: color-mix(in srgb, var(--mk-left) 12%, transparent);
      }
      .colband.right {
        background: color-mix(in srgb, var(--mk-right) 12%, transparent);
      }
      .colband.center {
        background: color-mix(in srgb, var(--mk-center) 12%, transparent);
      }
      /* The three headers in one sticky, opaque band across the columns. */
      .colheads {
        grid-column: 1 / -1;
        position: sticky;
        top: 0;
        z-index: 3;
        display: grid;
        grid-template-columns: 1fr 1fr 1fr;
        column-gap: 56px;
        padding-top: 8px;
        background: var(--mat-sys-surface);
        box-shadow:
          -16px 0 0 var(--mat-sys-surface),
          16px 0 0 var(--mat-sys-surface),
          0 1px 0 var(--mat-sys-outline-variant);
      }
      /* Material's own margins: 16px to the edges, 8px between actions. */
      .colhead {
        display: flex;
        align-items: center;
        gap: 12px;
        min-height: 48px;
        padding: 4px 16px;
        font-size: 0.75rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        color: var(--mat-sys-on-surface-variant);
        border-radius: 8px 8px 0 0;
      }
      .colhead.left {
        border-bottom: 2px solid var(--mk-left);
        background: color-mix(in srgb, var(--mk-left) 14%, var(--mat-sys-surface));
      }
      .colhead.right {
        border-bottom: 2px solid var(--mk-right);
        background: color-mix(in srgb, var(--mk-right) 14%, var(--mat-sys-surface));
      }
      .colhead.center {
        border-bottom: 2px solid var(--mk-center);
        background: color-mix(in srgb, var(--mk-center) 14%, var(--mat-sys-surface));
      }
      .colhead .actions {
        margin-left: auto;
        display: inline-flex;
        align-items: center;
        gap: 8px;
        text-transform: none;
        letter-spacing: 0;
        white-space: nowrap;
      }
      /* Three columns of actions on one line: the buttons go dense. */
      .colhead .actions button {
        --mdc-text-button-container-height: 32px;
        --mdc-filled-button-container-height: 32px;
        --mdc-outlined-button-container-height: 32px;
        --mat-text-button-horizontal-padding: 10px;
        --mat-filled-button-horizontal-padding: 12px;
        --mat-outlined-button-horizontal-padding: 10px;
        font-size: 0.75rem;
      }
      .colhead .srcsel {
        flex: 1;
        min-width: 0;
        text-transform: none;
        letter-spacing: 0;
        font-weight: 500;
      }
      .colhead .warnings {
        display: inline-flex;
        align-items: center;
        gap: 2px;
        color: var(--mat-sys-error);
        font-weight: 600;
        cursor: default;
      }
      .colhead .warnings mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      .section {
        grid-column: 1 / -1;
        margin-top: 14px;
        padding-left: 8px;
        font-weight: 600;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
        display: flex;
        align-items: center;
        gap: 8px;
        position: relative;
        z-index: 1;
      }
      .section::after {
        content: '';
        flex: 1;
        height: 1px;
        background: var(--mat-sys-outline-variant);
      }
      .section .n {
        font-weight: 400;
      }
      .cap {
        margin-left: 12px;
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .cap.full {
        color: var(--mat-sys-error);
      }
      /* Nothing to compose: no bands either, the headers alone and one line
         under them with its mark. */
      .loom.bare .colband {
        display: none;
      }
      .empty {
        grid-column: 1 / -1;
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 8px;
        padding: 56px 16px;
        text-align: center;
        color: var(--mat-sys-on-surface-variant);
      }
      .empty mat-icon {
        font-size: 40px;
        width: 40px;
        height: 40px;
        color: var(--mk-center);
      }
      /* The apply item's icon turns red while the result removes something. */
      .danger-icon {
        color: var(--mat-sys-error);
      }
    `,
  ],
  template: `
    <div class="filters">
      <mat-chip-listbox aria-label="Which rows" i18n-aria-label="@@Compose_which_rows">
        <mat-chip-option [selected]="onlyDiff()" (selectionChange)="setOnlyDiff($event.selected)" i18n="@@Only_differences">
          Only differences
        </mat-chip-option>
      </mat-chip-listbox>
      <mat-chip-listbox multiple aria-label="Which kinds" i18n-aria-label="@@Compose_which_kinds">
        @for (k of kindCounts(); track k.kind) {
          <mat-chip-option [selected]="kinds().has(k.kind)" (selectionChange)="toggleKind(k.kind, $event.selected)">
            {{ k.label }} <span class="n">{{ k.n }}</span>
          </mat-chip-option>
        }
      </mat-chip-listbox>
      <span class="grow"></span>
    </div>
    <!-- Its place is reserved whether it shows or not: a bar that appears
         pushes the whole loom down by its height, and back up when it goes. -->
    <div class="progress">
      <mat-progress-bar mode="indeterminate" [style.visibility]="busy() ? 'visible' : 'hidden'" />
    </div>
    @if (alerts().length) {
      <div class="alerts" role="alert">
        <mat-icon>warning</mat-icon>
        <div class="lines">
          @for (a of alerts(); track a) {
            <div class="line">{{ a }}</div>
          }
        </div>
      </div>
    }
    <div class="loom" [class.bare]="answer() && !groups().length" appFileDrop [accept]="accepted" (filesDropped)="dropped($event)">
      <svg class="ribbons"></svg>
      <div class="grid">
        <div class="colband left"></div>
        <div class="colband center"></div>
        <div class="colband right"></div>
        <div class="colheads">
          <div class="colhead left">
            <mat-select class="srcsel" [value]="leftKey()" (selectionChange)="chooseSource('left', $event.value)" aria-label="Left side" i18n-aria-label="@@Compose_left_side">
              @for (o of options('left'); track o.value) {
                <mat-option [value]="o.value">{{ o.label }}</mat-option>
              }
              <mat-option value="file" i18n="@@Compose_a_file">A file...</mat-option>
            </mat-select>
            <!-- Always there; its entries live only when the side is a saved
                 configuration, the one thing they act on. -->
            <button matIconButton [matMenuTriggerFor]="moreLeft" matTooltip="More" i18n-matTooltip="@@More" aria-label="More" i18n-aria-label="@@More">
              <mat-icon>more_vert</mat-icon>
            </button>
            <mat-menu #moreLeft="matMenu">
              <button mat-menu-item [disabled]="!sideFile('left')" (click)="readSide('left')"><mat-icon>description</mat-icon><span i18n="@@Read_the_file">Read the file</span></button>
              <button mat-menu-item [disabled]="!sideSaved('left')" (click)="renameSide(sideSaved('left')!)"><mat-icon>edit</mat-icon><span i18n="@@Rename">Rename</span></button>
              <button mat-menu-item [disabled]="!sideSaved('left')" (click)="deleteSide('left', sideSaved('left')!)"><mat-icon>delete</mat-icon><span i18n="@@Delete">Delete</span></button>
            </mat-menu>
            <span class="actions">
              <button matIconButton [disabled]="!undecided()" (click)="chooseAll('left')" matTooltip="Every difference still to decide: choose this side" i18n-matTooltip="@@Choose_all_tip" aria-label="Choose all from this side" i18n-aria-label="@@Choose_all_aria">
                <mat-icon>done_all</mat-icon>
              </button>
            </span>
          </div>
          <div class="colhead center">
            <span i18n="@@Result">Result</span>
            @if (alerts().length) {
              <span class="warnings" [matTooltip]="alerts().join('\n')" matTooltipClass="tooltip-lines">
                <mat-icon>warning</mat-icon>{{ alerts().length }}
              </span>
            }
            <span class="actions">
              <button matIconButton [disabled]="!touched()" (click)="reset()" matTooltip="Forget every choice" i18n-matTooltip="@@Reset_tip" aria-label="Forget every choice" i18n-aria-label="@@Reset_tip">
                <mat-icon>reset_settings</mat-icon>
              </button>
              <button matButton="filled" [matMenuTriggerFor]="saveMenu" [disabled]="busy() || !answer()">
                <ng-container i18n="@@Save">Save</ng-container>
                <mat-icon iconPositionEnd>arrow_drop_down</mat-icon>
              </button>
              <mat-menu #saveMenu="matMenu">
                <!-- Where the result goes: this gateway first, then the shelf,
                     a file, the git location read on the right. -->
                <button
                  mat-menu-item
                  [disabled]="!answer()?.plan || !!blocked()"
                  [matTooltip]="blocked() || ''"
                  (click)="apply()"
                >
                  <mat-icon [class.danger-icon]="removes() > 0">play_circle</mat-icon>
                  <span i18n="@@Apply_as_current">Apply as current</span>
                </button>
                <button mat-menu-item (click)="saveOnShelf()">
                  <mat-icon>inventory_2</mat-icon>
                  <span i18n="@@On_the_shelf">On the shelf</span>
                  @if (cap()) {
                    <span class="cap" [class.full]="full()" i18n="@@Configurations_shelf">{{ saved().length }} of {{ cap() }} saved</span>
                  }
                </button>
                <button mat-menu-item (click)="saveOnFilesystem()">
                  <mat-icon>download</mat-icon>
                  <span i18n="@@On_the_filesystem">On the filesystem</span>
                </button>
                <!-- A push goes to the location read on the right, and nowhere
                     else: what is pushed replaces the directory there, and
                     nobody replaces what they have not looked at. -->
                @if (rightRemote(); as r) {
                  <button mat-menu-item (click)="pushTo(r)">
                    <mat-icon>cloud_upload</mat-icon>
                    <span i18n="@@Push_to_NAME">Push to {{ r.name }}</span>
                  </button>
                } @else if (remotes().length) {
                  <button mat-menu-item disabled>
                    <mat-icon>cloud_upload</mat-icon>
                    <span i18n="@@Push_load_right_first">To push to a git location, load it on the right first</span>
                  </button>
                }
              </mat-menu>
            </span>
          </div>
          <div class="colhead right">
            <mat-select class="srcsel" [value]="rightKey()" (selectionChange)="chooseSource('right', $event.value)" aria-label="Right side" i18n-aria-label="@@Compose_right_side">
              @for (o of options('right'); track o.value) {
                <mat-option [value]="o.value">{{ o.label }}</mat-option>
              }
              <mat-option value="file" i18n="@@Compose_a_file_or_drop">A file... (or drop it on the page)</mat-option>
            </mat-select>
            <!-- Always there; its entries live only when the side is a saved
                 configuration, the one thing they act on. -->
            <button matIconButton [matMenuTriggerFor]="moreRight" matTooltip="More" i18n-matTooltip="@@More" aria-label="More" i18n-aria-label="@@More">
              <mat-icon>more_vert</mat-icon>
            </button>
            <mat-menu #moreRight="matMenu">
              <button mat-menu-item [disabled]="!sideFile('right')" (click)="readSide('right')"><mat-icon>description</mat-icon><span i18n="@@Read_the_file">Read the file</span></button>
              <button mat-menu-item [disabled]="!sideSaved('right')" (click)="renameSide(sideSaved('right')!)"><mat-icon>edit</mat-icon><span i18n="@@Rename">Rename</span></button>
              <button mat-menu-item [disabled]="!sideSaved('right')" (click)="deleteSide('right', sideSaved('right')!)"><mat-icon>delete</mat-icon><span i18n="@@Delete">Delete</span></button>
            </mat-menu>
            <span class="actions">
              <button matIconButton [disabled]="!undecided()" (click)="chooseAll('right')" matTooltip="Every difference still to decide: choose this side" i18n-matTooltip="@@Choose_all_tip" aria-label="Choose all from this side" i18n-aria-label="@@Choose_all_aria">
                <mat-icon>done_all</mat-icon>
              </button>
            </span>
          </div>
        </div>
        @for (g of groups(); track g.kind) {
          <div class="section">{{ g.label }}<span class="n">{{ g.rows.length }}</span></div>
          @for (e of g.rows; track e.path) {
            <app-compose-block
              [entity]="e"
              [recipe]="recipe"
              [version]="version()"
              [leftName]="answer()?.leftName ?? ''"
              [rightName]="answer()?.rightName ?? ''"
              [onlyDiff]="onlyDiff()"
              [pulledBy]="pulledBy(e)"
              [warnings]="warningsOf(e)"
              [pairProposal]="pairProposal(e)"
              [media]="mediaOf(e)"
              [routeNames]="routeNames()"
              (pick)="onPick($event)"
            />
          }
        } @empty {
          @if (answer()) {
            <div class="empty">
              <mat-icon>done_all</mat-icon>
              <span i18n="@@Compose_identical">These two configurations are identical. Nothing to compose.</span>
            </div>
          }
        }
      </div>
    </div>
  `,
})
export class ComposePageComponent {
  private readonly api = inject(ApiService);
  private readonly me = inject(MeService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly dialogs = inject(DialogsService);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly recipe = new Recipe();
  // Bumped at each change of the recipe: the blocks read it to recompute.
  protected readonly version = signal(0);
  protected readonly accepted = FILES;
  protected readonly answer = signal<ComposeAnswer | null>(null);
  // How many saved configurations this image keeps, 0 for no limit: said
  // before it is reached, where a save is decided.
  protected readonly cap = signal(0);
  protected readonly full = computed(() => this.cap() > 0 && this.saved().length >= this.cap());
  protected readonly busy = signal(false);
  // Off by default: the page opens on what runs, on both sides, whole; the
  // differences appear when the right side changes.
  protected readonly onlyDiff = signal(false);
  protected readonly kinds = signal(new Set<string>());

  protected readonly leftKey = signal('running');
  protected readonly rightKey = signal('running');
  // A file chosen or dropped for a side: its text, kept here and sent with
  // the body.
  private readonly files = signal(new Map<'left' | 'right', { name: string; src: ComposeSource }>());

  protected readonly saved = signal<SavedConfiguration[]>([]);
  protected readonly remotes = signal<ConfigRemote[]>([]);

  // What a side can show: what runs, the shelf, the git locations, the file
  // picked for that side. The restore points stay out: thirty of them drown
  // the shelf, and the tape's own page is where one is chosen.
  private readonly sources = computed<SourceOption[]>(() => {
    const out: SourceOption[] = [{ value: 'running', label: $localize`:@@Running_this_gateway:Running (this gateway)`, src: { source: 'running' } }];
    for (const c of this.saved()) {
      out.push({ value: `configuration:${c.id}`, label: $localize`:@@Saved_NAME:Saved: ${c.name}:name:`, src: { source: 'configuration', id: c.id } });
    }
    for (const r of this.remotes()) {
      out.push({ value: `remote:${r.id}`, label: $localize`:@@Git_NAME:Git: ${r.name}:name:`, src: { source: 'remote', id: r.id } });
    }
    for (const [side, f] of this.files()) {
      out.push({ value: `file:${side}`, label: $localize`:@@File_NAME:File: ${f.name}:name:`, src: f.src });
    }
    return out;
  });

  protected options(side: 'left' | 'right'): SourceOption[] {
    return this.sources().filter((o) => !o.value.startsWith('file:') || o.value === `file:${side}`);
  }

  private readonly ask = new Subject<ComposeBody>();

  constructor() {
    // The sources, once; the page asks nothing again by itself. A side named
    // in the URL resolves once its list is here: the first preview waits for it.
    const loaded = () => {
      if (!this.answer()) this.preview();
    };
    const readShelf = () => this.api.configurations().subscribe({ next: (l) => (this.saved.set(l), loaded()), error: () => undefined });
    const readRemotes = () => {
      if (this.me.enterprise()) this.api.configRemotes().subscribe({ next: (l) => (this.remotes.set(l), loaded()), error: () => undefined });
    };
    readShelf();
    readRemotes();
    this.api.edition().subscribe({ next: (e) => this.cap.set(e.configurationCap ?? 0), error: () => undefined });
    // The lists follow what the gateway announces (a row renamed in another
    // session, a location added): the live channel, never a timer.
    const live = inject(LiveChangesService);
    live.on('configuration', readShelf);
    live.on('config-remote', readRemotes);
    // One preview in flight at a time, the last click wins.
    this.ask
      .pipe(
        debounceTime(150),
        switchMap((body) =>
          this.api.composePreview(body).pipe(
            catchError((err: unknown) => {
              this.fail(err);
              return of(null);
            }),
          ),
        ),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((a) => {
        this.busy.set(false);
        if (a) this.answer.set(a);
      });
    // The sides follow the URL: a link pasted, a side chosen in a header, the
    // first load - one path for all three.
    this.route.queryParamMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((q) => {
      const left = q.get('left');
      const right = q.get('right');
      let moved = false;
      if (left && left !== this.leftKey()) {
        this.leftKey.set(left);
        moved = true;
      }
      if (right && right !== this.rightKey()) {
        this.rightKey.set(right);
        moved = true;
      }
      if (moved) this.restart();
    });
    afterRenderEffect(() => {
      this.answer();
      this.version();
      this.onlyDiff();
      this.kinds();
      this.redraw();
    });
    new ResizeObserver(() => this.redraw()).observe(this.host.nativeElement);
  }

  // ── sources ────────────────────────────────────────────────────────────────

  private sourceOf(key: string, side: 'left' | 'right'): ComposeSource | null {
    if (key === `file:${side}` || key === 'file') {
      return this.files().get(side)?.src ?? null;
    }
    return this.sources().find((o) => o.value === key)?.src ?? null;
  }

  protected async chooseSource(side: 'left' | 'right', key: string): Promise<void> {
    if (key === 'file') {
      const [file] = await pickFiles({ accept: FILES });
      if (!file) return;
      await this.useFile(side, file);
      return;
    }
    (side === 'left' ? this.leftKey : this.rightKey).set(key);
    // The URL says the same as the screen: a later navigation of the page
    // (a file opened in the drawer) must not read an older side back from it.
    void this.router.navigate([], { relativeTo: this.route, queryParams: { [side]: key }, queryParamsHandling: 'merge', replaceUrl: true });
    this.restart();
  }

  // A file dropped on the page lands on the right: the side one compares with.
  protected async dropped(files: File[]): Promise<void> {
    if (files[0]) await this.useFile('right', files[0]);
  }

  private async useFile(side: 'left' | 'right', file: File): Promise<void> {
    // A package by what it is, not by its name: PK, the magic number of a ZIP.
    const bytes = new Uint8Array(await file.arrayBuffer());
    const zip = bytes[0] === 0x50 && bytes[1] === 0x4b;
    const src: ComposeSource = zip
      ? { source: 'file', package: await base64(file) }
      : { source: 'file', document: new TextDecoder().decode(bytes) };
    this.files.update((m) => new Map(m).set(side, { name: file.name, src }));
    (side === 'left' ? this.leftKey : this.rightKey).set(`file:${side}`);
    // A file is not in the URL: nothing there may name another side.
    void this.router.navigate([], { relativeTo: this.route, queryParams: { [side]: null }, queryParamsHandling: 'merge', replaceUrl: true });
    this.restart();
  }

  // A side changed: the choices made against the previous one mean nothing.
  private restart(): void {
    this.recipe.reset();
    this.version.update((v) => v + 1);
    this.preview();
  }

  private body(): ComposeBody | null {
    const left = this.sourceOf(this.leftKey(), 'left');
    const right = this.sourceOf(this.rightKey(), 'right');
    if (!left || !right) return null;
    return { left, right, pairs: this.recipe.pairs, picks: this.recipe.list() };
  }

  protected preview(): void {
    const body = this.body();
    if (!body) return;
    this.busy.set(true);
    this.ask.next(body);
  }

  // ── rows ───────────────────────────────────────────────────────────────────

  protected readonly rows = computed<Entity[]>(() => {
    const a = this.answer();
    this.version();
    if (!a) return [];
    return entities(a.left, a.right, a.result, this.recipe.pairs, a.media);
  });

  protected readonly kindCounts = computed(() =>
    KINDS.map((K) => ({
      kind: K.kind,
      label: K.label,
      n: this.rows().filter((e) => e.kind === K.kind && (e.status !== 'same' || this.pulledBy(e).length)).length,
    })).filter((k) => k.n > 0 || this.kinds().has(k.kind)),
  );

  protected readonly groups = computed(() => {
    const picked = this.kinds();
    const only = this.onlyDiff();
    return KINDS.map((K) => ({
      kind: K.kind,
      label: K.label,
      rows: this.rows().filter((e) => {
        if (e.kind !== K.kind) return false;
        if (picked.size && !picked.has(e.kind)) return false;
        if (!only) return true;
        return e.status !== 'same' || this.pulledBy(e).length > 0 || !e.result;
      }),
    })).filter((g) => g.rows.length > 0);
  });

  protected toggleKind(kind: string, on: boolean): void {
    this.kinds.update((s) => {
      const next = new Set(s);
      if (on) next.add(kind);
      else next.delete(kind);
      return next;
    });
    this.scrollTop();
  }

  protected setOnlyDiff(on: boolean): void {
    this.onlyDiff.set(on);
    this.scrollTop();
  }

  // A filter changed what the loom lists: back to the top of the panel that
  // scrolls it, or the reader is left in the middle of another list.
  private scrollTop(): void {
    let el: HTMLElement | null = this.host.nativeElement;
    while (el && !/(auto|scroll)/.test(getComputedStyle(el).overflowY)) el = el.parentElement;
    el?.scrollTo({ top: 0 });
  }

  protected pulledBy(e: Entity): string[] {
    return this.answer()?.report.pulled.find((p) => p.kind === e.kind && p.id === e.id)?.by ?? [];
  }

  protected warningsOf(e: Entity): string[] {
    const a = this.answer();
    if (!a) return [];
    return a.report.missing
      .filter((m) => m.by.includes(e.label) || m.by.includes(e.id))
      .map((m) =>
        m.leftOut
          ? m.hard
            ? $localize`:@@Compose_hard_left_out:${e.label}:who: cannot be saved without ${m.kind}:kind: ${m.key}:key:, which was left out. Take it back, or leave ${e.label}:who2: out too.`
            : $localize`:@@Compose_soft_left_out:${e.label}:who: names ${m.kind}:kind: ${m.key}:key:, which was left out: the rule never matches until it exists.`
          : $localize`:@@Compose_declared_nowhere:${m.kind}:kind: ${m.key}:key: is named here (${m.via}:via:) and declared on neither side: the rule never matches. Declare it, or take it out.`,
      );
  }

  protected pairProposal(e: Entity): string | undefined {
    if (e.status !== 'left-only') return undefined;
    return this.answer()?.report.pairs.find((p) => p.kind === e.kind && p.left === e.id)?.right;
  }

  // The spec and files a route carries, on each side and in the result.
  // id -> name of every route either side carries, for what names an entry.
  protected readonly routeNames = computed(() => {
    const out = new Map<string, string>();
    const a = this.answer();
    for (const doc of [a?.left, a?.right]) {
      const routes = doc?.['routes'];
      if (!Array.isArray(routes)) continue;
      for (const r of routes as { id?: string; name?: string }[]) {
        if (typeof r.id === 'string' && typeof r.name === 'string' && !out.has(r.id)) out.set(r.id, r.name);
      }
    }
    return out;
  });

  protected mediaOf(e: Entity): Media | null {
    const a = this.answer();
    if (!a || e.kind !== 'route') return null;
    const m = { left: a.media.left[e.id], right: a.media.right[e.pairedWith ?? e.id], result: a.media.result[e.id] };
    return m.left || m.right || m.result ? m : null;
  }

  // What the result would lack, in sentences: the plan's holes (vault
  // entries, spec and files, Enterprise parts, OIDC bindings) and the
  // composition's hard misses.
  protected readonly alerts = computed<string[]>(() => {
    const a = this.answer();
    if (!a) return [];
    const out: string[] = [];
    for (const m of a.plan?.missing ?? []) {
      const where = m.used?.length ? ` (${m.used.join(', ')})` : '';
      out.push($localize`:@@Compose_vault_missing:Vault entry ${m.name}:name: is named${where}:where: and this gateway does not hold it: it would be reserved empty.`);
    }
    for (const f of a.plan?.missingFiles ?? []) {
      out.push($localize`:@@Compose_file_missing:Route ${f.route}:route: names ${f.kind}:kind: ${f.name}:name:, which neither side carries.`);
    }
    for (const n of a.plan?.notApplied ?? []) {
      out.push($localize`:@@Compose_not_applied:Not applied on this edition: ${n}:what:.`);
    }
    for (const u of a.plan?.unbound ?? []) {
      out.push(u);
    }
    for (const m of a.report.missing.filter((x) => x.hard)) {
      out.push($localize`:@@Compose_blocked_line:${m.by.join(', ')}:who: cannot be saved without ${m.kind}:kind: ${m.key}:key:: applying is blocked until it is taken back.`);
    }
    if (a.planError) out.push(a.planError);
    return out;
  });

  protected readonly blocked = computed(() => {
    const m = this.answer()?.report.missing.find((x) => x.hard);
    return m ? $localize`:@@Compose_blocked:${m.by.join(', ')}:who: cannot be saved without ${m.kind}:kind: ${m.key}:key:` : '';
  });

  // The blocks still to decide: what Choose all would touch.
  protected readonly undecided = computed(() => {
    this.version();
    return this.rows().filter((e) => e.status !== 'same' && !this.pulledBy(e).length && !this.recipe.touched(e.path) && !this.pairProposal(e)).length;
  });
  // Whether a choice was made at all: what Reset would forget.
  protected readonly touched = computed(() => {
    this.version();
    return this.recipe.list().length > 0 || this.recipe.pairs.length > 0;
  });

  protected readonly removes = computed(() => this.answer()?.plan?.changes.filter((c) => c.action === 'remove').length ?? 0);

  // ── the recipe ─────────────────────────────────────────────────────────────

  protected onPick(ev: PickEvent): void {
    switch (ev.type) {
      case 'whole':
        this.recipe.whole(ev.path, ev.from);
        break;
      case 'field':
        this.recipe.field(ev.path, ev.from);
        break;
      case 'element':
        this.recipe.element(ev.list, ev.key, ev.op);
        break;
      case 'order':
        this.recipe.order(ev.list, ev.keys);
        break;
      case 'pair':
        this.recipe.pair(ev.kind, ev.left, ev.right);
        break;
      case 'include':
        if (ev.on) this.recipe.forget(ev.path);
        else this.recipe.whole(ev.path, 'none');
        break;
    }
    this.version.update((v) => v + 1);
    this.preview();
  }

  // Every block still to decide, from one side: kept or taken when the side
  // has it, left out when it does not. A pairing stays a human call.
  protected chooseAll(side: 'left' | 'right'): void {
    const proposedRight = new Set(this.answer()?.report.pairs.map((p) => p.right) ?? []);
    for (const e of this.rows()) {
      if (e.status === 'same' || this.pulledBy(e).length || this.recipe.touched(e.path) || this.pairProposal(e)) continue;
      if (e.status === 'right-only' && proposedRight.has(e.id)) continue;
      const has = side === 'left' ? !!e.left : !!e.right;
      this.recipe.whole(e.path, has ? side : 'none');
    }
    this.version.update((v) => v + 1);
    this.preview();
  }

  protected reset(): void {
    this.recipe.reset();
    this.version.update((v) => v + 1);
    this.preview();
  }

  // ── a side that is a saved configuration: what one does to the row ─────────

  protected sideSaved(side: 'left' | 'right'): SavedConfiguration | undefined {
    const key = side === 'left' ? this.leftKey() : this.rightKey();
    if (!key.startsWith('configuration:')) return undefined;
    return this.saved().find((c) => `configuration:${c.id}` === key);
  }

  protected async renameSide(c: SavedConfiguration): Promise<void> {
    const name = await this.dialogs.prompt({
      title: $localize`:@@Rename_NAME:Rename ${c.name}:name:`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Rename:Rename`,
    });
    if (!name || name === c.name) return;
    this.api.renameConfiguration(c.id, name, c.description ?? '').subscribe({
      next: () => {
        this.snack.open($localize`:@@Renamed:Renamed`, undefined, { duration: 2500 });
        this.api.configurations().subscribe({ next: (l) => this.saved.set(l), error: () => undefined });
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  // Deleting the row a side shows: the side goes back to what runs.
  protected async deleteSide(side: 'left' | 'right', c: SavedConfiguration): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_configuration:Delete ${c.name}:name:?`,
      message: c.active
        ? $localize`:@@Delete_active_configuration_message:It is the current one. The configuration in use does not change; it just loses its name.`
        : $localize`:@@Delete_configuration_message:The saved copy is deleted. The configuration in use does not change.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteConfiguration(c.id).subscribe({
      next: () => {
        this.snack.open($localize`:@@Deleted:Deleted`, undefined, { duration: 2500 });
        this.api.configurations().subscribe({ next: (l) => this.saved.set(l), error: () => undefined });
        void this.router.navigate([], { relativeTo: this.route, queryParams: { [side]: 'running' }, queryParamsHandling: 'merge' });
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  // ── destinations ───────────────────────────────────────────────────────────

  // Apply: one confirmation, in numbers and names - what is added, changed
  // and removed, and what the result lacks - then the switch.
  protected async apply(): Promise<void> {
    const a = this.answer();
    const body = this.body();
    if (!a?.plan || !body) return;
    const plan = a.plan;
    const count = (action: string) => plan.changes.filter((c) => c.action === action).length;
    const removed = plan.changes.filter((c) => c.action === 'remove').map((c) => c.label);
    const lines = [
      $localize`:@@Compose_counts:${count('add')}:added: added, ${count('update')}:changed: changed, ${count('remove')}:removed: removed.`,
    ];
    if (removed.length) lines.push($localize`:@@Compose_removed_list:Removed: ${removed.join(', ')}:names:.`);
    lines.push(...this.alerts());
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Apply_the_composed_configuration:Apply the composed configuration`,
      message: lines.join('\n'),
      confirmLabel: $localize`:@@Apply:Apply`,
      danger: removed.length > 0 || this.alerts().length > 0,
    });
    if (!ok) return;
    this.busy.set(true);
    this.api.composeApply(body).subscribe({
      next: () => {
        this.busy.set(false);
        this.snack.open($localize`:@@Composed_configuration_applied:The composed configuration is now the current one`, undefined, { duration: 2500 });
        this.rightKey.set('');
        this.answer.set(null);
        this.recipe.reset();
        void this.router.navigate([], { relativeTo: this.route, queryParams: { right: null, left: null }, queryParamsHandling: 'merge' });
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Which file the drawer can open for a side: a saved configuration's, to
  // read and correct, or what runs, to read.
  protected sideFile(side: 'left' | 'right'): string | null {
    const key = side === 'left' ? this.leftKey() : this.rightKey();
    return key === 'running' ? 'running' : (this.sideSaved(side)?.id ?? null);
  }

  protected readSide(side: 'left' | 'right'): void {
    void this.router.navigate([], { relativeTo: this.route, queryParams: { read: this.sideFile(side) }, queryParamsHandling: 'merge' });
  }

  // Save on the shelf. A name already there is a question, not a refusal:
  // "save it again" is the commonest wish here.
  protected async saveOnShelf(): Promise<void> {
    const body = this.body();
    if (!body) return;
    const name = await this.dialogs.prompt({
      title: $localize`:@@Save_under_a_name:Save under a name`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Save:Save`,
    });
    if (!name) return;
    let replace = false;
    if (this.saved().some((c) => c.name.toLowerCase() === name.toLowerCase())) {
      replace = await this.dialogs.confirm({
        title: $localize`:@@Replace_NAME:Replace ${name}:name:?`,
        message: $localize`:@@Replace_configuration_message:A configuration of that name is on the shelf. Its document is replaced by the result. Nothing is applied.`,
        confirmLabel: $localize`:@@Replace:Replace`,
        danger: true,
      });
      if (!replace) return;
    }
    this.busy.set(true);
    this.api.composeSave({ ...body, name, replace }).subscribe({
      next: (c) => {
        this.busy.set(false);
        this.snack.open($localize`:@@Saved_as_NAME:Saved as ${c.name}:name:`, undefined, { duration: 2500 });
        this.api.configurations().subscribe({ next: (l) => this.saved.set(l), error: () => undefined });
      },
      error: async (err: unknown) => {
        this.busy.set(false);
        // The shelf moved under us: the name is taken after all. Same question.
        if (!replace && (err as { status?: number }).status === 409) {
          const again = await this.dialogs.confirm({
            title: $localize`:@@Replace_NAME:Replace ${name}:name:?`,
            message: $localize`:@@Replace_configuration_message:A configuration of that name is on the shelf. Its document is replaced by the result. Nothing is applied.`,
            confirmLabel: $localize`:@@Replace:Replace`,
            danger: true,
          });
          if (again) {
            this.busy.set(true);
            this.api.composeSave({ ...body, name, replace: true }).subscribe({
              next: (c) => {
                this.busy.set(false);
                this.snack.open($localize`:@@Saved_as_NAME:Saved as ${c.name}:name:`, undefined, { duration: 2500 });
              },
              error: (e2: unknown) => {
                this.busy.set(false);
                this.fail(e2);
              },
            });
          }
          return;
        }
        this.fail(err);
      },
    });
  }

  // The shape is chosen where what each one leaves behind can be read.
  protected async saveOnFilesystem(): Promise<void> {
    const a = this.answer();
    if (!a) return;
    const names = this.routeNames();
    const routes = (has: (m: { spec?: boolean; files?: number }) => unknown) =>
      Object.entries(a.media.result)
        .filter(([, m]) => has(m))
        .map(([id]) => names.get(id) ?? id);
    const { SaveFileDialogComponent } = await import('./save-file-dialog.component');
    const shape = await firstValueFrom(
      this.dialog
        .open(SaveFileDialogComponent, {
          data: { leaves: a.leaves, specs: routes((m) => m.spec), files: routes((m) => m.files) },
          width: '560px',
        })
        .afterClosed(),
    );
    if (shape) this.saveAsFile(shape === 'package');
  }

  private saveAsFile(zip: boolean): void {
    const body = this.body();
    if (!body) return;
    this.api.composeExport({ ...body, format: zip ? 'zip' : 'yaml' }).subscribe({
      next: (blob) => {
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = zip ? 'meerkat-composed.zip' : 'meerkat-composed.yaml';
        a.click();
        URL.revokeObjectURL(url);
      },
      // A download answers a Blob, its refusal too: the words are inside.
      error: async (err: unknown) => {
        const blob = (err as { error?: unknown }).error;
        if (!(blob instanceof Blob)) return this.fail(err);
        const said = await blob.text().then((t) => JSON.parse(t) as { error?: string }, () => ({}) as { error?: string });
        this.fail({ error: said });
      },
    });
  }

  // The git location read on the right, when it is one: the only place a
  // push can go.
  protected readonly rightRemote = computed<ConfigRemote | undefined>(() => {
    const key = this.rightKey();
    if (!key.startsWith('remote:')) return undefined;
    return this.remotes().find((r) => `remote:${r.id}` === key);
  });

  // Push: the location names the row (a directory per platform holds one
  // configuration), so nothing is asked but the one thing worth confirming:
  // the directory there is replaced. The revision the right side was read at
  // goes with it: a location that moved since is a refusal, never a force.
  protected async pushTo(r: ConfigRemote): Promise<void> {
    const body = this.body();
    const rev = this.answer()?.rightRev;
    if (!body || !rev) return;
    const where = `${r.url} (${r.branch}${r.dir ? ', ' + r.dir : ''})`;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Push_to_LOCATION_title:Push to ${r.name}:location:`,
      message: $localize`:@@Push_replaces_remote:This writes ${where}:where:: the directory there, as read on the right (${rev.slice(0, 8)}:rev:), is replaced by the result. Nothing is saved here and nothing is applied to this gateway.`,
      confirmLabel: $localize`:@@Push:Push`,
      danger: true,
    });
    if (!ok) return;
    this.busy.set(true);
    this.api.composePush({ ...body, remoteId: r.id, remoteRev: rev }).subscribe({
      next: (p) => {
        this.busy.set(false);
        this.snack.open($localize`:@@Pushed_to_git_at:Pushed to ${r.name}:location: (${p.rev.slice(0, 8)}:rev:)`, undefined, { duration: 2500 });
        // The location now holds the result: the right side is read again.
        this.restart();
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // ── ribbons and bands ──────────────────────────────────────────────────────

  // The ribbons are a curve from the card or the value that feeds the result
  // to the result's own, drawn in the loom's coordinates; the bands under each
  // column follow the headers. Redrawn after each render and on resize, never
  // on a timer.
  protected redraw(): void {
    const host = this.host.nativeElement;
    const loom = host.querySelector<HTMLElement>('.loom');
    const grid = host.querySelector<HTMLElement>('.grid');
    const svg = host.querySelector<SVGSVGElement>('svg.ribbons');
    if (!loom || !grid || !svg) return;
    const g = grid.getBoundingClientRect();
    for (const side of ['left', 'center', 'right']) {
      const head = grid.querySelector<HTMLElement>('.colhead.' + side);
      const band = grid.querySelector<HTMLElement>('.colband.' + side);
      if (!head || !band) continue;
      // Exactly the header's width: the frame of a block overflows the
      // bands on both sides, the bands never overflow the headers.
      const h = head.getBoundingClientRect();
      band.style.left = `${h.left - g.left}px`;
      band.style.width = `${h.width}px`;
    }
    const base = loom.getBoundingClientRect();
    svg.setAttribute('width', String(loom.scrollWidth));
    svg.setAttribute('height', String(loom.scrollHeight));
    const rect = (el: globalThis.Element) => {
      const r = el.getBoundingClientRect();
      return { x: r.left - base.left, y: r.top - base.top, w: r.width, h: r.height };
    };
    const paths: string[] = [];
    const curve = (from: HTMLElement, to: HTMLElement, colour: string, fromRight: boolean) => {
      const a = rect(from);
      const b = rect(to);
      if (!a.w || !b.w) return;
      const x1 = fromRight ? a.x : a.x + a.w;
      const y1 = a.y + a.h / 2;
      const x2 = fromRight ? b.x + b.w : b.x;
      const y2 = b.y + b.h / 2;
      // A ribbon runs from a side to the centre: never backwards, never a loop.
      if (fromRight ? x2 >= x1 : x2 <= x1) return;
      const dx = (x2 - x1) / 2;
      paths.push(
        `<path d="M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}" fill="none" stroke="${colour}" stroke-width="3" opacity=".75"/>`,
      );
    };
    const left = 'var(--mk-left)';
    const right = 'var(--mk-right)';
    // Object cards: from the side whose radio is on to the result card.
    for (const res of grid.querySelectorAll<HTMLElement>('.card.result.from-left, .card.result.from-right, .card.result.from-both')) {
      const cell = res.closest('.cell');
      const row = cell?.parentElement;
      if (!row) continue;
      const both = res.classList.contains('from-both');
      if (both || res.classList.contains('from-left')) {
        const src = row.querySelector<HTMLElement>('.card.side-left');
        if (src) curve(src, res, left, false);
      }
      if (both || res.classList.contains('from-right')) {
        const src = row.querySelector<HTMLElement>('.card.side-right');
        if (src) curve(src, res, right, true);
      }
    }
    // Field lines: from the chosen value to its result line.
    for (const cell of grid.querySelectorAll<HTMLElement>('.fcell.val.chosen')) {
      const card = cell.dataset['fcard'];
      if (!card) continue;
      const path = card.replace(/:(left|right)$/, '');
      const res = grid.querySelector<HTMLElement>(`[data-fcard="${CSS.escape(path + ':res')}"]`);
      if (!res) continue;
      const fromLeft = cell.classList.contains('left');
      curve(cell, res, fromLeft ? left : right, !fromLeft);
    }
    svg.innerHTML = paths.join('');
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
