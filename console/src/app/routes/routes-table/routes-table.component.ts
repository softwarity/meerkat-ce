import { CdkDragDrop, DragDropModule, moveItemInArray } from '@angular/cdk/drag-drop';
import { booleanAttribute, computed, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { Route, RouteHealth, RuntimeImage } from '../../api.service';
import { AccessBadgesComponent } from '../endpoint-security/access-badges.component';
import { AccessState, emptyAccess, isEmpty } from '../endpoint-security/access-editor.component';

// Presentational routes table: data in, intents out. Rows are drag-orderable
// (first-match-wins, so order is significant) via the drag handle.
@Component({
  selector: 'app-routes-table',
  imports: [
    DragDropModule,
    MatButtonModule,
    MatIconModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    AccessBadgesComponent,
  ],
  templateUrl: './routes-table.component.html',
  styleUrl: './routes-table.component.scss',
})
export class RoutesTableComponent {
  readonly routes = input.required<Route[]>();
  // Order is significant (first match wins), so it can only be changed on the
  // WHOLE list: dragging row 3 above row 1 of a filtered view would move it
  // above whatever sits at index 0 of the real one.
  readonly reorderable = input(true, { transform: booleanAttribute });
  // What the gateway has actually seen from each upstream (SVC-04), keyed by
  // route id. Empty until it is loaded, and a route with no entry says nothing
  // - which is the honest answer for one nobody has called yet.
  readonly health = input<Record<string, RouteHealth>>({});
  // Where the applications answer, from the gateway itself. Empty until it is
  // loaded, and an empty one hides the open button rather than aiming it at
  // the console's own origin - which is the one address that is always wrong.
  readonly dataOrigin = input('');
  // Whether the installation exports traces at all. A route can only be
  // "traced" when something leaves: without this, the mark would claim a trace
  // on every route of an installation that sends nothing anywhere.
  readonly tracingOn = input(false);
  readonly edit = output<Route>();
  readonly remove = output<Route>();
  readonly duplicate = output<Route>();
  readonly toggleEnabled = output<Route>();
  // Emits the full ordered list of route ids after a drag.
  readonly reorder = output<string[]>();

  protected readonly columns = ['name', 'access', 'route'];

  // Order is first-match-wins, so it means nothing on a subset: a row dragged
  // to the top of three visible rows would land above whatever really sits
  // first. Saying it beats a handle that quietly ignores the mouse.
  protected readonly handleTip = computed(() =>
    this.reorderable()
      ? $localize`:@@Drag_to_reorder:Drag to reorder`
      : $localize`:@@Clear_the_search_to_reorder:Clear the search to reorder`,
  );

  // Where a UI route ANSWERS, as a browser would ask for it: the first path
  // pattern with its wildcards cut off. A route matched on the host alone has
  // no path of its own and opens at the root.
  //
  // The link is left to the browser rather than fetched here: an unauthorised
  // caller has to LAND on the gateway's own answer - the sign-in page, or the
  // page saying which organisation or role was wanted - and a fetch would turn
  // all of that into a status code nobody reads.
  protected openUrl(r: Route): string {
    const origin = this.dataOrigin();
    if (!origin) return '';
    const first = (r.predicates ?? [])
      .filter((p) => p.type === 'path')
      .flatMap((p) => (p.args?.['patterns'] as string[]) ?? [])[0];
    const path = (first ?? '/').replace(/\*.*$/, '');
    return origin + (path.startsWith('/') ? path : '/' + path);
  }

  // Row-actions toolbars self-close on row click (closeOnClick, 3.1.0) -
  // nothing floats over the editor drawer this click opens.
  protected onRowClick(route: Route): void {
    this.edit.emit(route);
  }

  protected drop(event: CdkDragDrop<Route[]>): void {
    if (event.previousIndex === event.currentIndex) return;
    const ids = this.routes().map((r) => r.id);
    moveItemInArray(ids, event.previousIndex, event.currentIndex);
    this.reorder.emit(ids);
  }

  // The route's base Access as the badges' non-optional shape (level /
  // organisations / users / roles, same as the endpoint-security screen).
  protected accessBadge(r: Route): AccessState {
    const a = r.access;
    return a
      ? { level: a.level ?? '', tenants: a.tenants ?? [], roles: a.roles ?? [], users: a.users ?? [] }
      : emptyAccess();
  }

  // How many operations carry their own rule (RBAC-07), or null for a route
  // that exposes no OpenAPI spec: there is nothing to override there, and a
  // badge counting to zero would only be noise.
  protected endpointRules(r: Route): number | null {
    if (!r.api?.spec) return null;
    return r.api?.security?.endpoints?.length ?? 0;
  }

  // Meerkat gates nothing on this route: no rule of its own, and no endpoint
  // making up for it. A legitimate choice, and also what a route someone has
  // not finished configuring looks like - hence the warning tone.
  protected gatesNothing(r: Route): boolean {
    return isEmpty(this.accessBadge(r)) && !this.endpointRules(r);
  }

  protected summary(r: Route): string {
    return r.predicates
      .map((p) => {
        const first = p.args ? Object.values(p.args)[0] : undefined;
        const value = Array.isArray(first) ? first.join(', ') : (first ?? '');
        return value === '' ? p.type : `${p.type}: ${value}`;
      })
      .join(' AND ');
  }

  // The filters by name, in the order they run: a count said there were some,
  // the names say which.
  protected filterNames(r: Route): string {
    return r.filters.map((f) => f.type).join(', ');
  }

  // Where the request goes: the upstream, or - for a route that answers by
  // itself (a redirect, a fixed answer) - the filter that does.
  protected target(r: Route): string {
    if (r.upstream) return r.upstream;
    const last = r.filters[r.filters.length - 1];
    return last ? $localize`:@@Answered_by_FILTER:answered by ${last.type}:filter:` : '';
  }

  // The heart beside a route's name, or null when there is nothing to say:
  // unknown yet, no upstream, or the route is off. Three colours for a
  // service the runtime counts: all replicas ready, some, none.
  protected heart(r: Route): { state: 'up' | 'degraded' | 'down'; tip: string } | null {
    const h = this.health()[r.id];
    if (!r.enabled || !h?.target) return null;
    const why = h.targetWhy ?? '';
    const images = (h.replicas?.images ?? []).map((i) => `${i.ref}${i.digest ? ' @' + i.digest : ''} (${i.ready}/${i.count})`);
    const tail = images.length ? '\n' + images.join('\n') : '';
    switch (h.target) {
      case 'up':
        return { state: 'up', tip: $localize`:@@Target_up_tip:Up: ${why}:WHY:` + tail };
      case 'degraded':
        return { state: 'degraded', tip: $localize`:@@Target_degraded_tip:Degraded: ${why}:WHY:` + tail };
      default:
        return { state: 'down', tip: $localize`:@@Target_down_tip:Down: ${why}:WHY:` + tail };
    }
  }

  protected readonly uiTip = $localize`:@@Serves_pages_in_a_browser:UI: serves pages in a browser`;
  protected readonly notUiTip = $localize`:@@Not_a_UI_route:Not a UI route: serves an API`;
  protected readonly tracedTip = $localize`:@@Route_is_traced:Traced: pushed to OpenTelemetry`;
  protected readonly notTracedTip = $localize`:@@Route_not_traced:Not traced`;

  // Whether the route produces traces: the export is on and the route is not
  // left out.
  protected traced(r: Route): boolean {
    return this.tracingOn() && r.telemetry !== false;
  }

  // "2/3" under the heart, for a service the runtime counts.
  protected replicas(r: Route): string {
    const rep = this.health()[r.id]?.replicas;
    return r.enabled && rep ? `${rep.ready}/${rep.wanted}` : '';
  }

  // What the replicas run, after the upstream: the tag, or the start of the
  // digest when the tag says nothing ("latest", or none). Several during a
  // rollout, each with how many run it.
  protected images(r: Route): string {
    const imgs = this.health()[r.id]?.replicas?.images ?? [];
    if (!r.enabled || imgs.length === 0) return '';
    const name = (i: RuntimeImage) =>
      i.tag && i.tag !== 'latest' ? i.tag : `${i.tag || 'latest'}${i.digest ? ' ' + i.digest : ''}`;
    if (imgs.length === 1) return name(imgs[0]);
    return imgs.map((i) => `${name(i)} (${i.count})`).join(', ');
  }
}
