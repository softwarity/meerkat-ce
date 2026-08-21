import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { DatePipe } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs/operators';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { Observable } from 'rxjs';
import { ApiService, ConfigPlan, RestorePoint } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { ConfigurationYamlComponent } from './configuration-yaml.component';

// The tape (CFG-06): one restore point per change, newest first.
//
// A different object from the shelf next door, and the difference is what
// makes both readable. A saved configuration is intentional and named ("the
// Acme setup"); a point is automatic and timestamped ("what it looked like at
// 14:32"). One is for switching between customers, the other for going back.
//
// Nothing here is a list of files: it is a chronology. So the rows read as
// time and person first, the label second - the words the audit trail used for
// the same change, at the same second, because two vocabularies for one event
// is one story too many.
@Component({
  selector: 'app-configuration-history',
  imports: [
    DatePipe,
    MatButtonModule,
    MatChipsModule,
    MatIconModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    EeLockComponent,
    ConfigurationYamlComponent,
  ],
  styleUrl: './configuration-cards.scss',
  styles: [
    `
      :host {
        display: block;
        height: 100%;
        max-width: none;
        padding: 0;
      }
      mat-drawer-container {
        background: transparent;
        height: 100%;
      }
      mat-drawer-content {
        display: flex;
        flex-direction: column;
        overflow: auto;
        padding: 0 4px 24px;
      }
      mat-drawer {
        width: min(760px, 96vw);
        border-left: 1px solid var(--mat-sys-outline-variant);
        background: var(--mat-sys-surface-container-high);
      }
      .intro {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 8px 0 12px;
      }
      mat-table {
        background: transparent;
      }
      mat-header-row {
        background: var(--mat-sys-surface);
        position: sticky;
        top: 0;
        z-index: 2;
      }
      mat-cell,
      mat-header-cell {
        padding: 0 8px;
      }
      /* A day is a row of its own: the tape is read by "when", and a date
         repeated on forty lines is forty times the same word. */
      mat-row.day {
        min-height: 34px;
        background: transparent;
      }
      mat-row.day mat-cell {
        font-size: 0.78rem;
        font-weight: 600;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-row.point {
        cursor: pointer;
      }
      .mat-column-when {
        flex: 0 0 90px;
        font-variant-numeric: tabular-nums;
        color: var(--mat-sys-on-surface-variant);
      }
      .mat-column-who {
        flex: 0 0 170px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .mat-column-what {
        gap: 8px;
      }
      .what {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: var(--mat-sys-on-surface-variant);
      }
      .empty {
        margin: 24px 0;
        font-size: 0.9rem;
        color: var(--mat-sys-on-surface-variant);
        font-style: italic;
      }
    `,
  ],
  template: `
    <mat-drawer-container>
      <mat-drawer-content>
        <div class="intro">
          <p class="hint" style="margin: 0" i18n="@@History_hint">
            A restore point every time a change moves this gateway's configuration - never on
            purpose, never named. Accounts, sessions and the vault are not configuration and leave
            no trace here. Nothing is thrown away, and changes made by one person within a couple
            of minutes count as one.
          </p>
        </div>

        @if (!points().length) {
          <p class="empty" i18n="@@No_restore_point">
            Nothing recorded yet: this gateway has not been changed since it started.
          </p>
        } @else {
          <mat-table [dataSource]="rows()">
            <ng-container matColumnDef="when">
              <mat-header-cell *matHeaderCellDef i18n="@@Time">Time</mat-header-cell>
              <mat-cell *matCellDef="let p">{{ p.at * 1000 | date: 'HH:mm' }}</mat-cell>
            </ng-container>

            <ng-container matColumnDef="who">
              <mat-header-cell *matHeaderCellDef i18n="@@Who">Who</mat-header-cell>
              <mat-cell *matCellDef="let p">{{ p.actorName || started() }}</mat-cell>
            </ng-container>

            <!-- The row actions live in the LAST existing column, with the
                 same directive every other table in this console uses: one
                 gesture for one meaning, or the screens read as two products. -->
            <ng-container matColumnDef="what">
              <mat-header-cell *matHeaderCellDef i18n="@@Change">Change</mat-header-cell>
              <mat-cell *matCellDef="let p">
                <span class="what">{{ p.label }}</span>
                @if (p.current) {
                  <mat-chip disableRipple highlighted i18n="@@Current">Current</mat-chip>
                }
                @if (p.savedAs; as name) {
                  <mat-chip disableRipple [matTooltip]="savedTip()">{{ name }}</mat-chip>
                }
                @if (p.sameAs; as at) {
                  <mat-chip disableRipple [matTooltip]="sameTip()">
                    <mat-icon matChipAvatar>history</mat-icon>
                    {{ at * 1000 | date: 'HH:mm' }}
                  </mat-chip>
                }
                <span rowActions="tonal">
                  <!-- Disabled on every point the gateway is ALREADY in, not
                       only on the newest of them: two points can hold the same
                       state (going back produces exactly that), and offering
                       Restore on the older one offers a button that reports
                       success and changes nothing. -->
                  <button
                    matIconButton
                    ee-feature="configurations"
                    [disabled]="busy() || p.live"
                    (click)="$event.stopPropagation(); restore(p)"
                    [matTooltip]="p.live ? alreadyTip() : restoreTip()"
                    i18n-aria-label="@@Restore"
                    aria-label="Restore"
                  >
                    <mat-icon>history</mat-icon>
                  </button>
                  <button
                    matIconButton
                    ee-feature="configurations"
                    [disabled]="busy()"
                    (click)="$event.stopPropagation(); saveAs(p)"
                    i18n-matTooltip="@@Save_under_a_name"
                    matTooltip="Save under a name"
                    i18n-aria-label="@@Save_under_a_name"
                    aria-label="Save under a name"
                  >
                    <mat-icon>bookmark_add</mat-icon>
                  </button>
                </span>
              </mat-cell>
            </ng-container>

            <!-- The day heading is a row with one cell, told apart by the when
                 predicate - how Material renders two shapes from one source. -->
            <ng-container matColumnDef="day">
              <mat-cell *matCellDef="let g">{{ g.day }}</mat-cell>
            </ng-container>

            <mat-header-row *matHeaderRowDef="columns"></mat-header-row>
            <mat-row *matRowDef="let row; columns: ['day']; when: isDay" class="day"></mat-row>
            <mat-row *matRowDef="let row; columns: columns" class="point" (click)="open(row)"></mat-row>
          </mat-table>
        }

                <app-ee-lock
          feature="configurations"
          i18n-why="@@History_ee_why"
          why="Go back to any moment of this gateway's configuration, and keep several configurations side by side."
        />
      </mat-drawer-content>

      <mat-drawer position="end" mode="over" [opened]="opened() !== null" (closedStart)="close()">
        @if (opened(); as p) {
          <app-configuration-yaml
            [title]="title(p)"
            [subtitle]="p.label ?? ''"
            [text]="document()"
            [against]="comparing() ? live() : ''"
            [canEdit]="false"
            [note]="note()"
            (download)="download(p)"
            (closed)="close()"
          />
          <div style="padding: 0 16px 16px">
            <button matButton (click)="comparing.set(!comparing())">
              <mat-icon>difference</mat-icon>
              @if (comparing()) {
                <ng-container i18n="@@Show_the_document">Show the document</ng-container>
              } @else {
                <ng-container i18n="@@Compare_with_now">Compare with now</ng-container>
              }
            </button>
            <button
              matButton="filled"
              ee-feature="configurations"
              [disabled]="busy() || p.current"
              (click)="restore(p)"
            >
              <mat-icon>history</mat-icon>
              <ng-container i18n="@@Restore">Restore</ng-container>
            </button>
          </div>
        }
      </mat-drawer>
    </mat-drawer-container>
  `,
})
export class ConfigurationHistoryComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  protected readonly points = signal<RestorePoint[]>([]);
  protected readonly busy = signal(false);
  protected readonly document = signal('');
  protected readonly live = signal('');
  protected readonly comparing = signal(false);

  private readonly openId = toSignal(this.route.paramMap.pipe(map((p) => p.get('id'))), {
    initialValue: null,
  });
  protected readonly opened = computed(() => {
    const id = this.openId();
    return id ? (this.points().find((p) => p.id === id) ?? null) : null;
  });

  protected readonly columns = ['when', 'who', 'what'];

  // One flat list of day markers and points: a table wants rows, and the day
  // is a row like the others - told apart by `isDay`, which is how Material
  // renders two shapes from one source.
  protected readonly rows = computed<(RestorePoint | { day: string })[]>(() => {
    const out: (RestorePoint | { day: string })[] = [];
    let current = '';
    for (const p of this.points()) {
      const day = new Date(p.at * 1000).toLocaleDateString();
      if (day !== current) {
        current = day;
        out.push({ day });
      }
      out.push(p);
    }
    return out;
  });

  protected readonly isDay = (_: number, row: RestorePoint | { day: string }) => 'day' in row;

  protected readonly started = () => $localize`:@@The_gateway:the gateway`;
  protected readonly savedTip = () =>
    $localize`:@@Saved_here:A saved configuration holds exactly this state.`;
  protected readonly restoreTip = () => $localize`:@@Restore:Restore`;
  protected readonly alreadyTip = () =>
    $localize`:@@Already_this_state:This gateway is already in this state.`;
  protected readonly sameTip = () =>
    $localize`:@@Same_as_earlier:The same configuration as that earlier moment - this is where it came back to it.`;
  protected readonly note = () =>
    $localize`:@@Point_note:A point never carried the images: restoring one leaves the pictures in place.`;

  constructor() {
    this.reload();
    // Same as the Management drawer: the URL decides what is open, so a pasted
    // link loads the same document a click would.
    effect(() => {
      const p = this.opened();
      untracked(() => this.load(p));
    });
  }

  private loadedId = '';

  private load(p: RestorePoint | null): void {
    if (!p) {
      this.loadedId = '';
      this.document.set('');
      return;
    }
    if (this.loadedId === p.id) return;
    this.loadedId = p.id;
    this.document.set('');
    this.comparing.set(false);
    this.api.restorePointDocument(p.id).subscribe({
      next: (text) => this.document.set(text),
      error: (err: unknown) => this.fail(err),
    });
    // Fetched alongside, so "compare with now" answers on the click rather
    // than after one.
    this.api.currentDocument().subscribe({ next: (text) => this.live.set(text) });
  }

  protected title(p: RestorePoint): string {
    return new Date(p.at * 1000).toLocaleString();
  }

  private reload(): void {
    this.api.configHistory().subscribe({
      next: (list) => this.points.set(list),
      error: () => this.points.set([]),
    });
  }

  protected open(p: RestorePoint): void {
    void this.router.navigate(['/infra/configuration/history', p.id]);
  }

  protected close(): void {
    void this.router.navigate(['/infra/configuration/history']);
  }

  // Going back is a change like any other: it shows its plan first, and it
  // leaves its own point - the tape never loses the state it was asked to
  // leave.
  protected restore(p: RestorePoint): void {
    this.busy.set(true);
    this.api.restorePointPlan(p.id).subscribe({
      next: async (plan) => {
        this.busy.set(false);
        const ok = await this.dialogs.confirm({
          title: $localize`:@@Go_back_to:Go back to ${this.title(p)}:when:?`,
          message: this.planMessage(plan),
          confirmLabel: $localize`:@@Restore:Restore`,
          danger: this.removes(plan) > 0,
        });
        if (!ok) return;
        // A plan that changes nothing must not be reported as a change: "done"
        // over an unchanged screen is the most confusing answer a button can
        // give.
        const done = plan.changes.some((c) => c.action !== 'same')
          ? $localize`:@@Restored:Restored`
          : $localize`:@@Nothing_to_change:Nothing to change: the gateway already matched that moment`;
        this.run(this.api.restoreConfigPoint(p.id), done);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  protected async saveAs(p: RestorePoint): Promise<void> {
    const name = await this.dialogs.prompt({
      title: $localize`:@@Save_under_a_name:Save under a name`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Save:Save`,
    });
    if (!name) return;
    this.run(
      this.api.saveRestorePoint(p.id, name),
      $localize`:@@Configuration_saved:Configuration saved`,
    );
  }

  protected download(p: RestorePoint): void {
    this.api.restorePointDocument(p.id).subscribe({
      next: (text) => {
        const url = URL.createObjectURL(new Blob([text], { type: 'application/yaml' }));
        const a = document.createElement('a');
        a.href = url;
        a.download = `meerkat-${new Date(p.at * 1000).toISOString().slice(0, 16).replace(/[:T]/g, '')}.yaml`;
        a.click();
        URL.revokeObjectURL(url);
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  private planMessage(plan: ConfigPlan): string {
    const count = (action: string) => plan.changes.filter((c) => c.action === action).length;
    const removed = this.removes(plan);
    if (!count('add') && !count('update') && !removed) {
      return $localize`:@@Restore_no_change:This gateway already matches that moment: restoring it changes nothing.`;
    }
    const gone = plan.changes
      .filter((c) => c.action === 'remove')
      .map((c) => c.label)
      .join(', ');
    const head = $localize`:@@Activate_counts:${count('add')}:added: added, ${count('update')}:updated: updated, ${removed}:removed: removed.`;
    return removed ? head + ' ' + $localize`:@@Activate_removed_list:What goes: ${gone}:gone:.` : head;
  }

  private removes(plan: ConfigPlan): number {
    return plan.changes.filter((c) => c.action === 'remove').length;
  }

  private run(call: Observable<unknown>, done: string): void {
    this.busy.set(true);
    call.subscribe({
      next: () => {
        this.busy.set(false);
        this.reload();
        this.snack.open(done, undefined, { duration: 2500 });
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      undefined,
      { duration: 6000 },
    );
  }
}
