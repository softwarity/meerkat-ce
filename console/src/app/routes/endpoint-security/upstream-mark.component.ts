import { Component, computed, input } from '@angular/core';
import { MatTooltipModule } from '@angular/material/tooltip';
import { UpstreamAccess } from '../../api.service';

// The roles the APPLICATION declares for one operation, as its OpenAPI spec
// says them. Only the roles: they are what the route maps onto the catalogue,
// so they are the one thing here somebody acts on. What a document says about
// a credential in general ("signed in", "public") is no fact about the running
// application - a stock spec declares basic auth on everything whatever the
// deployment does - and beside Meerkat's own rule it read as a contradiction.
// Read-only, and drawn apart from
// Meerkat's own access badges on purpose - dashed, in the tertiary tone, never
// the orange of a gateway rule - because the two answer different questions:
// the badges are what Meerkat refuses at the door, this is what the
// application will check itself once the call is through. Reading one as the
// other is how somebody believes a route is protected twice when it is once.
//
// A declared role mapped onto a catalogue role says which one, since that is
// what decides whether Auth forward sends it: an unmapped role is never sent,
// so it is drawn faded.
@Component({
  selector: 'app-upstream-mark',
  imports: [MatTooltipModule],
  template: `
    @for (r of rows(); track r.declared) {
      <span class="up role" [class.unmapped]="!r.target" [matTooltip]="r.tip">
        {{ r.declared }}
        @if (detailed() && r.target) {
          <span class="to" i18n="@@Upstream_mapped_to">mapped to {{ r.target }}</span>
        }
      </span>
    }
  `,
  styles: [
    `
      :host {
        display: inline-flex;
        flex-wrap: wrap;
        gap: 4px;
      }
      .up {
        display: inline-block;
        padding: 1px 8px;
        border-radius: 999px;
        border: 1px dashed var(--mat-sys-tertiary);
        color: var(--mat-sys-tertiary);
        font-size: 0.7rem;
        white-space: nowrap;
      }
      .role {
        font-family: var(--mk-mono);
      }
      .unmapped {
        opacity: 0.55;
      }
      .to {
        margin-left: 6px;
        padding-left: 6px;
        border-left: 1px solid var(--mat-sys-outline-variant);
        color: var(--mat-sys-on-surface-variant);
        font-family: var(--mk-mono);
      }
    `,
  ],
})
export class UpstreamMarkComponent {
  readonly upstream = input<UpstreamAccess | undefined>(undefined);
  // The route's saved mapping: declared role to catalogue role.
  readonly roleMap = input<Record<string, string> | undefined>(undefined);
  // In the drawer the mapping is written out; in the table it is one hover away.
  readonly detailed = input(false);

  protected readonly appChecks = $localize`:@@Upstream_declared_by_app:Declared in the application's OpenAPI spec. Meerkat does not check it.`;

  protected readonly rows = computed(() => {
    const map = this.roleMap() ?? {};
    return (this.upstream()?.roles ?? []).map((declared) => {
      const target = map[declared] ?? '';
      const how = target
        ? $localize`:@@Upstream_role_mapped:${declared}:ROLE:, mapped to ${target}:TARGET:.`
        : $localize`:@@Upstream_role_unmapped:${declared}:ROLE:, not mapped: never sent to the application.`;
      return { declared, target, tip: how + ' ' + this.appChecks };
    });
  });
}
