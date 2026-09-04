import { CdkDragDrop, DragDropModule, moveItemInArray } from '@angular/cdk/drag-drop';
import { booleanAttribute, computed, Component, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { Route, RouteHealth } from '../../api.service';
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
  readonly edit = output<Route>();
  readonly remove = output<Route>();
  readonly duplicate = output<Route>();
  readonly toggleEnabled = output<Route>();
  // Emits the full ordered list of route ids after a drag.
  readonly reorder = output<string[]>();

  protected readonly columns = ['name', 'access', 'matching', 'upstream'];

  // Order is first-match-wins, so it means nothing on a subset: a row dragged
  // to the top of three visible rows would land above whatever really sits
  // first. Saying it beats a handle that quietly ignores the mouse.
  protected readonly handleTip = computed(() =>
    this.reorderable()
      ? $localize`:@@Drag_to_reorder:Drag to reorder`
      : $localize`:@@Clear_the_search_to_reorder:Reordering applies to the whole list: clear the search to move a route`,
  );

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
    const preds = r.predicates
      .map((p) => {
        const first = p.args ? Object.values(p.args)[0] : undefined;
        const value = Array.isArray(first) ? first.join(', ') : (first ?? '');
        return value === '' ? p.type : `${p.type}: ${value}`;
      })
      .join(' AND ');
    return r.filters.length ? `${preds} - ${r.filters.length} filter(s)` : preds;
  }

  // What to show beside a route's name, or null when there is nothing worth
  // saying. Three states and no fourth: refusing, failing, and quiet.
  protected trouble(r: Route): { icon: string; open: boolean; tip: string } | null {
    const h = this.health()[r.id];
    if (!h) return null;
    if (h.state !== 'closed') {
      return {
        icon: 'heart_broken',
        open: true,
        tip: $localize`:@@Not_answering_tip:Not answering - callers are getting the unavailable page. Last: ${
          h.lastError || h.lastStatus || '?'
        }:LAST:`,
      };
    }
    if (h.failures) {
      return {
        icon: 'warning',
        open: false,
        tip: $localize`:@@Failing_tip:${h.failures}:COUNT: failed answers in a row. Last: ${
          h.lastError || h.lastStatus || '?'
        }:LAST:`,
      };
    }
    return null;
  }
}
