import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, PlugSession } from '../api.service';
import { MeService } from '../me.service';
import { DialogsService } from '../shared/dialogs.service';
import { LiveChangesService } from '../shared/live-changes.service';
import { zoned } from '../shared/zoned-date.pipe';

// Who is plugged into the cluster, now (DEV-13): one row per plug run, merged
// over the gateway's replicas, with what it serves - or nothing, for a client
// that only consumes - and a way to end it. Ending one stops the developer's
// command as a Ctrl-C would and puts back at once what it had set aside: the
// colleague gone to lunch with checkout served from their laptop.
//
// Live: a run opening or ending, a name served or put back, is an entry of the
// trail under the kind 'plug', and the list is read again.
@Component({
  selector: 'app-plug-sessions',
  imports: [MatButtonModule, MatIconModule, MatTableModule, MatTooltipModule, RowActionsDirective],
  styleUrl: './plug-sessions.component.scss',
  template: `
    <h3 class="group" i18n="@@Plug_group_sessions">Plugged in now</h3>
    @if (rows().length === 0) {
      <p class="hint" i18n="@@Plug_no_sessions">Nobody is plugged in.</p>
    } @else {
      <table mat-table [dataSource]="rows()">
        <ng-container matColumnDef="who">
          <th mat-header-cell *matHeaderCellDef i18n="@@Plug_session_developer">Developer</th>
          <td mat-cell *matCellDef="let s">{{ s.fullname || s.who || '-' }}</td>
        </ng-container>
        <ng-container matColumnDef="serves">
          <th mat-header-cell *matHeaderCellDef i18n="@@Plug_session_serves">Serves</th>
          <td mat-cell *matCellDef="let s">
            @for (n of s.names; track n.name) {
              <span class="served">
                <!-- The cluster as a grid, the run's piece coloured: IN the grid
                     when it takes a deployed service's place (grid_view), set
                     beside it when the name is new (widgets). -->
                @if (n.parked) {
                  <svg
                    class="mark"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    i18n-matTooltip="@@Plug_session_parked"
                    matTooltip="Replaces the deployed {{ n.name }} while plugged in"
                  >
                    <rect x="4" y="4" width="6" height="6" />
                    <rect class="mine" x="14" y="4" width="6" height="6" />
                    <rect x="4" y="14" width="6" height="6" />
                    <rect x="14" y="14" width="6" height="6" />
                  </svg>
                } @else {
                  <svg
                    class="mark"
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    i18n-matTooltip="@@Plug_session_added"
                    matTooltip="New name: no {{ n.name }} is deployed in the cluster"
                  >
                    <rect x="4" y="4" width="6" height="6" />
                    <rect class="mine" x="14.5" y="4.5" width="5" height="5" transform="rotate(45 17 7)" />
                    <rect x="4" y="14" width="6" height="6" />
                    <rect x="14" y="14" width="6" height="6" />
                  </svg>
                }
                <code class="name">{{ n.name }}</code>
              </span>
            } @empty {
              @if (s.mode === 'serve') {
                <span class="sub" i18n="@@Plug_session_nothing_served_yet">nothing served yet</span>
              } @else {
                <span class="sub" i18n="@@Plug_session_client_only">client only</span>
              }
            }
          </td>
        </ng-container>
        <ng-container matColumnDef="from">
          <th mat-header-cell *matHeaderCellDef i18n="@@Plug_session_from">From</th>
          <td mat-cell *matCellDef="let s">
            <span class="two-line">
              <span class="mono">{{ s.from || '-' }}</span>
              @if (s.version || s.docker) {
                <span class="sub version">
                  @if (s.version) {
                    plug {{ s.version }}
                  }
                  @if (s.docker) {
                    <mat-icon
                      class="docker"
                      svgIcon="docker"
                      i18n-matTooltip="@@Plug_session_docker"
                      matTooltip="Runs in a container (--dockerrun)"
                    />
                  }
                </span>
              }
            </span>
          </td>
        </ng-container>
        <!-- The row action lives in the LAST existing column. -->
        <!-- WHEN it was plugged in, not "how long ago": a relative time is
             only true at the moment it is computed, and this list is only read
             again when a run opens or ends. -->
        <ng-container matColumnDef="since">
          <th mat-header-cell *matHeaderCellDef i18n="@@Plug_session_plugged_at">Plugged in at</th>
          <td mat-cell *matCellDef="let s">
            <span class="sub nowrap">{{ at(s.since) }}</span>
            <span rowActions="tonal">
              <button
                matIconButton
                (click)="close(s)"
                i18n-matTooltip="@@Plug_session_close"
                matTooltip="End this session"
                i18n-aria-label="@@Plug_session_close"
                aria-label="End this session"
              >
                <mat-icon>link_off</mat-icon>
              </button>
            </span>
          </td>
        </ng-container>
        <tr mat-header-row *matHeaderRowDef="columns"></tr>
        <tr mat-row *matRowDef="let row; columns: columns"></tr>
      </table>
    }
  `,
})
export class PlugSessionsComponent {
  private readonly api = inject(ApiService);
  private readonly dialogs = inject(DialogsService);
  private readonly snack = inject(MatSnackBar);
  private readonly zone = inject(MeService).timezone;

  protected readonly columns = ['who', 'serves', 'from', 'since'];
  protected readonly rows = signal<PlugSession[]>([]);

  constructor() {
    this.load();
    inject(LiveChangesService).on('plug', () => this.load());
  }

  private load(): void {
    this.api.plugSessions().subscribe({ next: (s) => this.rows.set(s) });
  }

  protected async close(s: PlugSession): Promise<void> {
    const who = s.fullname || s.who || s.key || s.id;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Plug_session_close_title:End this session?`,
      message: $localize`:@@Plug_session_close_message:The plug command of ${who}:who: stops as with Ctrl-C, and the deployed services take over at once. They can still reconnect: to block them, remove their key.`,
      confirmLabel: $localize`:@@Plug_session_close_confirm:End`,
      danger: true,
    });
    if (!ok) return;
    this.api.closePlugSession(s.id).subscribe({
      next: () => this.load(),
      error: (e) =>
        this.snack.open(e?.error?.error ?? $localize`:@@Request_failed:Request failed`, undefined, { duration: 4000 }),
    });
  }

  // The moment it was plugged in, in the reader's zone (the wire says RFC 3339).
  protected at(at: string): string {
    const ms = at ? Date.parse(at) : NaN;
    return isNaN(ms) ? '-' : zoned(ms / 1000, this.zone());
  }
}
