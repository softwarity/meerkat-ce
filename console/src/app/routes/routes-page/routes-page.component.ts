import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatInputModule } from '@angular/material/input';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router } from '@angular/router';
import { LiveTopic, LivewireClient } from '@softwarity/livewire';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { forkJoin } from 'rxjs';
import { ApiService, CatalogEntry, Edition, Maintenance, Route, RouteHealth } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { ChangeRow, LiveChangesService } from '../../shared/live-changes.service';
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
    MatSelectModule,
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
  // UI routes are a different job from the rest - they are the ones with a
  // button, a theme and a page someone opens - and on a list this long they
  // are hard to pick out of the services around them.
  protected readonly kind = signal<'all' | 'ui' | 'service'>('all');
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLowerCase();
    const kind = this.kind();
    return this.routes().filter((r) => {
      if (kind === 'ui' && !r.isUi) return false;
      if (kind === 'service' && r.isUi) return false;
      if (!q) return true;
      const paths = (r.predicates ?? []).flatMap((p) => (p.args?.['patterns'] as string[]) ?? []);
      return [r.name, r.upstream ?? '', ...paths].some((s) => s.toLowerCase().includes(q));
    });
  });
  // Where the applications answer. Asked of the gateway, never worked out from
  // the console's own address: the two planes are two origins, and a published
  // port or an ingress sits between them.
  private readonly editionRes = httpResource<Edition>(() => '/api/edition');
  protected readonly dataOrigin = computed(() => this.editionRes.value()?.dataOrigin ?? '');
  protected readonly tracingOn = signal(false);

  private readTracing(): void {
    this.api.telemetrySetting().subscribe({
      next: (t) => this.tracingOn.set(t.enabled && t.traces),
      error: () => {},
    });
  }
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
    // A route's target went up or down (SVC-04): the gateway checks them in
    // the background and says so here, and the hearts are read again.
    new LiveTopic(inject(LivewireClient), 'route-health')
      .open(null)
      .pipe(takeUntilDestroyed())
      .subscribe(() =>
        this.api.routeHealth().subscribe({ next: (h) => this.health.set(h), error: () => {} }),
      );
    // Whether traces leave at all, for the mark beside each traced route. Read
    // again when a setting moves, silently: it changes a mark, not a form, so
    // there is nothing to ask anybody.
    this.readTracing();
    inject(LiveChangesService).on('settings', () => this.readTracing());
    // Somebody else's write (CONSOLE-13): the list follows a route created,
    // renamed, reordered or deleted anywhere, and the health badge with it -
    // a route that stopped compiling is exactly the news this screen is for.
    //
    // With ONE exception, and it is the whole reason this is not two lines: the
    // list is what feeds the open editor, so reloading it while somebody is
    // typing in that route would reseed their draft and take the typing with
    // it. In that case the list waits and the editor says so instead. A write
    // to ANOTHER route is no such problem, which is what the event's own
    // targetId answers.
    inject(LiveChangesService).on('route', (change) => {
      const open = this.editingRoute();
      // A DELETION of the route in the drawer wins over everything below,
      // unsaved work included: there is nothing left to edit, and an editor
      // sitting on a row that no longer exists lets somebody keep working
      // towards a 409. It says who did it and closes.
      if (open && change.targetId === open.id && change.action?.endsWith('.delete')) {
        this.deletedBy = change.actor ?? '';
        this.refresh();
        return;
      }
      if (open && this.editorDirty() && (!change.targetId || change.targetId === open.id)) {
        // Is it somebody else's? The event says WHO by account name, and two
        // tabs of one operator are the very case this feature is about - so
        // the account is no answer. The REVISION is: what this screen holds is
        // what it last saved or last read, and a write that left it where it
        // is is this screen's own. Asked of the gateway rather than guessed,
        // because "I saved a second ago" is a race and a revision is a fact.
        this.api.getRoute(open.id).subscribe({
          next: (fresh) => {
            if (fresh.rev === open.rev) return; // ours: nothing to say
            this.changedUnderEdit.set(change);
          },
          error: () => this.changedUnderEdit.set(change),
        });
        return;
      }
      this.apply(change);
    });
  }

  // What one write costs this screen.
  //
  // A write NAMES the row it touched, so a save on one route costs one GET
  // rather than the whole list - which is what matters on the installations
  // this is written for, where the list is not five rows. A write that names
  // nothing (a reorder, a configuration import, a gap in the journal after a
  // laptop woke up) moved rows nobody named, and there the list is the answer.
  //
  // A creation and a deletion go through the list too, deliberately: where a
  // new route lands is decided by its order, and the gateway is what knows it.
  private apply(change: ChangeRow): void {
    const id = change.targetId;
    if (!id || !change.action?.endsWith('.update')) {
      this.refresh();
      return;
    }
    if (!this.routes().some((r) => r.id === id)) {
      // Not on this screen - a route the filter hides, or one this reader has
      // never listed. Nothing to reload, and asking for it would be asking the
      // gateway about a row nobody is showing.
      return;
    }
    this.api.getRoute(id).subscribe({
      next: (fresh) => {
        // A NEW array holding a NEW object. The table takes its rows through an
        // input signal, and this application is zoneless: mutating the route in
        // place, or writing back the array this signal already holds, changes
        // what is on screen for nobody - the reference is what is compared, and
        // there is no zone to notice the rest.
        this.routes.update((list) => list.map((r) => (r.id === fresh.id ? fresh : r)));
        // The badge follows the same write: a route that stopped compiling is
        // exactly the news this screen is for.
        this.api.routeHealth().subscribe({ next: (h) => this.health.set(h), error: () => {} });
      },
      // Gone between the write and this read - or refused. The list settles it.
      error: () => this.refresh(),
    });
  }

  // The list and the health badge, as the gateway has them now.
  private refresh(): void {
    this.load(true);
    this.api.routeHealth().subscribe({ next: (h) => this.health.set(h), error: () => {} });
  }

  // What somebody else did to the route being edited, while it holds unsaved
  // changes. Cleared by whatever settles it: reloading, saving, or closing.
  protected readonly changedUnderEdit = signal<ChangeRow | null>(null);

  // The reader asked for it, so the editor IS reseeded - that is what Reload
  // means here, and it is why the banner does not do it on its own.
  protected reloadUnderEdit(): void {
    this.changedUnderEdit.set(null);
    this.load(true, true);
    this.api.routeHealth().subscribe({ next: (h) => this.health.set(h), error: () => {} });
  }

  protected openEdit(route: Route): void {
    this.changedUnderEdit.set(null);
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
    // Whatever was waiting on the open editor is settled by leaving it: the
    // list reloads on its own from here.
    this.changedUnderEdit.set(null);
    void this.router.navigate(['/infra/routes']);
    this.refresh();
  }

  protected changeSection(s: string): void {
    const e = this.editing();
    if (e && e !== 'new') void this.router.navigate(['/infra/routes', e.id, s]);
  }

  // quiet skips the loading state, which is what a reload nobody asked for
  // needs: a push that replaced the list with a spinner would take the screen
  // away from somebody reading it, to show them the same list a moment later.
  load(quiet = false, adopt = false): void {
    if (!quiet) this.loading.set(true);
    forkJoin({ catalog: this.api.catalog(), routes: this.api.listRoutes() }).subscribe({
      next: ({ catalog, routes }) => {
        this.catalog.set(catalog);
        this.routes.set(this.mindTheOpenEditor(routes, adopt));
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  // A fresh list, minus what it would break in the drawer.
  //
  // The editor reads the route out of THIS list, so replacing the list reseeds
  // its draft - and a reload nobody asked for would throw away somebody's
  // typing because a colleague created an unrelated route. So while the editor
  // holds unsaved work, the open route keeps the object the editor was seeded
  // from, and the news is put in the banner instead.
  //
  // The route being edited DISAPPEARING is the other case, and it is not about
  // dirtiness: a drawer that closes by itself while somebody is reading it owes
  // them a word about why.
  private mindTheOpenEditor(fresh: Route[], adopt: boolean): Route[] {
    const open = this.editingRoute();
    if (!open) return fresh;
    const now = fresh.find((r) => r.id === open.id);
    if (!now) {
      this.sayDeleted(open, this.deletedBy);
      return fresh;
    }
    if (!this.editorDirty() || adopt) return fresh;
    if (JSON.stringify(now) !== JSON.stringify(open)) {
      // Changed, and nothing told us by whom - an import, or a gap in the
      // journal. The banner says so without naming anyone.
      this.changedUnderEdit.update((known) => known ?? { kind: 'route', at: Date.now() / 1000 } as ChangeRow);
    }
    return fresh.map((r) => (r.id === open.id ? open : r));
  }

  // Who deleted the route under the editor, when the write named them.
  private deletedBy = '';

  private sayDeleted(open: Route, by: string): void {
    this.deletedBy = '';
    this.changedUnderEdit.set(null);
    this.snack.open(
      by
        ? $localize`:@@NAME_deleted_this_route_while_you_were_editing:${by}:NAME: deleted the route "${open.name}:ROUTE:" while you were editing it. Your changes were not saved.`
        : $localize`:@@This_route_was_deleted_while_you_were_editing:The route "${open.name}:ROUTE:" was deleted while you were editing it. Your changes were not saved.`,
      undefined,
      { duration: 10000 },
    );
    void this.router.navigate(['/infra/routes']);
  }

  // Save keeps the drawer OPEN: the URL stays (or gains the fresh id after a
  // creation), the reloaded list rebinds the fresh route into the editor.
  onSaved(saved: Route): void {
    this.snack.open($localize`:@@Route_NAME_saved_and_applied:Route "${saved.name}:NAME:" saved and applied`, undefined, { duration: 2500 });
    if (this.editing() === 'new') {
      void this.router.navigate(['/infra/routes', saved.id, 'target'], { replaceUrl: true });
    }
    // Saved, so there is nothing left to warn about: this copy IS the current
    // one now - the gateway refused it otherwise (409, and the interceptor says
    // so).
    //
    // ADOPTING, and that is what was missing: the draft is still "dirty"
    // against the list until the list carries what was just saved, so a plain
    // reload took the guard below and KEPT the old object - the draft was never
    // reseeded, dirty stayed true for ever, and every write from then on raised
    // the banner. Two saves in a row and the editor was unusable.
    this.changedUnderEdit.set(null);
    this.load(false, true);
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
