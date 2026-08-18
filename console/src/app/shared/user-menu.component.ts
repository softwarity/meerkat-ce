import { Component, computed, inject } from '@angular/core';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RailnavItemComponent } from '@softwarity/rail-nav';
import { SessionWatchService } from '../session';
import { ApiService } from '../api.service';
import { MeService } from '../me.service';

// The rail's bottom entry: who you are (from the identity the gateway stamps
// on <body>), the way to one's own profile, and sign out.
//
// No language submenu: the console is an operator's tool served in English at
// the root, with no locale segment (there are no per-locale builds to switch
// to). The menu kept offering French long after the build that served it was
// gone, and choosing it walked into a URL nobody serves.
@Component({
  selector: 'app-user-menu',
  imports: [MatDividerModule, MatIconModule, MatMenuModule, MatTooltipModule, RailnavItemComponent],
  styles: [
    `
      .avatar {
        display: grid;
        place-items: center;
        width: 26px;
        height: 26px;
        border-radius: 50%;
        background: var(--mat-sys-primary);
        color: var(--mat-sys-on-primary);
        font-size: 0.7rem;
        font-weight: 700;
        letter-spacing: 0.02em;
      }
      .who {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        color: inherit;
        text-decoration: none;
      }
      .who:hover {
        background: var(--mat-sys-surface-container-high);
      }
      .who .go {
        color: var(--mat-sys-on-surface-variant);
      }
      .who .avatar {
        width: 36px;
        height: 36px;
        font-size: 0.85rem;
      }
      .who .lines {
        display: grid;
        line-height: 1.3;
        flex: 1;
        min-width: 0;
      }
      .who .name {
        font-weight: 600;
      }
      .who .email {
        font-size: 0.78rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <rail-nav-item [label]="username()" [matMenuTriggerFor]="userMenu">
      @if (initials()) {
        <span class="avatar">{{ initials() }}</span>
      } @else {
        <mat-icon>person</mat-icon>
      }
    </rail-nav-item>
    <mat-menu #userMenu="matMenu">
      <!-- Who you are IS the way to your profile: the line said the name and
           the address, and a "My profile" entry underneath repeated it as a
           destination. Whoever administers the gateway is a user too - a
           photo, a passkey, a second factor, an address - and the pages that
           do all that are the gateway's own, served on this port as on the
           other, because they are the SAME pages an end user gets. -->
      @if (user(); as u) {
        <a class="who" href="/profile" i18n-matTooltip="@@My_profile" matTooltip="My profile">
          <span class="avatar">{{ initials() }}</span>
          <span class="lines">
            <span class="name">{{ u.fullname || u.username }}</span>
            @if (u.email) {
              <span class="email">{{ u.email }}</span>
            }
          </span>
          <mat-icon class="go">chevron_right</mat-icon>
        </a>
        <mat-divider />
      }
      <button mat-menu-item (click)="logout()">
        <mat-icon>logout</mat-icon>
        <span i18n="@@Sign_out">Sign out</span>
      </button>
    </mat-menu>
  `,
})
export class UserMenuComponent {
  private readonly api = inject(ApiService);
  private readonly watch = inject(SessionWatchService);

  protected readonly user = inject(MeService).user;
  protected readonly username = computed(() => this.user()?.username ?? '');
  protected readonly initials = computed(() => {
    const u = this.user();
    if (!u) return '';
    const parts = (u.fullname || u.username).trim().split(/\s+/);
    return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? parts[0]?.[1] ?? '')).toUpperCase();
  });

  protected logout(): void {
    // The other tabs of this console lose the session at the same instant, and
    // without the message they would go on looking signed in until their next
    // call failed.
    const leave = (): void => {
      this.watch.signedOut();
      location.href = '/login';
    };
    this.api.logout().subscribe({ next: leave, error: leave });
  }
}
