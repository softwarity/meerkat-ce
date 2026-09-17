import { Component, DestroyRef, ElementRef, computed, effect, inject, signal, viewChild } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, ModuleChild, ModuleParent, PortalConfig, Route, Settings } from '../api.service';
import { ModuleEditorComponent, ModuleDraft, ModuleFormData } from './module-editor.component';

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
    MatIconModule,
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

  protected readonly enabled = signal(false);
  protected readonly layout = signal<'header' | 'rail'>('header');
  protected readonly side = signal<'left' | 'right'>('left');
  protected readonly display = signal<'both' | 'icon' | 'label'>('both');
  protected readonly showAppName = signal(false);
  protected readonly parents = signal<ModuleParent[]>([]);

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
    this.api.listRoutes().subscribe({
      next: (rs) => this.uiRoutes.set(rs.filter((r) => r.isUi && r.enabled)),
    });
    this.api.settings().subscribe({
      next: (s) => {
        this.settings = s;
        this.applyPortal(s.portal);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });

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
      // Editing is only offered at full width: the narrower device previews are
      // there to WATCH the bar respond, not to click through it.
      edit: this.previewWidth() === 0,
      selected: this.selected(),
      parents: this.parents().map((p, i) => ({
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
  private applyPortal(p: PortalConfig | undefined): void {
    const c = p ?? { enabled: false, layout: 'header', side: 'left', display: 'both' };
    this.enabled.set(!!c.enabled);
    this.layout.set(c.layout === 'rail' ? 'rail' : 'header');
    this.side.set(c.side === 'right' ? 'right' : 'left');
    this.display.set(c.display === 'icon' || c.display === 'label' ? c.display : 'both');
    this.showAppName.set(!!c.showAppName);
    this.parents.set((c.parents ?? []).map(clone));
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
      enabled: this.enabled(),
      layout: this.layout(),
      side: this.side(),
      display: this.display(),
      showAppName: this.showAppName(),
      parents: this.parents(),
    };
  }

  // ── display helpers ─────────────────────────────────────────────────────

  protected routeName(id: string): string {
    return this.uiRoutes().find((r) => r.id === id)?.name ?? id;
  }

  protected label(m: ModuleParent | ModuleChild): string {
    return (m.label || '').trim() || this.routeName(m.routeId);
  }

  protected homeLabel(m: ModuleParent): string {
    return (m.homeLabel || '').trim() || this.label(m);
  }

  // ── layout controls ─────────────────────────────────────────────────────

  protected setEnabled(v: boolean): void {
    this.enabled.set(v);
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
  protected setPreviewWidth(v: number): void {
    this.previewWidth.set(v);
    // Leaving full width leaves the editor: close any open drawer so it cannot
    // linger over a preview that no longer accepts clicks.
    if (v !== 0) this.onClosed();
  }


  // ── adding ──────────────────────────────────────────────────────────────

  protected addParent(): void {
    this.open(this.data.title.add, true, undefined, null, (m) => {
      this.parents.update((ps) => [...ps, { ...m, children: [] }]);
      this.selected.set(String(this.parents().length - 1)); // keep the new one selected
    });
  }

  private editParent(i: number): void {
    this.open(this.data.title.edit, true, this.parents()[i], { pi: i, ci: null }, (m) =>
      this.parents.update((ps) => ps.map((x, j) => (j === i ? { ...x, ...m } : x))),
    );
  }

  private addChild(pi: number): void {
    this.open(this.data.title.addSub, false, undefined, null, (m) => {
      this.mutateChildren(pi, (cs) => [...cs, childFromDraft(m)]);
      const ci = (this.parents()[pi].children ?? []).length - 1;
      this.selected.set(`${pi}/${ci}`); // keep the new sub-module selected
    });
  }

  private editChild(pi: number, ci: number): void {
    const c = (this.parents()[pi].children ?? [])[ci];
    this.open(this.data.title.editSub, false, c, { pi, ci }, (m) =>
      this.mutateChildren(pi, (cs) => cs.map((x, j) => (j === ci ? { ...x, ...childFromDraft(m) } : x))),
    );
  }

  private mutateChildren(pi: number, fn: (cs: ModuleChild[]) => ModuleChild[]): void {
    this.parents.update((ps) =>
      ps.map((p, j) => (j === pi ? { ...p, children: fn(p.children ?? []) } : p)),
    );
  }

  // ── the drawer's quick actions (existing module) ─────────────────────────

  protected onMove(dir: -1 | 1): void {
    const p = this.editingPath;
    if (!p) return;
    if (p.ci === null) {
      const ni = p.pi + dir;
      this.parents.update((ps) => move(ps, p.pi, dir));
      this.persist();
      this.editParent(ni);
    } else {
      const nc = p.ci + dir;
      this.mutateChildren(p.pi, (cs) => move(cs, p.ci as number, dir));
      this.persist();
      this.editChild(p.pi, nc);
    }
  }

  protected onToggleDisabled(v: boolean): void {
    const p = this.editingPath;
    if (!p) return;
    if (p.ci === null) {
      this.parents.update((ps) => ps.map((x, j) => (j === p.pi ? { ...x, disabled: v } : x)));
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
    if (p.ci === null) this.parents.update((ps) => ps.filter((_, j) => j !== p.pi));
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
    m: ModuleParent | ModuleChild | undefined,
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
        path.ci === null ? this.parents().length : (this.parents()[path.pi].children ?? []).length;
      const idx = path.ci === null ? path.pi : path.ci;
      canUp = idx > 0;
      canDown = idx < siblings - 1;
    }
    this.editing.set({
      title,
      isParent,
      existing: path !== null,
      canUp,
      canDown,
      routes: this.uiRoutes(),
      module: {
        routeId: m?.routeId ?? '',
        icon: m?.icon ?? '',
        label: m?.label ?? '',
        homeLabel: (m as ModuleParent | undefined)?.homeLabel ?? '',
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

function clone(p: ModuleParent): ModuleParent {
  return { ...p, children: (p.children ?? []).map((c) => ({ ...c })) };
}

// The editor's draft is shared by parents and children, so it carries the
// parent-only `homeLabel`. A child must NOT ship it: the settings API rejects
// unknown fields, and one stray key would 400 the whole save (silently dropping
// the sub-module). Keep only the fields a ModuleChild owns.
function childFromDraft(m: ModuleDraft): ModuleChild {
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
