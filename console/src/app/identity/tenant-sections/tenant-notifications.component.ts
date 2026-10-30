import { Component, computed, effect, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSlideToggleChange, MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, SentNotification, TenantNotify } from '../../api.service';
import { MeService } from '../../me.service';
import { DialogsService } from '../../shared/dialogs.service';
import { LiveChangesService } from '../../shared/live-changes.service';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { offByText } from '../../notifications/notify-levels';
import { IdentityDrawer } from '../identity-drawer';
import { TenantScope } from '../tenant-scope';

// The tenant's notifications (NOTIF-05), its administrator's level: a switch
// that can only take away what root and the application admin allowed, and
// what the services sent to this tenant, with who read it. Never who may
// publish nor the cap: those are the gateway's.
@Component({
  selector: 'app-tenant-notifications',
  imports: [
    MatButtonModule,
    MatIconModule,
    MatSlideToggleModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    ZonedDatePipe,
  ],
  templateUrl: './tenant-notifications.component.html',
  styleUrl: './tenant-notifications.component.scss',
})
export class TenantNotificationsComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  protected readonly scope = inject(TenantScope);
  protected readonly drawer = inject(IdentityDrawer);
  private readonly me = inject(MeService);
  // Times in the reader's own zone, as everywhere in the console.
  protected readonly zone = this.me.timezone;

  protected readonly view = signal<TenantNotify | null>(null);
  protected readonly list = signal<SentNotification[]>([]);
  protected readonly saving = signal(false);
  // Why the switch cannot be turned on: a level above turned it off.
  protected readonly reason = computed(() => {
    const v = this.view();
    return v?.state === 'offAbove' ? offByText(v.offBy) : '';
  });
  protected readonly columns = ['title', 'sender', 'reads', 'ack', 'sent'];

  constructor() {
    effect(() => {
      const t = this.scope.tenant();
      this.drawer.withdrawn();
      if (t) this.load(t.id);
    });
    // Sent, withdrawn or acknowledged somewhere else: the list, and the
    // notification open in the drawer, read again.
    if (this.me.mayWatch()) {
      inject(LiveChangesService).on('notification', () => {
        const t = this.scope.tenant();
        if (t) this.load(t.id);
      });
    }
  }

  private load(id: string): void {
    this.api.tenantNotify(id).subscribe((v) => this.view.set(v));
    this.api.tenantNotifications(id).subscribe((l) => {
      this.list.set(l);
      const open = this.drawer.notification();
      const fresh = open && l.find((n) => n.id === open.id);
      if (fresh && JSON.stringify(fresh) !== JSON.stringify(open)) this.drawer.openNotification(fresh);
    });
  }

  protected set(e: MatSlideToggleChange): void {
    const t = this.scope.tenant();
    if (!t) return;
    this.saving.set(true);
    this.api.setTenantNotify(t.id, e.checked).subscribe({
      next: (v) => {
        this.saving.set(false);
        this.view.set(v);
        this.snack.open(
          e.checked
            ? $localize`:@@Tenant_notifications_turned_on:Notifications on for this tenant`
            : $localize`:@@Tenant_notifications_turned_off:Notifications off for this tenant`,
          undefined,
          { duration: 2500 },
        );
      },
      error: (err) => {
        this.saving.set(false);
        e.source.checked = !e.checked;
        const msg = err?.error?.error;
        this.snack.open(typeof msg === 'string' ? msg : $localize`:@@Request_failed:Request failed`, undefined, {
          duration: 4000,
        });
      },
    });
  }

  protected title(n: SentNotification): string {
    return n.title['en'] ?? n.title[''] ?? Object.values(n.title)[0] ?? '';
  }

  // Where its acknowledgement stands, in one cell: who closed a single one,
  // how many of all gave theirs, or that it waits. A withdrawn or expired one
  // asks nothing any more.
  protected ack(n: SentNotification): { text: string; done: boolean } | null {
    if (!n.ack) return null;
    if (n.ack === 'single') {
      if (n.ackCompleteAt) {
        const who = n.ackBy?.username || n.ackBy?.id || '?';
        return { text: $localize`:@@Notify_ack_by_WHO:by ${who}:WHO:`, done: true };
      }
      return this.state(n) === 'live' ? { text: $localize`:@@Notify_ack_pending:Pending`, done: false } : null;
    }
    return { text: `${n.acknowledged} / ${n.recipients}`, done: !!n.ackCompleteAt };
  }

  // Why a notification can no longer be withdrawn or replaced: it ended.
  protected endedReason(n: SentNotification): string {
    switch (this.state(n)) {
      case 'withdrawn':
        return $localize`:@@Notify_off_withdrawn:Already withdrawn`;
      case 'replaced':
        return $localize`:@@Notify_off_replaced:Already replaced`;
      case 'expired':
        return $localize`:@@Notify_off_expired:Expired`;
    }
    return '';
  }

  // And a replacement names it by the id its sender gave it.
  protected replaceOff(n: SentNotification): string {
    return (
      this.endedReason(n) ||
      (n.ref ? '' : $localize`:@@Notify_off_no_id:Sent without an id: nothing names it to replace it`)
    );
  }

  // Confirmed, as from its drawer: it leaves every recipient's page and badge
  // at once.
  protected async withdraw(n: SentNotification): Promise<void> {
    const t = this.scope.tenant();
    if (!t) return;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Withdraw_notification_TITLE:Withdraw "${this.title(n)}:TITLE:"?`,
      message: $localize`:@@Withdraw_notification_message:It disappears at once for all its recipients. The list of who read it is kept.`,
      confirmLabel: $localize`:@@Withdraw:Withdraw`,
      danger: true,
    });
    if (!ok) return;
    this.api.withdrawTenantNotification(t.id, n.id).subscribe(() => {
      if (this.drawer.notification()?.id === n.id) this.drawer.close();
      this.drawer.withdrawn.update((v) => v + 1);
    });
  }

  protected state(n: SentNotification): 'live' | 'expired' | 'withdrawn' | 'replaced' {
    if (n.withdrawnAt) return n.endedAs === 'replaced' ? 'replaced' : 'withdrawn';
    return n.expiresAt * 1000 < Date.now() ? 'expired' : 'live';
  }
}
