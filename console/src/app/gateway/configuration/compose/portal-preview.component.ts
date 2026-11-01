import { Component, DestroyRef, ElementRef, afterNextRender, computed, effect, inject, input, signal, untracked, viewChild } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { ApiService } from '../../../api.service';

// The portal a side's settings describe, drawn by the REAL bar: the same
// frame the portal editor uses (`/meerkat/portal-preview`, served by the
// admin plane in edit mode), fed the catalogue over postMessage. One per
// column under the portal setting's fields shows what a choice changes
// better than a list of JSON chips ever will.
//
// The payload is the editor's (buildPreview in portal-page.component.ts):
// renaming a field of it silently empties the bar. Edit is on for the one
// thing it gives: a click selects an entry, the bar says which, and a
// container's modules show, which is how the sub-routes are seen.
type Entry = {
  routeId?: string;
  icon?: string;
  label?: string;
  description?: string;
  disabled?: boolean;
  special?: string;
  children?: Entry[];
};

@Component({
  selector: 'app-portal-preview',
  styles: [
    `
      :host {
        display: block;
        border-radius: 8px;
        overflow: hidden;
        border: 1px solid var(--mat-sys-outline-variant);
        background: var(--mat-sys-surface-container-lowest);
      }
      /* A miniature: the bar laid out at a width where its tabs show, then
         scaled to the column, the host as tall as what the scale leaves. */
      .mini {
        position: relative;
        overflow: hidden;
      }
      iframe {
        display: block;
        position: absolute;
        top: 0;
        left: 0;
        border: 0;
        transform-origin: top left;
      }
      .none {
        padding: 10px 12px;
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .none b {
        font-weight: 500;
      }
    `,
  ],
  template: `
    @if (portal(); as p) {
      @if (p['mode'] === 'portal') {
        <div class="mini" [style.height.px]="height() * scale()">
          <iframe
            #frame
            [src]="url"
            [style.width.px]="WIDE"
            [style.height.px]="height()"
            [style.transform]="'scale(' + scale() + ')'"
            title="Portal preview"
            i18n-title="@@Portal_preview"
          ></iframe>
        </div>
      } @else {
        <div class="none"><b>{{ name() }}</b>: <ng-container i18n="@@Portal_preview_off">no portal bar (mode {{ p['mode'] ?? 'none' }}).</ng-container></div>
      }
    } @else {
      <div class="none"><b>{{ name() }}</b>: <ng-container i18n="@@Portal_preview_absent">no portal setting.</ng-container></div>
    }
  `,
})
export class PortalPreviewComponent {
  // The portal setting as the compose screen reads it, or nothing.
  readonly portal = input<Record<string, unknown> | undefined>(undefined);
  // What the side is called, for a side that has no bar to show.
  readonly name = input('');

  // The width the bar is laid out at: a laptop's, where a container's row of
  // modules shows in full (narrower, the bar folds it into a menu, as it does
  // on a small window), and the ratio that fits it in the host.
  protected readonly WIDE = 1280;
  protected readonly scale = signal(0.25);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  private readonly api = inject(ApiService);
  protected readonly url = inject(DomSanitizer).bypassSecurityTrustResourceUrl('/meerkat/portal-preview');
  private readonly frame = viewChild<ElementRef<HTMLIFrameElement>>('frame');
  private readonly ready = signal(false);
  // The entry the bar has selected ("i" or "i/j"), or its first one.
  private readonly selected = signal<string | null>(null);
  // Route names, for an entry without a label: what the bar would show.
  private readonly routeNames = signal(new Map<string, string>());

  // A page's silhouette: the bar on top and room under it, so the miniature
  // reads as the portal and not as a line. A rail is vertical: its head, one
  // 64px item per entry, and its foot (the launcher and the scheme button).
  protected readonly height = computed(() => {
    const p = this.portal();
    if (p?.['layout'] !== 'rail') return 360;
    const entries = Array.isArray(p['entries']) ? (p['entries'] as Entry[]) : [];
    const i = Number((this.selected() ?? '0').split('/')[0]);
    const open = entries[i]?.children?.length ?? 0;
    return Math.max(360, 216 + 64 * (entries.length + open));
  });

  private readonly payload = computed(() => {
    const p = this.portal();
    if (!p) return null;
    const names = this.routeNames();
    const label = (m: Entry) => (m.label || '').trim() || (m.special ? 'Notifications' : names.get(m.routeId ?? '') || m.routeId || '');
    const entries = Array.isArray(p['entries']) ? (p['entries'] as Entry[]) : [];
    return {
      layout: p['layout'] ?? 'header',
      side: p['side'] ?? 'left',
      display: p['display'] ?? 'both',
      showName: !!p['showAppName'],
      hideLogo: !!p['hideLogo'],
      logoRadius: typeof p['logoRadius'] === 'number' ? p['logoRadius'] : 0,
      edit: true,
      selected: this.selected(),
      parents: entries.map((e, i) => ({
        id: String(i),
        label: label(e),
        icon: e.icon ?? '',
        description: e.description ?? '',
        disabled: !!e.disabled,
        off: false,
        ...(e.special ? { badge: '3' } : {}),
        children: (e.children ?? []).map((c, j) => ({
          id: `${i}/${j}`,
          label: label(c),
          icon: c.icon ?? '',
          description: c.description ?? '',
          disabled: !!c.disabled,
          off: false,
          ...(c.special ? { badge: '3' } : {}),
        })),
      })),
    };
  });

  constructor() {
    const destroy = inject(DestroyRef);
    // The ratio follows the column: the loom is resized, the rail folds.
    afterNextRender(() => {
      const fit = () => this.scale.set(Math.max(0.05, this.host.nativeElement.clientWidth / this.WIDE));
      fit();
      const ro = new ResizeObserver(fit);
      ro.observe(this.host.nativeElement);
      destroy.onDestroy(() => ro.disconnect());
    });
    this.api.listRoutes().subscribe({
      next: (list) => this.routeNames.set(new Map(list.map((r) => [r.id, r.name]))),
      error: () => undefined,
    });
    // A new frame has to say it is ready before it is posted anything.
    effect(() => {
      this.frame();
      untracked(() => this.ready.set(false));
    });
    // A new side forgets the selection.
    effect(() => {
      this.portal();
      untracked(() => this.selected.set(null));
    });
    effect(() => {
      const payload = this.payload();
      if (!this.ready() || !payload) return;
      this.frame()?.nativeElement.contentWindow?.postMessage({ type: 'mk-portal-draft', payload }, location.origin);
    });
    const onMsg = (e: MessageEvent) => {
      if (e.origin !== location.origin) return;
      if (e.source !== this.frame()?.nativeElement.contentWindow) return;
      const m = e.data as { type?: string; id?: string };
      if (m?.type === 'mk-portal-ready') this.ready.set(true);
      else if (m?.type === 'mk-portal-select' && typeof m.id === 'string') this.selected.set(m.id);
    };
    window.addEventListener('message', onMsg);
    destroy.onDestroy(() => window.removeEventListener('message', onMsg));
  }
}
