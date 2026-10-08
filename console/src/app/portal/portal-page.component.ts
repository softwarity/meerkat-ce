import { Component, DestroyRef, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatSliderModule } from '@angular/material/slider';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, PortalEntry, PortalSubEntry, PortalConfig, Route, Settings } from '../api.service';
import { ModuleEditorComponent, ModuleDraft, ModuleFormData } from './module-editor.component';
import { LiveChangesService } from '../shared/live-changes.service';

// The navigation portal (PORTAL-01): build the header-or-rail bar the proxied
// applications wear, with a live mock-up beside the editor. On, the bar carries
// the account button and replaces the per-route user buttons across every UI
// route - filtered per caller by each module's route access.
//
// It rides /api/settings like the pages' own settings: the whole payload is
// kept and sent back with the portal changed, so a partial body cannot reset
// the rest.
@Component({
  selector: 'app-portal-page',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatCheckboxModule,
    MatIconModule,
    MatSliderModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    ModuleEditorComponent,
  ],
  styleUrl: './portal-page.component.scss',
  templateUrl: './portal-page.component.html',
})
export class PortalPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  // The module editor lives in a drawer on this page (not a modal): open it with
  // the entry to edit and a callback that applies the result to the tree.
  protected readonly editing = signal<ModuleFormData | null>(null);
  private applyEdit: ((m: ModuleDraft) => void) | null = null;
  // Which existing module the drawer edits (null while adding a new one), so the
  // quick actions know their target.
  private editingPath: { pi: number; ci: number | null } | null = null;

  protected readonly loading = signal(true);

  // The rendering mode. ONE catalogue below it, read in "links" and "portal"
  // alike: switching between them must not cost the list (PORTAL-03).
  protected readonly mode = signal<'none' | 'links' | 'portal'>('none');
  // The bar is drawn only in portal mode; the catalogue is edited in both of
  // the modes that have one.
  protected readonly hasCatalogue = computed(() => this.mode() !== 'none');
  protected readonly isBar = computed(() => this.mode() === 'portal');
  protected readonly layout = signal<'header' | 'rail'>('header');
  protected readonly side = signal<'left' | 'right'>('left');
  protected readonly display = signal<'both' | 'icon' | 'label'>('both');
  protected readonly showAppName = signal(false);
  // The branding logo in the bar: drawn by default, and this takes it out.
  protected readonly showLogo = signal(true);
  // How round it is drawn, 0 (as drawn) to 50 (circle).
  protected readonly logoRadius = signal(0);
  // The logo itself, so the choice is made looking at it.
  protected readonly brandLogo = signal('');
  protected readonly entries = signal<PortalEntry[]>([]);

  // Only enabled UI routes can wear the bar; those are what a module binds to.
  protected readonly uiRoutes = signal<Route[]>([]);

  private settings: Settings | null = null;

  // The live preview is the REAL bar, rendered in an iframe (it mutates the
  // page body, so it must be isolated) served by the admin plane in edit mode.
  // The console drives it with a draft over postMessage and gets back the entry
  // that was clicked.
  private readonly sanitizer = inject(DomSanitizer);
  protected readonly previewUrl = this.sanitizer.bypassSecurityTrustResourceUrl('/meerkat/portal-preview');
  private readonly frame = viewChild<ElementRef<HTMLIFrameElement>>('frame');
  private readonly frameReady = signal(false);
  // Which entry the preview has selected ("i" for a parent, "i/j" for a child).
  protected readonly selected = signal<string | null>(null);
  // The preview's width, to shrink the frame and watch the bar respond - the
  // overflow chevrons and the app-selector waffle appear when the tabs no longer
  // fit. 0 is full width. View-only, never saved.
  protected readonly previewWidth = signal(0);

  constructor() {
    this.load();
    // Somebody else's write (CONSOLE-13). OFFERED and not applied: this screen
    // is a form, and reloading it under somebody typing would throw their work
    // away for news they did not ask for. The same bargain the 409 does from
    // the other end - see stale.interceptor.ts.
    const live = inject(LiveChangesService);
    live.on('settings', (change) => live.offer(change, () => this.load()));
    // The catalogue is a list of ROUTES, so it also follows what the routing
    // plane does - a route that stopped serving pages is a module that no
    // longer has anything behind it.
    live.on('route', () => this.loadRoutes());

    // The preview: post the draft whenever it (or the selection) changes and the
    // frame is ready.
    effect(() => {
      const payload = this.buildPreview();
      if (!this.frameReady()) return;
      this.frame()?.nativeElement.contentWindow?.postMessage(
        { type: 'mk-portal-draft', payload },
        location.origin,
      );
    });

    // The preview talks back: "ready" to receive, and "select" when an entry is
    // clicked.
    const onMsg = (e: MessageEvent) => {
      if (e.origin !== location.origin) return;
      const m = e.data as { type?: string; id?: string };
      if (m?.type === 'mk-portal-ready') this.frameReady.set(true);
      else if (m?.type === 'mk-portal-select' && typeof m.id === 'string') this.onSelect(m.id);
    };
    window.addEventListener('message', onMsg);
    inject(DestroyRef).onDestroy(() => window.removeEventListener('message', onMsg));
  }

  // The payload the preview renders: the arrangement plus each module resolved
  // to what the bar shows (id, label, icon SVG), the selection highlighted.
  private buildPreview(): unknown {
    return {
      layout: this.layout(),
      side: this.side(),
      display: this.display(),
      showName: this.showAppName(),
      hideLogo: !this.showLogo(),
      logoRadius: this.logoRadius(),
      // Editing is only offered at full width: the narrower device previews are
      // there to WATCH the bar respond, not to click through it.
      edit: this.previewWidth() === 0,
      selected: this.selected(),
      // `parents` is the BAR's own word, not the catalogue's: the component
      // draws two surfaces, and what is a parent there is simply an entry
      // here. Renaming it in the payload silently emptied the preview.
      parents: this.entries().map((p, i) => ({
        id: String(i),
        label: this.label(p),
        homeLabel: this.homeLabel(p),
        icon: p.icon ?? '',
        description: p.description ?? '',
        badge: p.badge ?? '',
        disabled: !!p.disabled,
        children: (p.children ?? []).map((c, j) => ({
          id: `${i}/${j}`,
          label: this.label(c),
          icon: c.icon ?? '',
          description: c.description ?? '',
          badge: c.badge ?? '',
          disabled: !!c.disabled,
        })),
      })),
    };
  }

  // A click in the preview selects the entry and opens its editor. The id is
  // "i" for a parent, "i/j" for a child.
  protected onSelect(id: string): void {
    this.selected.set(id);
    const parts = id.split('/');
    const pi = parseInt(parts[0], 10);
    if (parts.length < 2) this.editParent(pi);
    else this.editChild(pi, parseInt(parts[1], 10));
  }

  // Load a stored portal into the signals - on first read and to roll back a
  // failed save.
  private loadRoutes(): void {
    this.api.listRoutes().subscribe({
      next: (rs) => this.uiRoutes.set(rs.filter((r) => r.isUi && r.enabled)),
    });
  }

  private load(): void {
    this.loadRoutes();
    this.api.branding().subscribe({
      next: (b) => this.brandLogo.set(b.logo || ''),
      error: () => this.brandLogo.set(''),
    });
    this.api.settings().subscribe({
      next: (s) => {
        this.settings = s;
        this.applyPortal(s.portal);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  private applyPortal(p: PortalConfig | undefined): void {
    const c = p ?? { mode: 'none' as const, layout: 'header', side: 'left', display: 'both' };
    this.mode.set(c.mode ?? 'none');
    this.layout.set(c.layout === 'rail' ? 'rail' : 'header');
    this.side.set(c.side === 'right' ? 'right' : 'left');
    this.display.set(c.display === 'icon' || c.display === 'label' ? c.display : 'both');
    this.showAppName.set(!!c.showAppName);
    this.showLogo.set(!c.hideLogo);
    this.logoRadius.set(c.logoRadius ?? 0);
    this.entries.set((c.entries ?? []).map(clone));
  }

  // Every change persists at once: a toggle IS the setting, and a module edit is
  // complete when its drawer saves - there is no page-level Save button. The
  // whole /api/settings payload rides along (the portal is one of its fields);
  // on failure the signals roll back to what the server still holds.
  private persist(): void {
    if (!this.settings) return;
    const before = this.settings;
    this.api.saveSettings({ ...this.settings, portal: this.current() }).subscribe({
      next: (s) => (this.settings = s),
      error: (err: unknown) => {
        this.applyPortal(before.portal);
        this.fail(err);
      },
    });
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      undefined,
      { duration: 4000 },
    );
  }

  private current(): PortalConfig {
    return {
      mode: this.mode(),
      layout: this.layout(),
      side: this.side(),
      display: this.display(),
      showAppName: this.showAppName(),
      hideLogo: !this.showLogo(),
      logoRadius: this.logoRadius(),
      entries: this.entries(),
    };
  }

  // ── display helpers ─────────────────────────────────────────────────────

  protected routeName(id: string): string {
    return this.uiRoutes().find((r) => r.id === id)?.name ?? id;
  }

  protected label(m: PortalEntry | PortalSubEntry): string {
    return (m.label || '').trim() || this.routeName(m.routeId);
  }

  protected homeLabel(m: PortalEntry): string {
    return (m.homeLabel || '').trim() || this.label(m);
  }

  // ── layout controls ─────────────────────────────────────────────────────

  protected setMode(v: 'none' | 'links' | 'portal'): void {
    this.mode.set(v);
    this.persist();
  }
  protected setLayout(v: 'header' | 'rail'): void {
    if (!v) return;
    this.layout.set(v);
    this.persist();
  }
  protected setSide(v: 'left' | 'right'): void {
    if (!v) return;
    this.side.set(v);
    this.persist();
  }
  protected setDisplay(v: 'both' | 'icon' | 'label'): void {
    if (!v) return;
    this.display.set(v);
    this.persist();
  }
  protected setShowAppName(v: boolean): void {
    this.showAppName.set(v);
    this.persist();
  }
  protected setShowLogo(v: boolean): void {
    this.showLogo.set(v);
    this.persist();
  }
  protected setLogoRadius(v: number): void {
    this.logoRadius.set(v);
    this.persist();
  }
  protected setPreviewWidth(v: number): void {
    this.previewWidth.set(v);
    // Leaving full width leaves the editor: close any open drawer so it cannot
    // linger over a preview that no longer accepts clicks.
    if (v !== 0) this.onClosed();
  }


  // ── adding ──────────────────────────────────────────────────────────────

  // The route's own name, which is what an entry falls back to when nobody
  // gave it a label - the LAST fallback, and the only one left.
  protected routeNameOf(id: string): string {
    return this.uiRoutes().find((r) => r.id === id)?.name ?? id;
  }

  // Reorder from the flat list. The bar reorders from its drawer; both write
  // the same array, because the order IS the catalogue's order.
  protected moveEntry(i: number, dir: -1 | 1): void {
    const j = i + dir;
    this.entries.update((es) => {
      if (j < 0 || j >= es.length) return es;
      const out = [...es];
      [out[i], out[j]] = [out[j], out[i]];
      return out;
    });
    this.persist();
  }

  protected addParent(): void {
    this.open(this.data.title.add, true, undefined, null, (m) => {
      this.entries.update((ps) => [...ps, { ...m, children: [] }]);
      this.selected.set(String(this.entries().length - 1)); // keep the new one selected
    });
  }

  protected editParent(i: number): void {
    this.open(this.data.title.edit, true, this.entries()[i], { pi: i, ci: null }, (m) =>
      this.entries.update((ps) => ps.map((x, j) => (j === i ? { ...x, ...m } : x))),
    );
  }

  private addChild(pi: number): void {
    this.open(this.data.title.addSub, false, undefined, null, (m) => {
      this.mutateChildren(pi, (cs) => [...cs, childFromDraft(m)]);
      const ci = (this.entries()[pi].children ?? []).length - 1;
      this.selected.set(`${pi}/${ci}`); // keep the new sub-module selected
    });
  }

  private editChild(pi: number, ci: number): void {
    const c = (this.entries()[pi].children ?? [])[ci];
    this.open(this.data.title.editSub, false, c, { pi, ci }, (m) =>
      this.mutateChildren(pi, (cs) => cs.map((x, j) => (j === ci ? { ...x, ...childFromDraft(m) } : x))),
    );
  }

  private mutateChildren(pi: number, fn: (cs: PortalSubEntry[]) => PortalSubEntry[]): void {
    this.entries.update((ps) =>
      ps.map((p, j) => (j === pi ? { ...p, children: fn(p.children ?? []) } : p)),
    );
  }

  // ── the drawer's quick actions (existing module) ─────────────────────────

  protected onMove(dir: -1 | 1): void {
    const p = this.editingPath;
    if (!p) return;
    if (p.ci === null) {
      const ni = p.pi + dir;
      this.entries.update((ps) => move(ps, p.pi, dir));
      this.persist();
      this.editParent(ni);
    } else {
      const nc = p.ci + dir;
      this.mutateChildren(p.pi, (cs) => move(cs, p.ci as number, dir));
      this.persist();
      this.editChild(p.pi, nc);
    }
  }

  // Moves the open module under another one, or detaches a sub-module to the
  // top level, keeping everything it carries; then reopens it where it now is.
  protected onRelocate(target: number | null): void {
    const p = this.editingPath;
    if (!p) return;
    const ps = this.entries();
    if (p.ci === null) {
      // A top-level module going under another one: not with children of its
      // own (the editor does not offer it), and without its home label, which
      // only a parent has.
      if (target === null || (ps[p.pi].children ?? []).length > 0) return;
      const { children: _children, homeLabel: _home, ...rest } = ps[p.pi];
      const moved: PortalSubEntry = { ...rest };
      const without = ps.filter((_, j) => j !== p.pi);
      const ti = target > p.pi ? target - 1 : target;
      this.entries.set(without.map((x, j) => (j === ti ? { ...x, children: [...(x.children ?? []), moved] } : x)));
      this.persist();
      this.editChild(ti, (this.entries()[ti].children ?? []).length - 1);
      return;
    }
    const child = (ps[p.pi].children ?? [])[p.ci];
    const ci = p.ci;
    const lifted = ps.map((x, j) => (j === p.pi ? { ...x, children: (x.children ?? []).filter((_, k) => k !== ci) } : x));
    if (target === null) {
      // Detached: a module of its own, right after the one it was under.
      lifted.splice(p.pi + 1, 0, { ...child, children: [] });
      this.entries.set(lifted);
      this.persist();
      this.editParent(p.pi + 1);
      return;
    }
    this.entries.set(lifted.map((x, j) => (j === target ? { ...x, children: [...(x.children ?? []), child] } : x)));
    this.persist();
    this.editChild(target, (this.entries()[target].children ?? []).length - 1);
  }

  protected onToggleDisabled(v: boolean): void {
    const p = this.editingPath;
    if (!p) return;
    if (p.ci === null) {
      this.entries.update((ps) => ps.map((x, j) => (j === p.pi ? { ...x, disabled: v } : x)));
    } else {
      this.mutateChildren(p.pi, (cs) => cs.map((x, j) => (j === p.ci ? { ...x, disabled: v } : x)));
    }
    this.persist();
  }

  protected onAddSub(): void {
    const p = this.editingPath;
    if (p && p.ci === null) this.addChild(p.pi);
  }

  protected onDelete(): void {
    const p = this.editingPath;
    if (!p) return;
    if (p.ci === null) this.entries.update((ps) => ps.filter((_, j) => j !== p.pi));
    else this.mutateChildren(p.pi, (cs) => cs.filter((_, j) => j !== p.ci));
    this.persist();
    this.selected.set(null); // the module is gone; nothing to keep selected
    this.onClosed();
  }

  // Open the editing drawer for a module (existing = a path, or null when new),
  // remembering where its result goes and highlighting it in the preview.
  private open(
    title: string,
    isParent: boolean,
    m: PortalEntry | PortalSubEntry | undefined,
    path: { pi: number; ci: number | null } | null,
    apply: (m: ModuleDraft) => void,
  ): void {
    this.applyEdit = apply;
    this.editingPath = path;
    this.selected.set(path ? (path.ci === null ? String(path.pi) : `${path.pi}/${path.ci}`) : null);
    let canUp = false;
    let canDown = false;
    if (path) {
      const siblings =
        path.ci === null ? this.entries().length : (this.entries()[path.pi].children ?? []).length;
      const idx = path.ci === null ? path.pi : path.ci;
      canUp = idx > 0;
      canDown = idx < siblings - 1;
    }
    // The other top-level modules, where this one could go.
    const parents = this.entries()
      .map((p, index) => ({ index, name: p.label || this.routeNameOf(p.routeId) }))
      .filter((p) => !path || p.index !== path.pi);
    this.editing.set({
      title,
      isParent,
      bar: this.isBar(),
      existing: path !== null,
      canUp,
      canDown,
      parents,
      under: path && path.ci !== null ? path.pi : null,
      above:
        path && path.ci === null && path.pi > 0 && !(this.entries()[path.pi].children ?? []).length
          ? path.pi - 1
          : null,
      hasChildren: !!path && path.ci === null && (this.entries()[path.pi].children ?? []).length > 0,
      routes: this.uiRoutes(),
      module: {
        routeId: m?.routeId ?? '',
        icon: m?.icon ?? '',
        label: m?.label ?? '',
        homeLabel: (m as PortalEntry | undefined)?.homeLabel ?? '',
        description: m?.description ?? '',
        disabled: !!m?.disabled,
      } satisfies ModuleDraft,
    });
  }

  protected onSaved(m: ModuleDraft): void {
    this.applyEdit?.(m);
    this.onClosed();
    this.persist();
  }

  protected onClosed(): void {
    this.applyEdit = null;
    this.editingPath = null;
    this.editing.set(null);
    // Keep `selected` as it is: the last-edited module stays highlighted and the
    // preview stays on it, instead of snapping back to the first one every time
    // the drawer closes. onDelete clears it (the module is gone).
  }

  // Titles for the module dialog, kept together so the four cases read alike.
  protected readonly data = {
    title: {
      add: $localize`:@@Portal_add_module:Add a module`,
      edit: $localize`:@@Portal_edit_module:Edit module`,
      addSub: $localize`:@@Portal_add_submodule:Add a sub-module`,
      editSub: $localize`:@@Portal_edit_submodule:Edit sub-module`,
    },
  };
}

function clone(p: PortalEntry): PortalEntry {
  return { ...p, children: (p.children ?? []).map((c) => ({ ...c })) };
}

// The editor's draft is shared by parents and children, so it carries the
// parent-only `homeLabel`. A child must NOT ship it: the settings API rejects
// unknown fields, and one stray key would 400 the whole save (silently dropping
// the sub-module). Keep only the fields a PortalSubEntry owns.
function childFromDraft(m: ModuleDraft): PortalSubEntry {
  return {
    routeId: m.routeId,
    icon: m.icon,
    label: m.label,
    description: m.description,
    disabled: m.disabled,
  };
}

function move<T>(arr: T[], i: number, dir: -1 | 1): T[] {
  const j = i + dir;
  if (j < 0 || j >= arr.length) return arr;
  const out = [...arr];
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}
