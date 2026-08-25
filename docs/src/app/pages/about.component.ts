import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-about',
  imports: [RouterLink],
  preserveWhitespaces: true,
  styles: [
    `
      pre {
        background: var(--bg-secondary);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        padding: 14px 16px;
        overflow-x: auto;

        code {
          font-family: 'Courier New', Consolas, monospace;
          font-size: 0.9em;
          color: var(--text-primary);
        }
      }
    `,
  ],
  template: `
    <h2>About</h2>

    <div class="callout warn">
      <strong>Specification phase.</strong> Meerkat is being designed in the open — the full
      requirements document lives in the repository and every structural decision is recorded
      there. Implementation starts with a Go walking skeleton. See the
      <a routerLink="/roadmap">roadmap</a>.
    </div>

    <p>
      Meerkat is an <strong>app-gateway</strong> — a gateway built to serve an
      <em>application</em>, not to expose APIs to third parties. It is the single entry point of
      an enterprise internal application composed of many services, and it takes charge of
      everything those services should never have to reimplement:
    </p>

    <ul>
      <li>
        <strong>Authentication</strong> — login pages served by the gateway, passwordless first
        (passkeys, TOTP, email OTP); enterprise methods (OIDC, LDAP, SAML) supported for
        authentication <em>only</em>.
      </li>
      <li>
        <strong>Authorization</strong> — roles and role groups live in Meerkat, enforced per
        route and per endpoint (from the service's OpenAPI spec, an uploaded one, or a recorded
        session).
      </li>
      <li>
        <strong>Multi-tenancy</strong> — organizations, members, groups, tenant switching,
        per-tenant session policies.
      </li>
      <li>
        <strong>Routing</strong> — a service catalog discovered in the cluster, dynamic routes
        edited hot, versioned configurations you can duplicate, diff, switch and roll back.
      </li>
      <li>
        <strong>Quotas, audit &amp; observability</strong> — built into the console. No
        Prometheus, no Grafana, no YAML required.
      </li>
      <li>
        <strong>Dev mode</strong> — with <a href="https://github.com/softwarity/plug"
        target="_blank" rel="noopener">plug</a>, a developer's workstation joins the cluster and
        substitutes a deployed service, and everyone looking at the application is told which
        one, and by whom. See <a routerLink="/dev-mode">Dev mode</a>.
      </li>
    </ul>

    <p>
      Your services stay lean: they receive requests already authenticated, carrying a signed
      JWT with identity, roles and tenant. And the whole gateway is
      <strong>one binary with zero dependency</strong> — embedded storage by default, an
      external database only when you want a HA cluster.
    </p>

    <h3>Try it</h3>
    <pre><code>docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat</code></pre>
    <p>
      8080 is what your users reach, 9090 is the admin console. See
      <a routerLink="/deploy">Deploy</a> for a compose file, a Helm chart, and what each
      variable decides.
    </p>

    <h3>Two editions, two licences</h3>
    <p>
      One commit, two images. The <strong>community</strong> one is on Docker Hub and public;
      the <strong>Enterprise</strong> one is on GitHub Packages and private. What separates them
      is not a flag read at startup, it is what the linker put in the binary - so a feature that
      is not sold is not in the image at all, and there is no licence file to check at runtime.
    </p>
    <ul>
      <li>
        <strong>The core is
        <a href="https://fsl.software/" target="_blank" rel="noopener">FSL-1.1-Apache-2.0</a></strong>
        - the Functional Source License. Read it, change it, run it, ship it inside your own
        product: the only thing you may not do is sell a competing gateway with it. Two years
        after each release, that version becomes plain <strong>Apache 2.0</strong>, with no
        conditions left at all. The published sources are
        <a href="https://github.com/softwarity/meerkat-ce" target="_blank" rel="noopener">softwarity/meerkat-ce</a>.
      </li>
      <li>
        <strong>The Enterprise code (<code>ee/</code>) is under a Softwarity commercial
        licence.</strong> It may be read, modified and contributed to; it may be USED - built
        into a running gateway, or carried into another work - only under a commercial
        agreement. It is not in the community sources, which is why the mirror is the whole
        community product rather than a crippled copy of another one.
      </li>
    </ul>
    <p>
      What falls on each side is a rule rather than a price list:
      <strong>never a security primitive</strong>. TLS, the vault, MFA, passkeys, the audit
      trail and endpoint security are in both images - selling those would be selling safety.
      Enterprise is scale and organisation: several organisations, external directories, the
      arrangements of the built-in pages, the developer tunnel embedded in the gateway.
    </p>

    <h3>Why “Meerkat”?</h3>
    <p>
      The meerkat is nature's sentinel: it stands guard at the burrow entrance and raises the
      alert, so the rest of the colony can work without worrying about anything. That is exactly
      what this gateway does for your services. Even the plug tunnel fits the picture — it is
      how a developer's machine digs its way into the burrow. And since a group of meerkats is
      called a <em>mob</em>, you already know what to call a cluster of Meerkat nodes.
    </p>

    <h3>Lineage</h3>
    <p>
      Meerkat is the successor of
      <a href="https://github.com/softwarity/archway" target="_blank" rel="noopener">Archway</a>
      (Spring Cloud Gateway, MongoDB, Angular), rebuilt from the ground up in Go on the lessons
      of that first implementation — everything it did well is kept, everything it left on the
      table (rate limiting, built-in audit, zero-dependency startup) is a requirement this time.
    </p>
  `,
})
export class AboutComponent {}
