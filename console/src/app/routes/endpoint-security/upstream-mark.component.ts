import { Component, computed, input } from '@angular/core';
import { MatTooltipModule } from '@angular/material/tooltip';
import { UpstreamAccess } from '../../api.service';

// What the APPLICATION requires for one operation, as its OpenAPI spec says
// it: its roles, "Signed in" or "Public". Read-only, and drawn apart from
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
    @switch (upstream()?.access) {
      @case ('public') {
        <span class="up" [matTooltip]="appChecks" i18n="@@Upstream_public">Public</span>
      }
      @case ('authenticated') {
        <span class="up" [matTooltip]="appChecks" i18n="@@Upstream_signed_in">Signed in</span>
      }
      @case ('roles') {
        @for (r of rows(); track r.declared) {
          <span class="up role" [class.unmapped]="!r.target" [matTooltip]="r.tip">
            {{ r.declared }}
            @if (detailed() && r.target) {
              <span class="to" i18n="@@Upstream_mapped_to">mapped to {{ r.target }}</span>
            }
          </span>
        }
      }
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

  protected readonly appChecks = $localize`:@@Upstream_checked_by_app:The application checks this itself, Meerkat does not.`;

  protected readonly rows = computed(() => {
    const map = this.roleMap() ?? {};
    return (this.upstream()?.roles ?? []).map((declared) => {
      const target = map[declared] ?? '';
      const how = target
        ? $localize`:@@Upstream_role_mapped:${declared}:ROLE:, mapped to ${target}:TARGET:.`
        : $localize`:@@Upstream_role_unmapped:${declared}:ROLE:, not mapped: never forwarded.`;
      return { declared, target, tip: how + ' ' + this.appChecks };
    });
  });
}
