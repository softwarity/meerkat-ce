import { Component, inject, LOCALE_ID, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatPaginatorModule, PageEvent } from '@angular/material/paginator';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute } from '@angular/router';
import { LivewireClient, LiveTopic } from '@softwarity/livewire';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { RowActionsDirective } from '@softwarity/row-actions';
import { DateTime } from 'luxon';
import { ApiService, LiveSession } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';

// Who is signed in where, right now (CONSOLE-08, CONSOLE-01), and a way to
// end any one session - the laptop left open at a client's, the account being
// handed over.
//
// One plane per screen, from the route: the applications' sessions under Data
// plane (root, an application administrator, an organisation's administrator,
// each within their perimeter), the console's under Meerkat (root's alone,
// since who runs the console is root's business). Live: the presence topic
// says a session moved, and the page is read again.
@Component({
  selector: 'app-sessions-page',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatPaginatorModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
  ],
  styleUrl: './sessions-page.component.scss',
  template: `
    <div class="banner">
      <h1 i18n="@@Sessions">Sessions</h1>
      <mat-form-field class="search" subscriptSizing="dynamic">
        <mat-label i18n="@@Search_account">Account</mat-label>
        <input matInput [value]="search()" (input)="onSearch($any($event.target).value)" />
      </mat-form-field>
    </div>
    <p class="hint">
      @if (plane === 'admin') {
        <ng-container i18n="@@Sessions_console_hint">
          Who is signed in to this console, right now. Signing a session out ends it at once, on every gateway.
        </ng-container>
      } @else {
        <ng-container i18n="@@Sessions_hint">
          Who is signed in, right now. Signing a session out ends it at once, on every gateway.
        </ng-container>
      }
    </p>
    <div class="content">
      @if (loading()) {
        <loading-indicator withContainer />
      } @else {
        <div class="table-wrap">
          <mat-table [dataSource]="rows()">
            <ng-container matColumnDef="who">
              <mat-header-cell *matHeaderCellDef i18n="@@Account">Account</mat-header-cell>
              <mat-cell *matCellDef="let s">
                <span class="two-line">
                  <span>{{ s.username }}</span>
                  @if (s.tenantName) {
                    <span class="sub">{{ s.tenantName }}</span>
                  }
                </span>
              </mat-cell>
            </ng-container>
            <ng-container matColumnDef="browser">
              <mat-header-cell *matHeaderCellDef i18n="@@Browser">Browser</mat-header-cell>
              <mat-cell *matCellDef="let s">
                <span class="two-line">
                  <span>{{ s.label || '-' }}</span>
                  @if (s.ip) {
                    <span class="sub mono">{{ s.ip }}</span>
                  }
                </span>
              </mat-cell>
            </ng-container>
            <!-- The row action lives in the LAST existing column. -->
            <ng-container matColumnDef="since">
              <mat-header-cell *matHeaderCellDef i18n="@@Signed_in">Signed in</mat-header-cell>
              <mat-cell *matCellDef="let s">
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
              </mat-cell>
            </ng-container>
            <mat-header-row *matHeaderRowDef="columns; sticky: true"></mat-header-row>
            <mat-row *matRowDef="let row; columns: columns"></mat-row>
            <div class="empty" *matNoDataRow i18n="@@No_sessions">No session matches.</div>
          </mat-table>
        </div>
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

  protected readonly plane = (inject(ActivatedRoute).snapshot.data['sessions'] as 'data' | 'admin') ?? 'data';
  protected readonly columns = ['who', 'browser', 'since'];
  protected readonly rows = signal<LiveSession[]>([]);
  protected readonly total = signal(0);
  protected readonly loading = signal(true);
  protected readonly search = signal('');
  protected readonly pageSize = signal(50);
  protected readonly pageIndex = signal(0);
  private searchTimer: ReturnType<typeof setTimeout> | undefined;

  constructor() {
    this.load();
    new LiveTopic(inject(LivewireClient), 'presence')
      .open(null)
      .pipe(takeUntilDestroyed())
      .subscribe(() => this.load());
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
        plane: this.plane,
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
