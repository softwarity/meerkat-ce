import { Component, inject, input } from '@angular/core';
import { GroupEditorComponent } from './group-editor/group-editor.component';
import { IdentityDrawer } from './identity-drawer';
import { MemberEditorComponent } from './member-editor/member-editor.component';
import { TenantNotificationComponent } from './tenant-sections/tenant-notification.component';
import { TenantNotificationComposeComponent } from './tenant-sections/tenant-notification-compose.component';

// What the page's identity drawer shows: the member, the group or the
// notification it was opened on. Placed inside the page's <mat-drawer>, so every page hosting a
// members or groups matrix draws the same drawer the same way.
@Component({
  selector: 'app-identity-drawer-panel',
  imports: [GroupEditorComponent, MemberEditorComponent, TenantNotificationComponent, TenantNotificationComposeComponent],
  styles: `
    :host {
      display: block;
      height: 100%;
    }
  `,
  template: `
    @if (drawer.userId(); as userId) {
      <app-member-editor [tenantId]="tenantId()" [userId]="userId" [(view)]="drawer.view" />
    } @else if (drawer.groupId(); as groupId) {
      <app-group-editor [tenantId]="tenantId()" [groupId]="groupId" />
    } @else if (drawer.notification(); as n) {
      @defer {
        <app-tenant-notification [tenantId]="tenantId()" [notification]="n" />
      }
    } @else if (drawer.composing()) {
      @defer {
        <app-tenant-notification-compose [tenantId]="tenantId()" />
      }
    }
  `,
})
export class IdentityDrawerPanelComponent {
  readonly tenantId = input.required<string>();
  protected readonly drawer = inject(IdentityDrawer);
}
