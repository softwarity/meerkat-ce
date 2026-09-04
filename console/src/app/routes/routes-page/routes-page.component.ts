import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { forkJoin } from 'rxjs';
import { ApiService, CatalogEntry, Maintenance, Route, RouteHealth } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { RouteEditorComponent } from '../route-editor/route-editor.component';
import { RouteProbeDialogComponent } from '../route-probe-dialog.component';
import { RoutesTableComponent } from '../routes-table/routes-table.component';
import { GlobalPanelComponent } from '../global-panel/global-panel.component';
import { SigningKeysPanelComponent } from '../signing-keys/signing-keys-panel.component';

@Component({
  selector: 'app-routes-page',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSidenavModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RoutesTableComponent,
    RouteEditorComponent,
    FormFieldComponent,
    GlobalPanelComponent,
    SigningKeysPanelComponent,
  ],
  templateUrl: './routes-page.component.html',
  styleUrl: './routes-page.component.scss',
})
export class RoutesPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);
  private readonly ar = inject(ActivatedRoute);

  protected readonly loading = signal(true);
  protected readonly routes = signal<Route[]>([]);
  // 157 routes on a real estate: the list is unusable without this. Matched on
  // what someone actually knows about a route - its name, where it sends, and
  // the paths it answers to, which is usually what is being hunted for.
  protected readonly query = signal('');
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLowerCase();
    if (!q) return this.routes();
    return this.routes().filter((r) => {
      const paths = (r.predicates ?? []).flatMap((p) => (p.args?.['patterns'] as string[]) ?? []);
      return [r.name, r.upstream ?? '', ...paths].some((s) => s.toLowerCase().includes(q));
    });
  });
  protected readonly catalog = signal<CatalogEntry[]>([]);

  // The Global drawer, and the one piece of its state this page needs on its
  // own banner: an operator must see that everything is down without going
  // looking for it.
  protected readonly globalOpen = signal(false);
  protected readonly keysOpen = signal(false);
  protected readonly maintenance = signal<Maintenance | null>(null);
  // What the gateway has actually seen from each upstream (SVC-04). Read once
  // with the list and refreshed with it: this screen shows what is configured,
  // and it now shows what answers.
  protected readonly health = signal<Record<string, RouteHealth>>({});

  // The URL drives the drawer (F5-proof): /routes/new = creating,
  // /routes/:id/:section = editing that route on that section.
  private readonly params = toSignal(this.ar.paramMap);
  private readonly urlSegs = toSignal(this.ar.url);
  protected readonly editing = computed<Route | 'new' | null>(() => {
    if (this.urlSegs()?.some((s) => s.path === 'new')) return 'new';
    const id = this.params()?.get('id');
    if (!id) return null;
    return this.routes().find((r) => r.id === id) ?? null;
  });
  protected readonly editingRoute = computed(() => {
    const e = this.editing();
    return e === null || e === 'new' ? null : e;
  });
  protected readonly section = computed(() => this.params()?.get('section') ?? 'target');

  // A drawer that closes on a stray click outside takes the work with it. While
  // the editor holds unsaved changes the backdrop (and Escape) stop closing it,
  // and the way out is the drawer's own Close, which asks.
  private readonly editor = viewChild(RouteEditorComponent);
  protected readonly editorDirty = computed(() => this.editor()?.dirty() ?? false);

  constructor() {
    this.load();
    // Read once, for the banner. A failure is silence rather than an error:
    // the routes are what this page is for, and a badge that could not be
    // fetched must not put a red box in front of them.
    this.api.maintenance().subscribe({ next: (m) => this.maintenance.set(m), error: () => {} });
    this.api.routeHealth().subscribe({ next: (h) => this.health.set(h), error: () => {} });
  }

  protected openEdit(route: Route): void {
    void this.router.navigate(['/infra/routes', route.id, 'target']);
  }

  protected openNew(): void {
    void this.router.navigate(['/infra/routes', 'new']);
  }

  // What applies to every route at once (maintenance, the body-rewriting
  // ceiling, the signing keys). Not URL-driven, unlike the editor: it holds no
  // selection worth surviving an F5, and a maintenance switch behind a
  // bookmarkable URL is a switch somebody flips from a stale tab.
  protected openGlobal(): void {
    this.keysOpen.set(false);
    this.globalOpen.set(true);
  }

  // The signing keys, in a drawer of their own. The drawer shows one thing at
  // a time, so opening either closes the other rather than stacking them.
  protected openKeys(): void {
    this.globalOpen.set(false);
    this.keysOpen.set(true);
  }

  // A route named by the signing keys: land on its Identity section, which is
  // where the algorithm that put it in that list is chosen. The drawer swaps
  // content rather than stacking - it shows one thing at a time.
  protected openIdentity(routeId: string): void {
    this.keysOpen.set(false);
    void this.router.navigate(['/infra/routes', routeId, 'identity']);
  }

  // Route probe (ROUTE-15): compose a fictional request, see which route takes
  // it. The dialog shows the page's route list from the start.
  protected openProbe(): void {
    // maxWidth too: Material clamps dialogs to 560px otherwise.
    this.dialog.open(RouteProbeDialogComponent, {
      width: '760px',
      maxWidth: '760px',
      data: { routes: this.routes() },
      restoreFocus: true,
    });
  }

  // The drawer's own close: whichever content it is showing.
  protected async closeDrawer(): Promise<void> {
    if (this.globalOpen()) {
      this.globalOpen.set(false);
      return;
    }
    if (this.keysOpen()) {
      this.keysOpen.set(false);
      return;
    }
    await this.closeEditor();
  }

  protected async closeEditor(): Promise<void> {
    if (this.editing() === null) return;
    // The backdrop no longer closes on unsaved work, so this is the only way
    // out - and the only place left to warn before the changes are dropped.
    if (this.editorDirty()) {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Discard_changes_to_this_route:Discard the changes to this route?`,
        message: $localize`:@@Discard_changes_message:They have not been saved and cannot be recovered.`,
        confirmLabel: $localize`:@@Discard:Discard`,
        danger: true,
      });
      if (!ok) return;
    }
    void this.router.navigate(['/infra/routes']);
  }

  protected changeSection(s: string): void {
    const e = this.editing();
    if (e && e !== 'new') void this.router.navigate(['/infra/routes', e.id, s]);
  }

  load(): void {
    this.loading.set(true);
    forkJoin({ catalog: this.api.catalog(), routes: this.api.listRoutes() }).subscribe({
      next: ({ catalog, routes }) => {
        this.catalog.set(catalog);
        this.routes.set(routes);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  // Save keeps the drawer OPEN: the URL stays (or gains the fresh id after a
  // creation), the reloaded list rebinds the fresh route into the editor.
  onSaved(saved: Route): void {
    this.snack.open($localize`:@@Route_NAME_saved_and_applied:Route "${saved.name}:NAME:" saved and applied`, undefined, { duration: 2500 });
    if (this.editing() === 'new') {
      void this.router.navigate(['/infra/routes', saved.id, 'target'], { replaceUrl: true });
    }
    this.load();
  }

  // Persist a drag-reorder: apply optimistically, then save (order is
  // significant - first-match-wins). On failure, reload server truth.
  onReorder(ids: string[]): void {
    const byId = new Map(this.routes().map((r) => [r.id, r]));
    this.routes.set(ids.map((id) => byId.get(id)!).filter(Boolean));
    this.api.reorderRoutes(ids).subscribe({
      error: () => {
        this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 });
        this.load();
      },
    });
  }

  toggleEnabled(route: Route): void {
    this.api.putRoute({ ...route, enabled: !route.enabled }).subscribe({
      next: (saved) => {
        this.snack.open(
          saved.enabled
            ? $localize`:@@Route_NAME_enabled:Route "${saved.name}:NAME:" enabled`
            : $localize`:@@Route_NAME_disabled:Route "${saved.name}:NAME:" disabled`,
          undefined,
          { duration: 2500 },
        );
        this.load();
      },
      error: () => this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 }),
    });
  }

  // A copy of everything but the identity, DISABLED and placed right after
  // the original. Two routes matching the same paths is the ordinary way to
  // try a variant - a different upstream, another organisation's access - and
  // typing the whole thing again to compare two of them is how a difference
  // nobody meant creeps in.
  duplicate(route: Route): void {
    const name = `${route.name}-copy`;
    this.api
      // A fresh identity, like the editor mints for a new route: PUT is
      // keyed on the id, so reusing the original's would be an edit.
      .putRoute({ ...route, id: crypto.randomUUID(), name, enabled: false, order: route.order + 1 })
      .subscribe({
        next: (saved) => {
          this.snack.open(
            $localize`:@@Route_NAME_duplicated:Route "${saved.name}:NAME:" created, disabled`,
            undefined,
            { duration: 3000 },
          );
          this.load();
          void this.router.navigate(['/infra/routes', saved.id, 'target']);
        },
        error: () => this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 }),
      });
  }

  async remove(route: Route): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_route_NAME:Delete route "${route.name}:NAME:"?`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteRoute(route.id).subscribe({
      next: () => {
        this.snack.open($localize`:@@Route_NAME_deleted:Route "${route.name}:NAME:" deleted`, undefined, { duration: 2500 });
        this.load();
      },
      error: () => this.snack.open($localize`:@@Delete_failed:Delete failed`, undefined, { duration: 3000 }),
    });
  }
}
