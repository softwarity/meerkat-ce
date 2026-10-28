import { Component, computed, effect, inject, input, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { ApiService, NotificationReader, SentNotification } from '../../api.service';
import { MeService } from '../../me.service';
import { DialogsService } from '../../shared/dialogs.service';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { IdentityDrawer } from '../identity-drawer';
import { mailText } from '../../notifications/notify-levels';

// One notification sent to the tenant, in the tenant page's drawer (NOTIF-05):
// what it said, who was told and who read it, who acknowledged it when it
// asks, and its withdrawal. The tenant level's view of the Data plane
// screen's drawer, limited to this tenant.
@Component({
  selector: 'app-tenant-notification',
  imports: [MatButtonModule, MatIconModule, ZonedDatePipe],
  styleUrl: '../user-editor/user-editor.component.scss',
  styles: `
    .text {
      margin: 0;
      white-space: pre-wrap;
    }
    dl {
      display: grid;
      grid-template-columns: max-content 1fr;
      gap: 6px 16px;
      margin: 0;
    }
    dt {
      color: var(--mat-sys-on-surface-variant);
    }
    dd {
      margin: 0;
      min-width: 0;
    }
    .mono {
      font-family: 'JetBrains Mono', 'SF Mono', Consolas, monospace;
      font-size: 0.85em;
      word-break: break-all;
    }
    .muted {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85em;
    }
    .readers {
      list-style: none;
      margin: 0;
      padding: 0;
      display: grid;
      gap: 6px;
      align-self: stretch;
    }
    .readers li {
      display: flex;
      justify-content: space-between;
      gap: 12px;
    }
    .danger {
      color: var(--mat-sys-error);
    }
    .sub {
      margin: 4px 0 0;
      font-size: 0.85rem;
      font-weight: 500;
      color: var(--mat-sys-on-surface-variant);
    }
    .waiting {
      color: var(--mat-sys-primary);
    }
    .exercise {
      display: inline-block;
      margin-inline-end: 6px;
      padding: 0 6px;
      border-radius: 4px;
      font: var(--mat-sys-label-small);
      letter-spacing: 0.06em;
      vertical-align: middle;
      background: var(--mat-sys-tertiary-container);
      color: var(--mat-sys-on-tertiary-container);
    }
    .line {
      margin: 0;
    }
    /* An action: its label, then the call it makes and the body it sends. */
    .readers li.call {
      flex-direction: column;
      gap: 2px;
    }
    .payload {
      margin: 2px 0 0;
      padding: 6px 10px;
      border-radius: 8px;
      background: var(--mat-sys-surface-container);
      white-space: pre-wrap;
    }
  `,
  template: `
    <header>
      <h2>@if (notification().exercise) {<span class="exercise" i18n="@@Notify_exercise_label">EXERCISE</span>}{{ title() }}</h2>
      <span class="grow"></span>
      <button matIconButton (click)="drawer.close()" i18n-aria-label="@@Close" aria-label="Close">
        <mat-icon>close</mat-icon>
      </button>
    </header>
    <div class="body">
      @if (body()) {
        <p class="text">{{ body() }}</p>
      }
      <dl>
        <dt i18n="@@Sender">Sender</dt>
        <dd><span class="mono">{{ notification().sender }}</span></dd>
        @if (notification().ref) {
          <dt i18n="@@Id">Id</dt>
          <dd><span class="mono">{{ notification().ref }}</span></dd>
        }
        <dt i18n="@@Sent">Sent</dt>
        <dd class="nowrap">{{ notification().createdAt | zoned: zone() : "long" }}</dd>
        <dt i18n="@@Expires">Expires</dt>
        <dd class="nowrap">{{ notification().expiresAt | zoned: zone() : "long" }}</dd>
        @if (notification().ack; as mode) {
          <dt i18n="@@Notify_ack">Acknowledgement</dt>
          <dd>
            @if (mode === 'single') {
              <ng-container i18n="@@Notify_ack_single">One reader</ng-container>
            } @else {
              <ng-container i18n="@@Notify_ack_all">Every reader</ng-container>
            }
          </dd>
        }
      </dl>
      @if (notification().actions?.length) {
        <section>
          <h3 i18n="@@Actions">Actions</h3>
          <ul class="readers">
            @for (a of notification().actions; track $index) {
              <li class="call">
                <span>{{ labelOf(a.label) }}</span>
                <span class="mono">{{ a.method || 'GET' }} {{ a.url }}</span>
                @if (a.body !== undefined) {
                  <pre class="mono payload">{{ bodyOf(a.body) }}</pre>
                }
              </li>
            }
          </ul>
        </section>
      }
      @if (notification().ack; as mode) {
        <section>
          <h3>
            <ng-container i18n="@@Notify_acknowledged">Acknowledged</ng-container>
            {{ notification().acknowledged }} / {{ notification().recipients }}
          </h3>
          @if (mode === 'single') {
            @if (notification().ackCompleteAt) {
              <p class="line">
                <ng-container i18n="@@Notify_ack_closed_by">Closed for everyone by</ng-container>
                {{ closer() }},
                <span class="nowrap">{{ notification().ackCompleteAt | zoned: zone() : "long" }}</span>
              </p>
            } @else {
              <p class="line waiting" i18n="@@Notify_ack_single_pending">Pending: the first acknowledgement closes it for everyone.</p>
            }
          }
          @if (acked().length) {
            <ul class="readers">
              @for (r of acked(); track r.userId) {
                <li>
                  <span>{{ nameOf(r) }}</span>
                  <span class="muted nowrap">{{ r.ackedAt | zoned: zone() }}</span>
                </li>
              }
            </ul>
          }
          @if (mode === 'all' && missing().length) {
            <p class="sub">
              <ng-container i18n="@@Notify_ack_missing">Not acknowledged yet</ng-container>
              {{ missing().length }}
            </p>
            <ul class="readers">
              @for (r of missing(); track r.userId) {
                <li><span class="waiting">{{ nameOf(r) }}</span></li>
              }
            </ul>
          }
        </section>
      }
      <section>
        <h3>
          <ng-container i18n="@@Recipients">Recipients</ng-container>
          {{ notification().reads }} / {{ notification().recipients }}
        </h3>
        <ul class="readers">
          @for (r of readers(); track r.userId) {
            <li>
              <span>{{ r.username || r.userId }}</span>
              @if (r.mail) {
                <span class="muted">{{ mailText(r) }}</span>
              }
              @if (r.readAt) {
                <span class="muted nowrap">{{ r.readAt | zoned: zone() }}</span>
              } @else {
                <span class="muted" i18n="@@Not_read">Not read</span>
              }
            </li>
          }
        </ul>
      </section>
      @if (live()) {
        <div class="actions">
          <button matButton="outlined" class="danger" (click)="withdraw()" i18n="@@Withdraw">Withdraw</button>
        </div>
      }
    </div>
  `,
})
export class TenantNotificationComponent {
  readonly tenantId = input.required<string>();
  readonly notification = input.required<SentNotification>();

  private readonly api = inject(ApiService);
  private readonly dialogs = inject(DialogsService);
  protected readonly zone = inject(MeService).timezone;
  protected readonly drawer = inject(IdentityDrawer);
  protected readonly readers = signal<NotificationReader[]>([]);
  // Who acknowledged it, first first, and who has not.
  protected readonly acked = computed(() =>
    this.readers()
      .filter((r) => r.ackedAt)
      .sort((a, b) => (a.ackedAt ?? 0) - (b.ackedAt ?? 0)),
  );
  protected readonly missing = computed(() => this.readers().filter((r) => !r.ackedAt));
  // Who closed a single one for everyone.
  protected readonly closer = computed(() => {
    const by = this.notification().ackBy;
    return by ? by.name || by.username || by.id : '?';
  });

  constructor() {
    effect(() => {
      const n = this.notification();
      this.readers.set([]);
      this.api.tenantNotificationReaders(this.tenantId(), n.id).subscribe((r) => this.readers.set(r));
    });
  }

  protected nameOf(r: NotificationReader): string {
    return r.fullname ? `${r.fullname} (${r.username || r.userId})` : r.username || r.userId;
  }

  protected readonly mailText = mailText;

  protected title(): string {
    const t = this.notification().title;
    return t['en'] ?? t[''] ?? Object.values(t)[0] ?? '';
  }

  protected labelOf(l: Record<string, string>): string {
    return l['en'] ?? l[''] ?? Object.values(l)[0] ?? '';
  }

  // The JSON an action sends, laid out to be read.
  protected bodyOf(b: unknown): string {
    return JSON.stringify(b, null, 2);
  }

  protected body(): string {
    const b = this.notification().body ?? {};
    return b['en'] ?? b[''] ?? Object.values(b)[0] ?? '';
  }

  protected live(): boolean {
    const n = this.notification();
    return !n.withdrawnAt && n.expiresAt * 1000 > Date.now();
  }

  // Confirmed: it leaves every recipient's page and badge at once.
  protected async withdraw(): Promise<void> {
    const n = this.notification();
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Withdraw_notification_TITLE:Withdraw "${this.title()}:TITLE:"?`,
      message: $localize`:@@Withdraw_notification_message:It disappears at once for all its recipients. The list of who read it is kept.`,
      confirmLabel: $localize`:@@Withdraw:Withdraw`,
      danger: true,
    });
    if (!ok) return;
    this.api.withdrawTenantNotification(this.tenantId(), n.id).subscribe(() => {
      this.drawer.close();
      this.drawer.withdrawn.update((v) => v + 1);
    });
  }
}
