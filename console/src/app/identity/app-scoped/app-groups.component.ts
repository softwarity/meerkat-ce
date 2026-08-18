import { Component, computed, inject, signal } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, Tenant } from '../../api.service';
import { MeService } from '../../me.service';
import { GroupsMatrixComponent } from '../groups-matrix/groups-matrix.component';

// Groups, seen from Application rather than from an organisation.
//
// In single-tenant mode there IS one organisation - every installation owns
// one from its first boot - but nothing names it, so the screens that used to
// hang off it live here instead. The matrix underneath is the same component;
// it just gets the served organisation's id instead of the one in the URL.
//
// The group MODE moved here too, out of the organisation's General tab: it
// describes how these groups combine (cumulate every one, or pick a single one
// at sign-in), which is a question about this screen and not about the
// organisation's name or its hours.
@Component({
  selector: 'app-app-groups',
  imports: [
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    GroupsMatrixComponent,
  ],
  template: `
    <div class="banner">
      <h1 i18n="@@Groups">Groups</h1>
    </div>

    <div class="toolbar">
      <!-- The two modes carry their own explanation, in the list where one
           chooses between them - the same shape as the status-code picker. As
           a hint it was three lines under a 56px field, and the toolbar read
           as two rows for one filter bar. -->
      <mat-form-field class="mode" subscriptSizing="dynamic">
        <mat-label i18n="@@Group_mode">Group mode</mat-label>
        <mat-select [value]="groupMode()" (selectionChange)="setGroupMode($event.value)">
          <!-- Without it the closed field would read the option's WHOLE text,
               explanation included: mat-select takes the option's text content
               as its display value. -->
          <mat-select-trigger>{{ modeLabel() }}</mat-select-trigger>
          <mat-option value="MULTIPLE">
            <span i18n="@@Cumulative">Cumulative</span>
            <small class="what" i18n="@@Cumulative_what">merges the roles of every group</small>
          </mat-option>
          <mat-option value="SINGLE">
            <span i18n="@@Exclusive">Exclusive</span>
            <small class="what" i18n="@@Exclusive_what">one group is picked at sign-in</small>
          </mat-option>
        </mat-select>
      </mat-form-field>

      @if (tags().length) {
        <mat-form-field class="tag-pick" subscriptSizing="dynamic">
          <mat-label i18n="@@Tag">Tag</mat-label>
          <mat-select [value]="tagFilter()" (selectionChange)="tagFilter.set($event.value)">
            <mat-option value="" i18n="@@All">All</mat-option>
            @for (t of tags(); track t) {
              <mat-option [value]="t">{{ t }}</mat-option>
            }
          </mat-select>
        </mat-form-field>
      }

      <mat-form-field class="search" subscriptSizing="dynamic">
        <mat-label i18n="@@Search">Search</mat-label>
        <input matInput [value]="filter()" (input)="filter.set($any($event.target).value)" />
      </mat-form-field>
    </div>

    @if (tenantId(); as id) {
      <div class="panel">
        <app-groups-matrix
          [tenantId]="id"
          [filter]="filter()"
          [tagFilter]="tagFilter()"
          (availableTags)="tags.set($event)"
        />
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        min-height: 0;
      }
      .banner {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 12px 24px;
        flex: none;
      }
      .banner h1 {
        font-size: 1.15rem;
        font-weight: 500;
        margin: 0;
        flex: 1;
      }
      .toolbar {
        flex: none;
        display: flex;
        align-items: flex-start;
        gap: 12px;
        padding: 0 24px 12px;
        flex-wrap: wrap;
      }
      .mode,
      .tag-pick {
        flex: 0 1 260px;
      }
      /* The meaning under the mode's name, in the list only: a second line in
         the closed field would grow it past its neighbours. */
      .what {
        display: block;
        margin-top: -2px;
        font-size: 0.75rem;
        line-height: 1.25;
        color: var(--mat-sys-on-surface-variant);
      }
      .search {
        flex: 1 1 240px;
        max-width: 420px;
      }
      /* The same frame the organisation page gives its sections: without it
         the matrix touched the window edges here and nowhere else. */
      .panel {
        flex: 1 1 auto;
        min-height: 0;
        min-width: 0;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        padding: 0 24px 20px;
      }
      .panel > * {
        flex: 1 1 auto;
        min-height: 0;
      }
    `,
  ],
})
export class AppGroupsComponent {
  private readonly api = inject(ApiService);
  private readonly me = inject(MeService);

  protected readonly filter = signal('');
  protected readonly tags = signal<string[]>([]);
  protected readonly tagFilter = signal('');
  private readonly tenant = signal<Tenant | null>(null);

  protected readonly tenantId = computed(() => this.tenant()?.id ?? this.me.primaryTenant());
  protected readonly groupMode = computed(() => this.tenant()?.groupMode || 'MULTIPLE');
  protected readonly modeLabel = computed(() =>
    this.groupMode() === 'SINGLE'
      ? $localize`:@@Exclusive:Exclusive`
      : $localize`:@@Cumulative:Cumulative`,
  );

  constructor() {
    const id = this.me.primaryTenant();
    if (id) {
      this.api.getTenant(id).subscribe({ next: (t) => this.tenant.set(t), error: () => undefined });
    }
  }

  // Saving the mode carries the whole organisation back, which is what the
  // endpoint expects: it is one object, and sending a partial one would blank
  // whatever it left out.
  protected setGroupMode(mode: string): void {
    const t = this.tenant();
    if (!t) return;
    const next = { ...t, groupMode: mode };
    this.api.updateTenant(next).subscribe({ next: (saved) => this.tenant.set(saved), error: () => undefined });
  }
}
