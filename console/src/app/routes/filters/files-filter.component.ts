import { Component, computed, effect, inject, input, model, signal } from '@angular/core';
import { HttpErrorResponse, httpResource } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, Edition, RouteFile, Spec } from '../../api.service';
import { FileButtonComponent, FileDropDirective } from '../../shared/file-pick';
import { argStr, patchSpec } from '../predicates/args';

// The "files" mode of a route (ROUTE-22): the files uploaded on it, served
// under its own path - the font, stylesheet, script or image a UI asks for
// when nothing behind the gateway serves it. The brick holds how they are
// served (index, CORS, cache); the files themselves live beside the route and
// are uploaded at once, which is why a route that was never saved is saved
// first.
@Component({
  selector: 'app-files-filter',
  imports: [
    FileButtonComponent,
    FileDropDirective,
    MatAutocompleteModule,
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
  ],
  styles: `
    :host { display: block; }
    .drop {
      display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
      padding: 14px 16px; border-radius: 12px;
      border: 1px dashed var(--mat-sys-outline-variant);
      color: var(--mat-sys-on-surface-variant); font-size: 0.85rem;
    }
    .drop.dragging { border-color: var(--mat-sys-primary); background: color-mix(in srgb, var(--mat-sys-primary) 8%, transparent); }
    .list { margin: 12px 0; display: grid; gap: 8px; }
    .file { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 2fr) 4.5rem auto auto; align-items: center; gap: 8px; }
    .file .meta { text-align: end; }
    .warn { color: var(--mat-sys-error); }
    .warn.off { visibility: hidden; }
    .meta { font-size: 0.75rem; color: var(--mat-sys-on-surface-variant); white-space: nowrap; }
    .empty, .note { font-size: 0.8rem; color: var(--mat-sys-on-surface-variant); }
    .error { color: var(--mat-sys-error); font-size: 0.8rem; }
    .options { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 4px 14px; align-items: center; }
  `,
  template: `
    <div class="drop" appFileDrop multiple (filesDropped)="blocker() || upload($event)">
      <app-file-button multiple [disabled]="busy() || !!blocker()" (picked)="upload($event)">
        <span i18n="@@Files_upload">Upload files</span>
      </app-file-button>
      <span i18n="@@Files_drop_hint">or drop them here. Each is served at the route's path followed by its name.</span>
    </div>
    @if (blocker(); as b) {
      <p class="note"><ng-container i18n="@@Files_blocked2">Uploading saves the route first, and it cannot be saved yet. Missing:</ng-container> {{ b }}</p>
    }
    @if (error(); as e) {
      <p class="error">{{ e }}</p>
    }
    @if (files().length) {
      <div class="list">
        @for (f of files(); track f.name) {
          <div class="file">
            <!-- The name the file is SERVED as, which is not the name it was
                 uploaded under: "Agreement - Google Docs.pdf" from a download
                 folder is rarely what a page should ask for. The route's path
                 is fixed, only the name after it is edited. -->
            <mat-form-field subscriptSizing="dynamic">
              <mat-label i18n="@@Files_served_as">Served as</mat-label>
              <span matTextPrefix>{{ base() }}/</span>
              <input matInput [value]="f.name" [disabled]="busy()"
                (change)="update(f, { name: $any($event.target).value })" (keydown.enter)="$any($event.target).blur()" />
            </mat-form-field>
            <mat-form-field subscriptSizing="dynamic">
              <mat-label i18n="@@Files_type">Type</mat-label>
              <input matInput [value]="f.contentType" [disabled]="busy()" [matAutocomplete]="types"
                (focus)="typeQuery.set('')" (input)="typeQuery.set($any($event.target).value)"
                (change)="update(f, { contentType: $any($event.target).value })" (keydown.enter)="$any($event.target).blur()" />
              <mat-autocomplete #types="matAutocomplete" (optionSelected)="update(f, { contentType: $event.option.value })">
                @for (t of typeChoices(); track t) {
                  <mat-option [value]="t">{{ t }}</mat-option>
                }
              </mat-autocomplete>
              <!-- Always there, shown only for a type that was a guess: a suffix
                   inside an @if is not projected into the suffix slot. -->
              <mat-icon matSuffix class="warn" [class.off]="!guessed(f)" [matTooltipDisabled]="!guessed(f)"
                matTooltip="Not recognised from the name or the content - say what it is"
                i18n-matTooltip="@@Files_type_unknown2">warning</mat-icon>
            </mat-form-field>
            <span class="meta">{{ size(f.size) }}</span>
            <a matIconButton [href]="link(f.name)" target="_blank" rel="noopener"
              matTooltip="Open it on the data plane" i18n-matTooltip="@@Files_open"
              aria-label="Open it on the data plane" i18n-aria-label="@@Files_open">
              <mat-icon>open_in_new</mat-icon>
            </a>
            <button matIconButton type="button" [disabled]="busy()" (click)="remove(f)"
              matTooltip="Remove the file" i18n-matTooltip="@@Remove_the_file"
              aria-label="Remove the file" i18n-aria-label="@@Remove_the_file">
              <mat-icon>delete</mat-icon>
            </button>
          </div>
        }
      </div>
    } @else if (routeId()) {
      <p class="empty" i18n="@@Files_none">No file yet: the route answers 404 until one is uploaded.</p>
    } @else if (!blocker()) {
      <p class="empty" i18n="@@Files_unsaved">Uploading a file saves the route first.</p>
    }
    <div class="options">
      <mat-form-field subscriptSizing="dynamic">
        <mat-label><ng-container i18n="@@Files_index_at">Answered at</ng-container> {{ base() || '/' }}</mat-label>
        <mat-select [value]="index()" (selectionChange)="set('index', $event.value)"
          matTooltip="The file served at the route's own path, with no name after it - the index.html of a small site. None: that path answers 404, only the files' own addresses answer"
          i18n-matTooltip="@@Files_index_hint2">
          <mat-option value="" i18n="@@Files_index_none2">None - 404</mat-option>
          @for (f of files(); track f.name) {
            <mat-option [value]="f.name">{{ f.name }}</mat-option>
          }
        </mat-select>
      </mat-form-field>
      <mat-form-field subscriptSizing="dynamic">
        <mat-label i18n="@@Files_max_age">Browser cache (seconds)</mat-label>
        <input matInput type="number" min="0" [value]="maxAge()" (change)="setMaxAge($any($event.target).value)" />
      </mat-form-field>
      <mat-checkbox [checked]="cors()" (change)="set('cors', $event.checked)"
        matTooltip="A font or a module loaded by a page of another origin is refused without it"
        i18n-matTooltip="@@Files_cors_hint"><ng-container i18n="@@Files_cors">Readable from any origin (CORS)</ng-container></mat-checkbox>
    </div>
  `,
})
export class FilesFilterComponent {
  readonly spec = model.required<Spec>();
  readonly routeId = input<string | undefined>();
  // The public path the files are served under, for the links.
  readonly prefix = input('');
  // Saves the route when it has never been, and answers its id: a file is
  // stored against a route that exists.
  readonly ensureSaved = input.required<() => Promise<string>>();
  // Why the route cannot be saved yet - an unsaved route without a name or a
  // path. Uploading would start with a save that fails, so it waits.
  readonly blocker = input('');

  private readonly api = inject(ApiService);
  protected readonly files = signal<RouteFile[]>([]);
  // The id a first upload saved the route under, until the editor is handed
  // the created route: a second upload in between goes to the same one.
  private readonly savedId = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');
  protected readonly index = computed(() => argStr(this.spec(), 'index'));
  protected readonly cors = computed(() => this.spec().args?.['cors'] !== false);
  protected readonly maxAge = computed(() => argStr(this.spec(), 'maxAge') || '3600');

  constructor() {
    effect(() => {
      const id = this.routeId();
      if (id) this.load(id);
    });
  }

  // Where the applications answer: the files are served on the data plane,
  // another origin than this console, so a relative link opened the console.
  private readonly edition = httpResource<Edition>(() => '/api/edition');
  protected readonly base = computed(() => this.prefix().replace(/\/+$/, ''));

  protected link(name: string): string {
    return (this.edition.value()?.dataOrigin ?? '') + this.base() + '/' + encodePath(name);
  }

  // The types offered while one is typed: what a route serves most, filtered
  // by what is in the field. Anything else is typed in full.
  protected readonly typeQuery = signal('');
  protected readonly typeChoices = computed(() => {
    const q = this.typeQuery().trim().toLowerCase();
    return q ? COMMON_TYPES.filter((t) => t.includes(q)) : COMMON_TYPES;
  });

  // A type the gateway could not read from the name or the bytes.
  protected guessed(f: RouteFile): boolean {
    return f.contentType.startsWith('application/octet-stream') || f.contentType.startsWith('text/plain');
  }

  protected size(n: number): string {
    return n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KiB` : `${(n / (1 << 20)).toFixed(1)} MiB`;
  }

  protected set(key: string, value: unknown): void {
    this.spec.update((s) => patchSpec(s, key, value));
  }

  protected setMaxAge(v: string): void {
    const n = Math.max(0, Math.floor(Number(v)));
    this.set('maxAge', Number.isFinite(n) ? n : 3600);
  }

  protected async upload(files: File[]): Promise<void> {
    this.error.set('');
    this.busy.set(true);
    try {
      let id = this.routeId() || this.savedId();
      if (!id) {
        id = await this.ensureSaved()();
        this.savedId.set(id);
      }
      for (const f of files) {
        await firstValueFrom(this.api.putRouteFile(id, servedName(f.name), f));
      }
      this.load(id);
    } catch (e) {
      const msg = e instanceof HttpErrorResponse ? e.error?.error : (e as Error)?.message;
      this.error.set(msg || $localize`:@@Files_upload_failed:The file could not be uploaded`);
    } finally {
      this.busy.set(false);
    }
  }

  protected update(f: RouteFile, change: { name?: string; contentType?: string }): void {
    const id = this.routeId() || this.savedId();
    const name = change.name?.trim();
    const type = change.contentType?.trim();
    if (!id || (name !== undefined && (!name || name === f.name)) || (type !== undefined && (!type || type === f.contentType))) {
      // Nothing changed, or a field emptied: the list shows the stored value back.
      this.files.update((l) => [...l]);
      return;
    }
    this.error.set('');
    this.busy.set(true);
    this.api.updateRouteFile(id, f.name, { name, contentType: type }).subscribe({
      next: (out) => {
        this.busy.set(false);
        // The index names a file: it follows the file it named.
        if (this.index() === f.name) this.set('index', out.name);
        this.load(id);
      },
      error: (e: HttpErrorResponse) => {
        this.busy.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Files_update_failed:The file could not be changed`);
        this.load(id);
      },
    });
  }

  protected remove(f: RouteFile): void {
    const id = this.routeId() || this.savedId();
    if (!id) return;
    this.busy.set(true);
    this.api.deleteRouteFile(id, f.name).subscribe({
      next: () => {
        this.busy.set(false);
        if (this.index() === f.name) this.set('index', '');
        this.load(id);
      },
      error: (e: HttpErrorResponse) => {
        this.busy.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Files_remove_failed:The file could not be removed`);
      },
    });
  }

  private load(id: string): void {
    this.api.listRouteFiles(id).subscribe({ next: (f) => this.files.set(f), error: () => this.files.set([]) });
  }
}

function encodePath(p: string): string {
  return p.split('/').map(encodeURIComponent).join('/');
}

// The name an upload is served as, from the name it had on disk: accents
// dropped, spaces and anything an address would have to escape turned into a
// dash. It is a starting point - the name is edited in the list.
export function servedName(name: string): string {
  return (
    name
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
      .replace(/[^A-Za-z0-9._~-]+/g, '-')
      .replace(/-{2,}/g, '-')
      .replace(/-?\.-?/g, '.')
      .replace(/^[-.]+|-+$/g, '') || 'file'
  );
}

// What a route in the files mode serves, most of the time.
const COMMON_TYPES = [
  'font/woff2',
  'font/woff',
  'font/ttf',
  'font/otf',
  'text/css; charset=utf-8',
  'text/javascript; charset=utf-8',
  'application/json',
  'text/html; charset=utf-8',
  'text/plain; charset=utf-8',
  'image/svg+xml',
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
  'image/x-icon',
  'application/pdf',
  'application/wasm',
  'application/xml',
  'text/csv; charset=utf-8',
  'application/zip',
  'application/octet-stream',
];
