import { Component, inject } from '@angular/core';
import { GroupsMatrixComponent } from '../groups-matrix/groups-matrix.component';
import { TenantScope } from '../tenant-scope';

// The groups matrix, fed by the layout's scope: the tenant, the header search
// and the header tag.
//
// The group MODE is up there too, in the same row as the search and the tag -
// it heads the matrix the way they do, and a field of its own under them made
// one filter bar read as two. It left the General tab because it describes
// what the matrix means, not the organisation's name or its hours; the write
// lives in TenantScope, next to the tenant it saves.
@Component({
  selector: 'app-tenant-groups',
  imports: [GroupsMatrixComponent],
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        min-height: 0;
      }
      app-groups-matrix {
        flex: 1 1 auto;
        min-height: 0;
      }
    `,
  ],
  template: `
    @if (scope.tenant(); as t) {
      <app-groups-matrix
        [tenantId]="t.id"
        [filter]="scope.filter()"
        [tagFilter]="scope.tagFilter()"
        (availableTags)="scope.tags.set($event)"
      />
    }
  `,
})
export class TenantGroupsComponent {
  protected readonly scope = inject(TenantScope);
}
