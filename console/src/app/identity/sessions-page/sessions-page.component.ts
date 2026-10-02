import { Component, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { RowActionsDirective } from '@softwarity/row-actions';
import { DateTime } from 'luxon';
import { ApiService, LiveSession } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { MeService } from '../../me.service';

// Who is signed in where, right now (CONSOLE-08, CONSOLE-01), and a way to
// end any one session - the laptop left open at a client's, the account being
// handed over. Root reads both planes; an application administrator the
// applications' sessions only, since who runs the console is root's business.
@Component({
  selector: 'app-sessions-page',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatPaginatorModule,
    MatSelectModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
  ],
  styles: `
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
      padding: 12px 24px 0;
    }
    .banner h1 {
      font-size: 1.15rem;
      font-weight: 500;
      margin: 0;
      flex: 1;
    }
    .toolbar {
      display: flex;
      gap: 12px;
      flex-wrap: wrap;
      align-items: center;
      padding: 8px 24px;
    }
    .hint {
      margin: 0;
      padding: 0 24px;
      font-size: 0.85rem;
      color: var(--mat-sys-on-surface-variant);
      max-width: 90ch;
    }
    .search {
      flex: 1 1 220px;
      max-width: 360px;
    }
    .scroll {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 0 24px;
    }
    .who {
      display: grid;
      gap: 2px;
    }
    .sub,
    .when {
      font-size: 0.78rem;
      color: var(--mat-sys-on-surface-variant);
    }
    .mono {
      font-family: var(--mk-mono, monospace);
    }
    .plane {
      font-size: 0.72rem;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--mat-sys-on-surface-variant);
    }
  `,
  template: `
    <div class="banner">
      <h1 i18n="@@Sessions">Sessions</h1>
    </div>
    <p class="hint" i18n="@@Sessions_hint">
      Who is signed in, right now. Signing a session out ends it at once, on every gateway.
    </p>
    <div class="toolbar">
      <mat-form-field class="search" subscriptSizing="dynamic">
        <mat-label i18n="@@Search_account">Account</mat-label>
        <input matInput [value]="search()" (input)="onSearch($any($event.target).value)" />
      </mat-form-field>
      @if (me.isRoot()) {
        <mat-form-field subscriptSizing="dynamic">
          <mat-label i18n="@@Plane">Plane</mat-label>
          <mat-select [value]="plane()" (selectionChange)="plane.set($event.value); first()">
            <mat-option value="" i18n="@@All">All</mat-option>
            <mat-option value="data" i18n="@@Applications">Applications</mat-option>
            <mat-option value="admin" i18n="@@Console">Console</mat-option>
          </mat-select>
        </mat-form-field>
      }
    </div>
    <div class="scroll">
      @if (loading()) {
        <loading-indicator withContainer />
      } @else {
        <table mat-table [dataSource]="rows()">
          <ng-container matColumnDef="who">
            <th mat-header-cell *matHeaderCellDef i18n="@@Account">Account</th>
            <td mat-cell *matCellDef="let s">
              <span class="who">
                <span>{{ s.username }}</span>
                @if (s.tenantName) {
                  <span class="sub">{{ s.tenantName }}</span>
                }
              </span>
            </td>
          </ng-container>
          <ng-container matColumnDef="browser">
            <th mat-header-cell *matHeaderCellDef i18n="@@Browser">Browser</th>
            <td mat-cell *matCellDef="let s">
              <span class="who">
                <span>{{ s.label || '-' }}</span>
                @if (s.ip) {
                  <span class="sub mono">{{ s.ip }}</span>
                }
              </span>
            </td>
          </ng-container>
          <ng-container matColumnDef="plane">
            <th mat-header-cell *matHeaderCellDef i18n="@@Plane">Plane</th>
            <td mat-cell *matCellDef="let s">
              <span class="plane">
                @if (s.plane === 'admin') {
                  <ng-container i18n="@@Console">Console</ng-container>
                } @else {
                  <ng-container i18n="@@Applications">Applications</ng-container>
                }
              </span>
            </td>
          </ng-container>
          <!-- The row action lives in the LAST existing column. -->
          <ng-container matColumnDef="since">
            <th mat-header-cell *matHeaderCellDef i18n="@@Signed_in">Signed in</th>
            <td mat-cell *matCellDef="let s">
              <span class="when" [title]="full(s.createdAt)">{{ rel(s.createdAt) }}</span>
              <span rowActions="tonal">
                @if (!s.current) {
                  <button
                    matIconButton
                    (click)="revoke(s)"
                    i18n-matTooltip="@@Sign_out_this_session"
                    matTooltip="Sign this session out"
                    i18n-aria-label="@@Sign_out_this_session"
                    aria-label="Sign this session out"
                  >
                    <mat-icon>logout</mat-icon>
                  </button>
                }
              </span>
            </td>
          </ng-container>
          <tr mat-header-row *matHeaderRowDef="columns"></tr>
          <tr mat-row *matRowDef="let row; columns: columns"></tr>
          <tr class="mat-row" *matNoDataRow>
            <td class="mat-cell" [attr.colspan]="columns.length" i18n="@@No_sessions">No session matches.</td>
          </tr>
        </table>
      }
    </div>
    <mat-paginator
      [length]="total()"
      [pageSize]="pageSize()"
      [pageIndex]="pageIndex()"
      [pageSizeOptions]="[25, 50, 100]"
      (page)="onPage($event)"
    />
  `,
})
export class SessionsPageComponent {
  private readonly api = inject(ApiService);
  private readonly dialogs = inject(DialogsService);
  private readonly snack = inject(MatSnackBar);
  private readonly locale = inject(LOCALE_ID);
  protected readonly me = inject(MeService);

  protected readonly columns = ['who', 'browser', 'plane', 'since'];
  protected readonly rows = signal<LiveSession[]>([]);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly search = signal('');
  protected readonly plane = signal('');
  protected readonly pageSize = signal(50);
  protected readonly pageIndex = signal(0);
  private searchTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    this.load();
  }

  protected onSearch(value: string): void {
    this.search.set(value);
    clearTimeout(this.searchTimer);
    this.searchTimer = setTimeout(() => this.first(), 250);
  }

  protected first(): void {
    this.pageIndex.set(0);
    this.load();
  }

  protected onPage(e: PageEvent): void {
    this.pageIndex.set(e.pageIndex);
    this.pageSize.set(e.pageSize);
    this.load();
  }

  private load(): void {
    this.api
      .listSessions({
        q: this.search(),
        plane: this.plane(),
        limit: this.pageSize(),
        offset: this.pageIndex() * this.pageSize(),
      })
      .subscribe({
        next: (p) => {
          this.rows.set(p.sessions);
          this.total.set(p.total);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });
  }

  protected async revoke(s: LiveSession): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Sign_out_session_title:Sign this session out?`,
      message: $localize`:@@Sign_out_session_message:${s.username}:name: will have to sign in again on ${s.label || s.ip}:where:.`,
      confirmLabel: $localize`:@@Sign_out:Sign out`,
      danger: true,
    });
    if (!ok) return;
    this.api.revokeSession(s.id).subscribe({
      next: () => this.load(),
      error: (e) =>
        this.snack.open(e?.error?.error ?? $localize`:@@Request_failed:Request failed`, undefined, { duration: 4000 }),
    });
  }

  protected rel(at: number): string {
    return at ? (DateTime.fromSeconds(at).reconfigure({ locale: this.locale }).toRelative() ?? '') : '-';
  }

  protected full(at: number): string {
    return at ? DateTime.fromSeconds(at).reconfigure({ locale: this.locale }).toLocaleString(DateTime.DATETIME_MED) : '';
  }
}
