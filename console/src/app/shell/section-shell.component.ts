import { Component, inject } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { ActivatedRoute, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

// One section entry in a plane's left nav. The path is RELATIVE to the plane,
// so /infra/routes and /application/users need no repetition here.
interface SectionLink {
  path: string;
  label: string;
  icon: string;
  roles?: string;
  // singleOnly: the entry belongs to an installation with ONE, unnamed
  // organisation. In multi mode the same screens are reached per organisation,
  // from Tenants - see _modes.scss.
  singleOnly?: boolean;
}

const PLANES: Record<string, { title: string; links: SectionLink[] }> = {
  infra: {
    title: $localize`:@@Infra:Infra`,
    links: [
      { path: 'routes', label: $localize`:@@Routes:Routes`, icon: 'alt_route' },
      {
        path: 'endpoint-security',
        label: $localize`:@@Endpoint_security:Endpoint security`,
        icon: 'security',
      },
      {
        path: 'auth-providers',
        label: $localize`:@@Authentication:Authentication`,
        icon: 'passkey',
      },
      { path: 'mail-relay', label: $localize`:@@Mail_relay:Mail relay`, icon: 'outgoing_mail' },
      { path: 'tls', label: $localize`:@@TLS:TLS`, icon: 'lock' },
      {
        path: 'access-tokens',
        label: $localize`:@@Access_tokens:Access tokens`,
        icon: 'key',
        roles: 'root',
      },
      {
        path: 'configuration',
        label: $localize`:@@Configuration:Configuration`,
        icon: 'import_export',
        roles: 'root',
      },
    ],
  },
  application: {
    title: $localize`:@@Application:Application`,
    links: [
      { path: 'general', label: $localize`:@@Section_general:General`, icon: 'tune' },
      { path: 'locales', label: $localize`:@@Locales:Locales`, icon: 'translate' },
      { path: 'roles', label: $localize`:@@Roles:Roles`, icon: 'badge' },
      // Groups, Members and the group rules hang off an ORGANISATION. In single
      // mode there is one and nobody names it, so they live here; in multi they
      // go away and are reached per organisation, from Tenants. Users stays in
      // both: an account is global, it is the MEMBERSHIP that belongs to an
      // organisation.
      { path: 'groups', label: $localize`:@@Groups:Groups`, icon: 'groups', singleOnly: true },
      { path: 'users', label: $localize`:@@Users:Users`, icon: 'group' },
      { path: 'members', label: $localize`:@@Members:Members`, icon: 'badge_check', singleOnly: true },
      { path: 'group-rules', label: $localize`:@@Group_rules:Group rules`, icon: 'rule', singleOnly: true },
      // The pages this gateway serves: one entry, three tabs on its left panel
      // (theme, layout, branding) and ONE preview beside them. Three entries
      // for three jobs that all describe the same page sent three ways to look
      // at the same screen.
      {
        path: 'built-in-pages',
        label: $localize`:@@Built_in_pages:Built-in pages`,
        icon: 'wallpaper',
      },
      { path: 'security', label: $localize`:@@Security:Security`, icon: 'shield' },
    ],
  },
};

// The console shell: a plane's sections live in a LEFT NAV inside the page, the
// same shape a tenant already had, rather than in a drawer sliding out of the
// rail. Sections are CHILD routes (/infra/routes, /application/users), so the
// URL says which plane one is in and this shell stays mounted while moving
// between sections. Which plane it serves comes from the route's `plane` data -
// the router already knows, so there is no URL to parse.
//
// The transverse screens (vault, audit) sit outside: they belong to no plane.
// So does a tenant, which brings its own nav.
@Component({
  selector: 'app-section-shell',
  imports: [MatIconModule, RouterLink, RouterLinkActive, RouterOutlet],
  styleUrl: './section-shell.component.scss',
  templateUrl: './section-shell.component.html',
})
export class SectionShellComponent {
  private readonly plane = (inject(ActivatedRoute).snapshot.data['plane'] as string) ?? '';

  protected readonly sections: SectionLink[] = PLANES[this.plane]?.links ?? [];
  protected readonly title = PLANES[this.plane]?.title ?? '';
}
