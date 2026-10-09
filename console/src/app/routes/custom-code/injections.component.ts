import { CdkDrag, CdkDragDrop, CdkDragHandle, CdkDropList, moveItemInArray } from '@angular/cdk/drag-drop';
import { Component, computed, effect, inject, input, model, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { type FormValueControl } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, Injection, InjectionLoad, InjectionPosition, RouteFile } from '../../api.service';
import { pickFiles } from '../../shared/file-pick';
import { Lazy } from '../../shared/lazy';
import { servedName } from '../filters/files-filter.component';

// A UI route's own CSS and JavaScript (UIF-02): an ordered list of blocks,
// each written here or uploaded as a file, each placed in the page. The order
// is the list's, dragged; the place is each block's. A FormValueControl bound
// with [formField].
@Component({
  selector: 'app-injections',
  imports: [
    CdkDrag,
    CdkDragHandle,
    CdkDropList,
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatMenuModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  styles: `
    :host { display: block; }
    .list { display: grid; gap: 8px; margin-bottom: 12px; }
    .block {
      display: grid; grid-template-columns: auto auto minmax(0, 1fr) 11rem 9rem auto; align-items: center; gap: 8px;
      padding: 6px 8px; border-radius: 12px; background: var(--mat-sys-surface-container);
    }
    .handle { cursor: grab; color: var(--mat-sys-on-surface-variant); }
    .what { display: flex; align-items: center; gap: 6px; min-width: 0; }
    .what .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .meta { font-size: 0.8rem; color: var(--mat-sys-on-surface-variant); white-space: nowrap; }
    .missing { color: var(--mat-sys-error); }
    .empty { font-size: 0.85rem; color: var(--mat-sys-on-surface-variant); }
    .error { color: var(--mat-sys-error); font-size: 0.8rem; }
    .cdk-drag-preview { box-shadow: var(--mat-sys-level3); }
    .cdk-drag-placeholder { opacity: 0.3; }
  `,
  template: `
    @if (value().length) {
      <div class="list" cdkDropList (cdkDropListDropped)="drop($event)">
        @for (b of value(); track $index; let i = $index) {
          <div class="block" cdkDrag>
            <mat-icon class="handle" cdkDragHandle>drag_indicator</mat-icon>
            <mat-icon>{{ b.kind === 'css' ? 'css' : 'javascript' }}</mat-icon>
            <span class="what">
              @if (b.file) {
                <mat-icon class="meta">description</mat-icon>
                <span class="name" [class.missing]="!has(b.file)">{{ b.file }}</span>
                @if (!has(b.file)) {
                  <span class="meta missing" i18n="@@Injection_file_missing">not uploaded</span>
                }
                <button matIconButton type="button" [disabled]="busy()" (click)="replace(i)"
                  matTooltip="Upload another version" i18n-matTooltip="@@Injection_replace"
                  aria-label="Upload another version" i18n-aria-label="@@Injection_replace">
                  <mat-icon>upload_file</mat-icon>
                </button>
              } @else {
                <button matButton type="button" (click)="edit(i)">
                  <mat-icon>edit</mat-icon>
                  @if (lines(b); as n) {
                    <ng-container i18n="@@N_lines_plural">{n, plural, =1 {1 line} other {{{ n }} lines}}</ng-container>
                  } @else {
                    <ng-container i18n="@@Injection_write">Write it</ng-container>
                  }
                </button>
              }
            </span>
            <mat-form-field subscriptSizing="dynamic">
              <mat-label i18n="@@Injection_position">Where</mat-label>
              <mat-select [value]="b.position" (selectionChange)="patch(i, { position: $event.value })">
                @for (p of positions; track p.value) {
                  <mat-option [value]="p.value">{{ p.label }}</mat-option>
                }
              </mat-select>
            </mat-form-field>
            @if (b.kind === 'js') {
              <mat-form-field subscriptSizing="dynamic">
                <mat-label i18n="@@Injection_load">Runs</mat-label>
                <mat-select [value]="b.load ?? ''" (selectionChange)="patch(i, { load: $event.value })">
                  @for (l of loads; track l.value) {
                    <mat-option [value]="l.value" [disabled]="l.fileOnly && !b.file">{{ l.label }}</mat-option>
                  }
                </mat-select>
              </mat-form-field>
            } @else {
              <span></span>
            }
            <button matIconButton type="button" (click)="remove(i)"
              matTooltip="Remove the block" i18n-matTooltip="@@Injection_remove"
              aria-label="Remove the block" i18n-aria-label="@@Injection_remove">
              <mat-icon>delete</mat-icon>
            </button>
          </div>
        }
      </div>
    } @else {
      <p class="empty" i18n="@@Injection_none">No custom code yet.</p>
    }
    @if (error(); as e) {
      <p class="error">{{ e }}</p>
    }
    <button matButton="tonal" type="button" [matMenuTriggerFor]="addMenu" [disabled]="busy()">
      <mat-icon>add</mat-icon><ng-container i18n="@@Injection_add">Add</ng-container>
    </button>
    <mat-menu #addMenu="matMenu">
      <button mat-menu-item (click)="add('css', false)"><mat-icon>css</mat-icon><span i18n="@@Injection_add_css">CSS, written here</span></button>
      <button mat-menu-item (click)="add('css', true)"><mat-icon>upload_file</mat-icon><span i18n="@@Injection_add_css_file">CSS file</span></button>
      <button mat-menu-item (click)="add('js', false)"><mat-icon>javascript</mat-icon><span i18n="@@Injection_add_js">JavaScript, written here</span></button>
      <button mat-menu-item (click)="add('js', true)"><mat-icon>upload_file</mat-icon><span i18n="@@Injection_add_js_file">JavaScript file</span></button>
    </mat-menu>
  `,
})
export class InjectionsComponent implements FormValueControl<Injection[]> {
  readonly value = model<Injection[]>([]);
  readonly routeId = input<string | undefined>();
  // Saves the route when it has never been, and answers its id: a file is
  // stored against a route that exists.
  readonly ensureSaved = input.required<() => Promise<string>>();
  // Saves the route as it stands: what the code dialog's Save means, and
  // what makes an uploaded file and the block naming it land together.
  readonly persist = input.required<() => void>();

  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  private readonly lazy = inject(Lazy);
  private readonly files = signal<RouteFile[]>([]);
  private readonly savedId = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  private readonly names = computed(() => new Set(this.files().map((f) => f.name)));

  protected readonly positions: { value: InjectionPosition; label: string }[] = [
    { value: 'head-start', label: $localize`:@@Injection_head_start:Start of the head` },
    { value: 'head-end', label: $localize`:@@Injection_head_end:End of the head` },
    { value: 'body-end', label: $localize`:@@Injection_body_end:End of the body` },
  ];
  protected readonly loads: { value: InjectionLoad; label: string; fileOnly: boolean }[] = [
    { value: '', label: $localize`:@@Injection_load_classic:Where it stands`, fileOnly: false },
    { value: 'defer', label: $localize`:@@Injection_load_defer:Deferred`, fileOnly: true },
    { value: 'async', label: $localize`:@@Injection_load_async:Async`, fileOnly: true },
    { value: 'module', label: $localize`:@@Injection_load_module:Module`, fileOnly: false },
  ];

  constructor() {
    effect(() => {
      const id = this.routeId();
      if (id) this.load(id);
    });
  }

  protected has(name: string): boolean {
    return this.names().has(name);
  }

  protected lines(b: Injection): number {
    const code = (b.code ?? '').trim();
    return code ? code.split('\n').length : 0;
  }

  protected drop(e: CdkDragDrop<unknown>): void {
    if (e.previousIndex === e.currentIndex) return;
    this.value.update((l) => {
      const next = [...l];
      moveItemInArray(next, e.previousIndex, e.currentIndex);
      return next;
    });
  }

  protected patch(i: number, change: Partial<Injection>): void {
    this.value.update((l) => l.map((b, j) => (j === i ? { ...b, ...change } : b)));
  }

  protected remove(i: number): void {
    this.value.update((l) => l.filter((_, j) => j !== i));
  }

  // A new block lands at the end of the head: after the application's own
  // stylesheets, where an override wins, and where a script still runs before
  // the page shows. A script from a file is deferred: it runs once the page is
  // parsed, in the list's order.
  protected async add(kind: 'css' | 'js', fromFile: boolean): Promise<void> {
    const block: Injection = { kind, position: 'head-end' };
    if (!fromFile) {
      const code = await this.write(kind, '');
      if (code === undefined) return;
      this.value.update((l) => [...l, { ...block, code }]);
      this.persist()();
      return;
    }
    const name = await this.upload(kind);
    if (!name) return;
    this.value.update((l) => [...l, { ...block, file: name, ...(kind === 'js' ? { load: 'defer' as const } : {}) }]);
    this.persist()();
  }

  protected async edit(i: number): Promise<void> {
    const b = this.value()[i];
    const code = await this.write(b.kind, b.code ?? '');
    if (code === undefined) return;
    this.patch(i, { code });
    this.persist()();
  }

  protected async replace(i: number): Promise<void> {
    const b = this.value()[i];
    const name = await this.upload(b.kind, b.file);
    if (!name) return;
    if (name !== b.file) this.patch(i, { file: name });
    this.persist()();
  }

  private async write(kind: 'css' | 'js', code: string): Promise<string | undefined> {
    const mod = await this.lazy.load(() => import('../code-dialog.component'));
    if (!mod) return undefined;
    return firstValueFrom(
      this.dialog
        .open<unknown, { code: string; language: 'css' | 'js' }, string | undefined>(mod.CodeDialogComponent, {
          data: { code, language: kind },
          maxWidth: '90vw',
          restoreFocus: true,
        })
        .afterClosed(),
    );
  }

  // Picks a file and stores it on the route, under the name it is served as;
  // a replacement keeps the block's name, so nothing else has to change.
  private async upload(kind: 'css' | 'js', keep?: string): Promise<string | undefined> {
    const [file] = await pickFiles({ accept: kind === 'css' ? '.css,text/css' : '.js,.mjs,text/javascript' });
    if (!file) return undefined;
    this.error.set('');
    this.busy.set(true);
    try {
      let id = this.routeId() || this.savedId();
      if (!id) {
        id = await this.ensureSaved()();
        this.savedId.set(id);
      }
      const name = keep ?? servedName(file.name);
      await firstValueFrom(this.api.putRouteFile(id, name, file));
      this.load(id);
      return name;
    } catch (e) {
      const msg = e instanceof HttpErrorResponse ? e.error?.error : (e as Error)?.message;
      this.error.set(msg || $localize`:@@Files_upload_failed:The file could not be uploaded`);
      return undefined;
    } finally {
      this.busy.set(false);
    }
  }

  private load(id: string): void {
    this.api.listRouteFiles(id).subscribe({ next: (f) => this.files.set(f), error: () => this.files.set([]) });
  }
}
