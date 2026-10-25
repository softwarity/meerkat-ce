import { Component, computed, effect, inject, input, LOCALE_ID, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { RowActionsDirective } from '@softwarity/row-actions';
import { DateTime } from 'luxon';
import { catchError, concatMap, forkJoin, from, Observable, of, tap } from 'rxjs';
import { ApiService, Group, Member, User } from '../../api.service';
import { IdentityDrawer, MemberEditorView } from '../identity-drawer';
import { resetAccess } from '../reset-access';
import { LiveChangesService } from '../../shared/live-changes.service';
import { PresenceComponent, PresenceService } from '../../shared/presence';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { MeService } from '../../me.service';

interface UserRow {
  id: string;
  username: string;
  fullname: string;
  lastConnectionAt: number;
}

// The per-tenant members matrix (RBAC-02): rows are the available users, a
// first column marks membership OF this tenant - the checkbox joins/leaves,
// the "admin" badge beside it promotes a member to ADMIN (tenant
// administration is the membership type, TENANT-02). The tenant's owner
// (ownerId, transferred in the Danger zone) shows a read-only "owner" badge
// there instead. Then this tenant's groups are columns - a member is
// ticked into the groups they belong to; group columns are disabled for
// non-members. The last column shows the user's last connection and carries
// the tenant-scoped password reset. Everything persists immediately. Same
// mat-table pattern as the groups matrix.
@Component({
  selector: 'app-members-matrix',
  providers: [PresenceService],
  imports: [
    PresenceComponent,
    MatButtonModule,
    MatCheckboxModule,
    MatIconModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
    ZonedDatePipe,
  ],
  templateUrl: './members-matrix.component.html',
  styleUrl: './members-matrix.component.scss',
})
export class MembersMatrixComponent {

  readonly tenantId = input.required<string>();
  // The tenant's owner (ownerId): shown as a read-only badge, may be a
  // non-member. Ownership is transferred in the Danger zone, not here.
  readonly ownerId = input('');
  // Username/fullname search - typed in the drawer's right-zone header.
  readonly filter = input('');
  readonly changed = output<void>();

  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  private readonly locale = inject(LOCALE_ID);
  protected readonly zone = inject(MeService).timezone;

  protected readonly loading = signal(true);
  protected readonly rows = signal<UserRow[]>([]);
  protected readonly groups = signal<Group[]>([]);
  // userId -> their full membership (type, overrides) when a member.
  protected readonly memberships = signal<Map<string, Member>>(new Map());
  // userId -> the group ids they are assigned.
  protected readonly memberGroups = signal<Record<string, string[]>>({});

  protected readonly filteredRows = computed(() => {
    const q = this.filter().trim().toLowerCase();
    if (!q) return this.rows();
    return this.rows().filter((u) => u.username.toLowerCase().includes(q) || u.fullname.toLowerCase().includes(q));
  });

  // The membership column - belonging, the ADMIN badge, ownership - in BOTH
  // modes. Single mode means one organisation, not none: an account is not a
  // member until somebody makes it one, and administering that organisation
  // is still its own grant. Hiding the column there left the whole-column box
  // with no member to act on and nothing on screen to say why.
  protected readonly displayedColumns = computed(() => [
    'user',
    'member',
    'spacer',
    ...this.groups().map((g) => g.id),
    'lastConn',
  ]);
  // A second header row: one "the whole column" box per group. Same columns,
  // different definitions, which is how a mat-table stacks headers.
  protected readonly bulkColumns = computed(() => this.displayedColumns().map((c) => `${c}-all`));

  constructor() {
    // Somebody else's write (CONSOLE-13): a group created, a membership given
    // or taken away shows up here without a click. Quietly, and the whole
    // matrix rather than one cell - a cell here is a pair, and what moved may
    // be the row, the column or the crossing.
    inject(LiveChangesService).on('membership', () => this.load(true));
    inject(LiveChangesService).on('group', () => this.load(true));
    inject(LiveChangesService).on('user', () => this.load(true));
    effect(() => {
      this.tenantId();
      this.load();
    });
  }

  // The drawer of the page hosting this matrix.
  protected readonly drawer = inject(IdentityDrawer);

  // Only a member has a drawer: hours and history are the membership's. Each
  // of the row's two buttons opens it on the page it names.
  protected openMember(u: UserRow, view: MemberEditorView): void {
    if (this.isMember(u.id)) this.drawer.openMember(u.id, view);
  }

  private load(quiet = false): void {
    if (!quiet) this.loading.set(true);
    forkJoin({
      // listUsers is root-only; a tenant admin falls back to the members list.
      users: this.api.listUsers().pipe(catchError(() => of<User[]>([]))),
      members: this.api.listMembers(this.tenantId()),
      groups: this.api.listGroups(this.tenantId()),
    }).subscribe({
      next: ({ users, members, groups }) => {
        const rows: UserRow[] = users.length
          ? users.map((u) => ({
              id: u.id,
              username: u.username,
              fullname: u.fullname,
              lastConnectionAt: u.lastConnectionAt,
            }))
          : members.map((m) => ({
              id: m.userId,
              username: m.username,
              fullname: m.fullname,
              lastConnectionAt: m.lastConnectionAt,
            }));
        this.rows.set(rows);
        this.groups.set(groups);
        this.memberships.set(new Map(members.map((m) => [m.userId, m])));
        this.loadMemberGroups(members.map((m) => m.userId));
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  private loadMemberGroups(memberIds: string[]): void {
    if (!memberIds.length) {
      this.memberGroups.set({});
      return;
    }
    const calls = Object.fromEntries(memberIds.map((id) => [id, this.api.memberGroups(this.tenantId(), id)]));
    forkJoin(calls).subscribe({ next: (map) => this.memberGroups.set(map) });
  }

  protected isMember(userId: string): boolean {
    return this.memberships().has(userId);
  }

  protected memberType(userId: string): string {
    return this.memberships().get(userId)?.type ?? '';
  }

  protected isOwner(userId: string): boolean {
    return userId === this.ownerId();
  }

  protected toggleMember(u: UserRow, checked: boolean): void {
    const req: Observable<unknown> = checked
      ? this.api.putMember({
          userId: u.id,
          tenantId: this.tenantId(),
          type: 'USER',
          enabled: true,
          businessAccess: { inherited: true },
          sessionTTL: '',
        })
      : this.api.deleteMember(this.tenantId(), u.id);
    req.subscribe({
      next: () => {
        this.load(); // membership map, groups and types all move together
        this.changed.emit();
      },
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  // Tenant administration IS the membership type (TENANT-02): the badge flips
  // USER ↔ ADMIN. The owner has no toggle here - ownership is a tenant field,
  // transferred in the Danger zone.
  protected toggleAdmin(u: UserRow): void {
    const m = this.memberships().get(u.id);
    if (!m) return;
    const { username, fullname, email, lastConnectionAt, ...membership } = m;
    this.api.putMember({ ...membership, type: m.type === 'ADMIN' ? 'USER' : 'ADMIN' }).subscribe({
      next: (saved) =>
        this.memberships.update((map) => {
          const next = new Map(map);
          next.set(u.id, { ...m, type: saved.type });
          return next;
        }),
      error: (err) => this.snack.open(errMsg(err), undefined, { duration: 4000 }),
    });
  }

  protected memberInGroup(userId: string, groupId: string): boolean {
    return (this.memberGroups()[userId] ?? []).includes(groupId);
  }

  // ── the whole column ───────────────────────────────────────────────────────
  //
  // It acts on the members ON SCREEN, filter included: what one sees is what
  // one ticks. And only on MEMBERS - a group cannot be given to somebody who
  // does not belong to the organisation, so the box skips them rather than
  // failing halfway through.
  protected columnState(groupId: string): { checked: boolean; some: boolean } {
    const members = this.filteredRows().filter((u) => this.isMember(u.id));
    if (members.length === 0) return { checked: false, some: false };
    const inGroup = members.filter((u) => this.memberInGroup(u.id, groupId)).length;
    return { checked: inGroup === members.length, some: inGroup > 0 && inGroup < members.length };
  }

  // ONE write at a time. Firing these in parallel let each call read a
  // memberGroups the others were still changing, so the state one ended up with
  // depended on which answer came back last. concatMap makes the sequence the
  // one the user clicked, and a failure stops it instead of leaving half of a
  // column applied without saying which half.
  protected toggleColumn(groupId: string, checked: boolean): void {
    const todo = this.filteredRows().filter(
      (u) => this.isMember(u.id) && this.memberInGroup(u.id, groupId) !== checked,
    );
    if (todo.length === 0) return;
    from(todo)
      .pipe(
        concatMap((u) => {
          const current = this.memberGroups()[u.id] ?? [];
          const ids = checked ? [...current, groupId] : current.filter((g) => g !== groupId);
          return this.api
            .setMemberGroups(this.tenantId(), u.id, ids)
            .pipe(tap((saved) => this.memberGroups.update((m) => ({ ...m, [u.id]: saved }))));
        }),
      )
      .subscribe({
        error: (err) => {
          this.snack.open(errMsg(err), undefined, { duration: 5000 });
          this.load();
        },
      });
  }

  // Ticking a group MAKES someone a member, rather than requiring the
  // membership to exist first.
  //
  // That order was unusable in single-organisation mode: the column that
  // creates a membership is a multi-organisation notion and is hidden there,
  // so every checkbox stayed disabled with nothing on screen to explain it.
  // And it matches what the tick means anyway - giving someone a group in an
  // organisation IS putting them in it.
  protected toggleMemberGroup(user: UserRow, groupId: string, checked: boolean): void {
    const userId = user.id;
    const current = this.memberGroups()[userId] ?? [];
    const ids = checked ? [...current, groupId] : current.filter((g) => g !== groupId);
    const assign = () =>
      this.api.setMemberGroups(this.tenantId(), userId, ids).subscribe({
        next: (saved) => this.memberGroups.update((m) => ({ ...m, [userId]: saved })),
        error: (err) => {
          this.snack.open(errMsg(err), undefined, { duration: 4000 });
          this.load();
        },
      });
    if (checked && !this.isMember(userId)) {
      this.api
        .putMember({
          userId,
          tenantId: this.tenantId(),
          type: 'USER',
          enabled: true,
          businessAccess: { inherited: true },
          sessionTTL: '',
        })
        .subscribe({
          next: (m) => {
            // The row already carries what a Member adds to a Membership, so
            // the table updates without a second round trip.
            this.memberships.update((all) =>
              new Map(all).set(userId, {
                ...m,
                username: user.username,
                fullname: user.fullname,
                email: '',
                lastConnectionAt: user.lastConnectionAt,
              }),
            );
            assign();
          },
          error: (err) => {
            this.snack.open(errMsg(err), undefined, { duration: 4000 });
            this.load();
          },
        });
      return;
    }
    assign();
  }

  // ── last connection + tenant-scoped password reset ────────────────────────

  // The moment is written in the cell; how long ago, on hover.
  protected lastSeen(ts: number): string {
    if (!ts) return '';
    return DateTime.fromSeconds(ts).reconfigure({ locale: this.locale }).toRelative() ?? '';
  }

  protected resetPassword(u: UserRow): void {
    const email = this.memberships().get(u.id)?.email ?? '';
    void resetAccess(this.api, this.dialog, this.snack, { username: u.username, email }, (byMail) =>
      this.api.resetMemberPassword(this.tenantId(), u.id, byMail),
    );
  }
}

function errMsg(err: unknown): string {
  const e = err as { error?: { error?: string } };
  return typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`;
}
