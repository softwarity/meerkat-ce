import { Component, inject, signal } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { ApiService, PlugPeople } from '../api.service';
import { LiveChangesService } from '../shared/live-changes.service';
import { PlugSessionsComponent } from './plug-sessions.component';

// The people of the developer tunnel (DEV-13): Application, Plug.
//
// The tunnel itself is infrastructure (Infra, Plug: the switch, the address,
// the commands). Who uses it is the application's: the developer capability is
// granted on Application, Users, withdrawing it already ends what the key
// holds, and so the same administrator sees who is plugged in and ends a run.
// The entry shows only in the Enterprise image with the tunnel open (the
// `plug-open` class, _modes.scss); reached anyway, the page says why it is
// empty.
@Component({
  selector: 'app-plug-people-page',
  imports: [MatCardModule, MatIconModule, PlugSessionsComponent],
  styleUrl: './plug-people-page.component.scss',
  template: `
    <div class="banner">
      <h1 i18n="@@Plug">Plug</h1>
    </div>
    <div class="content">
      @if (people(); as p) {
        @if (!p.enabled) {
          <p class="hint" i18n="@@Plug_people_closed">
            The developer tunnel is closed. An infrastructure administrator can open it in Infra, Plug.
          </p>
        } @else {
          <mat-card appearance="outlined">
            <app-plug-sessions />
          </mat-card>
        }

        <mat-card appearance="outlined">
          <!-- Who could plug in. The capability is granted on Users; the key
               is theirs to deposit - so the list says who still has to. -->
          <h3 class="group" i18n="@@Plug_group_developers">Developers</h3>
          @if (p.developers.length === 0) {
            <p class="hint" i18n="@@Plug_no_developers_app">
              No user has the developer capability yet. Grant it in Application, Users.
            </p>
          } @else {
            <ul class="devs">
              @for (d of p.developers; track d.username) {
                <li>
                  <mat-icon [class.on]="d.keys.length > 0">{{ d.keys.length ? 'key' : 'key_off' }}</mat-icon>
                  <span class="who">{{ d.fullname || d.username }}</span>
                  @for (k of d.keys; track k.fingerprint) {
                    <code class="fp" [title]="k.comment ?? ''">{{ k.fingerprint }}</code>
                  } @empty {
                    <span class="muted" i18n="@@Plug_no_key">no key added yet</span>
                  }
                </li>
              }
            </ul>
          }
        </mat-card>
      }
    </div>
  `,
})
export class PlugPeoplePageComponent {
  private readonly api = inject(ApiService);
  protected readonly people = signal<PlugPeople | undefined>(undefined);

  constructor() {
    this.load();
    // A capability granted or taken (user), the tunnel opened or closed
    // (settings).
    const live = inject(LiveChangesService);
    live.on('user', () => this.load());
    live.on('settings', () => this.load());
  }

  private load(): void {
    this.api.plugDevelopers().subscribe({ next: (p) => this.people.set(p) });
  }
}
