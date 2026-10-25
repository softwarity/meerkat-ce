import { computed, Injectable, signal } from '@angular/core';
import { SentNotification } from '../api.service';

// Which page of the member drawer is on screen.
export type MemberEditorView = 'member' | 'history';

// What the drawer holds: a member of the organisation, one of its groups, or
// a notification sent to it.
type DrawerItem =
  | { kind: 'member'; userId: string }
  | { kind: 'group'; groupId: string }
  | { kind: 'notification'; notification: SentNotification }
  | { kind: 'compose' };

// The identity drawer's state, provided by the PAGE that hosts the members or
// groups matrix: the drawer is drawn at the page's level, over its whole
// height as the account drawer is, and a matrix deep inside it only opens it.
// One drawer, whose content changes: two panels on the same edge is a
// position Material refuses.
@Injectable()
export class IdentityDrawer {
  private readonly item = signal<DrawerItem | null>(null);
  readonly view = signal<MemberEditorView>('member');

  readonly userId = computed(() => {
    const i = this.item();
    return i?.kind === 'member' ? i.userId : null;
  });
  readonly groupId = computed(() => {
    const i = this.item();
    return i?.kind === 'group' ? i.groupId : null;
  });
  readonly notification = computed(() => {
    const i = this.item();
    return i?.kind === 'notification' ? i.notification : null;
  });
  // The notification builder.
  readonly composing = computed(() => this.item()?.kind === 'compose');
  readonly opened = computed(() => this.item() !== null);
  // Bumped when the drawer withdrew or sent a notification: the list re-reads.
  readonly withdrawn = signal(0);

  openMember(userId: string, view: MemberEditorView = 'member'): void {
    this.view.set(view);
    this.item.set({ kind: 'member', userId });
  }

  openGroup(groupId: string): void {
    this.item.set({ kind: 'group', groupId });
  }

  openNotification(notification: SentNotification): void {
    this.item.set({ kind: 'notification', notification });
  }

  openCompose(): void {
    this.item.set({ kind: 'compose' });
  }

  close(): void {
    this.item.set(null);
  }

  // The click outside: no sub-page to give back, each page of the member
  // drawer has its own button on the row.
  back(): void {
    this.close();
  }

  // An open overlay - a dialog, a select's panel - owns the key first.
  escape(): void {
    // Anywhere in the page: a select's panel is drawn as a popover beside
    // its field, not in the overlay container.
    if (document.querySelector('.cdk-overlay-pane')) return;
    if (!this.opened()) return;
    this.back();
  }
}
