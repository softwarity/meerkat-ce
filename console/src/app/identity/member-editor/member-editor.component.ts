import { Component, computed, effect, inject, input, linkedSignal, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { forkJoin } from 'rxjs';
import { ApiService, BusinessAccess, Member, Membership } from '../../api.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { LiveChangesService } from '../../shared/live-changes.service';
import { BusinessAccessFormComponent } from '../business-access-form.component';
import { LoginHistoryComponent } from '../login-history.component';
import { IdentityDrawer, MemberEditorView } from '../identity-drawer';

// One member of an organisation, in the right drawer of the page that hosts
// the members matrix. Two pages, each opened by its own button on the
// member's row: the member's working hours in this organisation, and its
// sign-in history - no way from one to the other, the row has both.
@Component({
  selector: 'app-member-editor',
  imports: [
    MatButtonModule,
    MatIconModule,
    LoadingIndicatorComponent,
    BusinessAccessFormComponent,
    EeLockComponent,
    LoginHistoryComponent,
  ],
  templateUrl: './member-editor.component.html',
  styleUrl: '../user-editor/user-editor.component.scss',
})
export class MemberEditorComponent {
  readonly tenantId = input.required<string>();
  readonly userId = input.required<string>();
  // The page outside answers the click on the backdrop: it has to know
  // whether that click owes a sub-page or the whole drawer.
  readonly view = model<MemberEditorView>('member');

  private readonly api = inject(ApiService);
  protected readonly drawer = inject(IdentityDrawer);

  protected readonly member = signal<Member | null>(null);

  protected readonly heading = computed(() =>
    this.view() === 'history'
      ? $localize`:@@Sign_in_history:Sign-in history`
      : $localize`:@@Working_hours:Working hours`,
  );

  // The member's own hours, saved on the button: the grid emits on every
  // keystroke, and each one is not a decision.
  protected readonly hours = linkedSignal<BusinessAccess>(() => this.member()?.businessAccess ?? { inherited: true });
  protected readonly hoursDirty = computed(
    () => JSON.stringify(this.hours()) !== JSON.stringify(this.member()?.businessAccess ?? { inherited: true }),
  );

  // What "inherited" shows and seeds an override from: the next level that
  // sets hours - the account's own, else the organisation's, else the
  // gateway's (TENANT-04).
  private readonly organisation = signal<BusinessAccess>({ inherited: false });
  protected readonly above = computed<BusinessAccess>(() => {
    const own = this.member()?.accountHours;
    return own && own.inherited === false ? own : this.organisation();
  });

  protected readonly saving = signal(false);
  protected readonly error = signal('');

  constructor() {
    effect(() => this.load(this.tenantId(), this.userId()));
    effect(() => {
      const tenantId = this.tenantId();
      forkJoin([this.api.getTenant(tenantId), this.api.settings()]).subscribe(([t, s]) =>
        this.organisation.set(
          t.businessAccess?.inherited === false ? t.businessAccess : (s.businessAccess ?? { inherited: false }),
        ),
      );
    });
    // Somebody else's write, or this drawer's own save, read back.
    inject(LiveChangesService).on('membership', () => this.load(this.tenantId(), this.userId()));
  }

  private load(tenantId: string, userId: string): void {
    this.api.listMembers(tenantId).subscribe((members) => {
      const m = members.find((x) => x.userId === userId);
      if (m) this.member.set(m);
      else this.drawer.close(); // no longer a member: nothing left to show
    });
  }

  protected saveHours(): void {
    const m = this.member();
    if (!m) return;
    const { username, fullname, email, lastConnectionAt, accountHours, ...membership } = m;
    const next: Membership = { ...membership, businessAccess: this.hours() };
    this.saving.set(true);
    this.error.set('');
    this.api.putMember(next).subscribe({
      next: () => {
        this.saving.set(false);
        this.load(this.tenantId(), this.userId());
      },
      error: (e) => {
        this.saving.set(false);
        this.error.set(typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`);
      },
    });
  }
}
