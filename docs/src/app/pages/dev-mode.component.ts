import { Component } from '@angular/core';

@Component({
  selector: 'app-dev-mode',
  imports: [],
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
    <h2>Dev mode</h2>

    <p>
      The problem every internal-application team knows: the cluster holds all the services
      <strong>and all the data</strong>, and that environment is hard — sometimes forbidden — to
      reproduce on a developer's machine. Meerkat's answer is not to duplicate the environment
      but to bring the workstation <strong>into the routing mesh</strong>, like a tenant
      restricted to routing, powered by
      <a href="https://github.com/softwarity/plug" target="_blank" rel="noopener">plug</a>.
    </p>

    <h3>How it works</h3>
    <p>
      An admin flags a user as <code>dev</code>. The developer deposits their
      <strong>public SSH key</strong> on their Meerkat account (Profile &gt; Developer &gt;
      key), then runs their service locally through plug:
    </p>

    <pre><code>plug -p cluster --service user-mng-service npm run start</code></pre>

    <ul>
      <li>
        <strong>Workstation to cluster</strong>: the local process resolves and reaches the
        cluster services as if it ran inside the cluster (plug's original behaviour).
      </li>
      <li>
        <strong>Cluster to workstation</strong>: <code>--service</code> declares a
        <strong>substitution</strong> - traffic to <code>user-mng-service</code> goes through
        the reverse tunnel to the local process, which is seen, inbound and outbound, as a full
        member of the cluster. Every route referencing the service follows automatically.
      </li>
      <li>
        <strong>It lives as long as the session</strong>: the substitution vanishes when the
        process stops, and a gateway restart takes both ends with it.
      </li>
    </ul>

    <h3>A key, not a certificate</h3>
    <p>
      A certificate authority was examined and dropped. A certificate carries its own validity:
      once issued it is accepted until it expires, so taking it back means waiting or standing
      up revocation machinery. A gateway is up by definition - it can answer
      <em>is this key still allowed</em> at every single connection. So
      <strong>revoking is deleting the line</strong>, and someone who leaves stops being able to
      tunnel within seconds. The profile page shows the SHA256 fingerprint OpenSSH itself
      prints, and no expiry date: the absence is the design.
    </p>

    <h3>Nobody picks a variant - everybody is told</h3>
    <p>
      There is no menu of developers to choose from, and no tester role. A substitution changes
      what the application in front of you <em>is</em>, so it is news for everyone looking at
      it: Meerkat's job is to make the current state <strong>visible and attributed</strong> -
      "checkout, by Alice" rather than "checkout, by somebody" - not to offer a choice nobody
      can make correctly.
    </p>
    <p>
      That attribution is what the Enterprise edition adds. Standalone plug holds one key baked
      into the published binary, so a connection proves the caller <em>has</em> plug, not who
      they are - which is honest, and enough for the trusted clusters it targets.
    </p>

    <div class="callout">
      <strong>Community and Enterprise.</strong> The tunnel embedded in the gateway is
      Enterprise. plug itself stays a product of its own: the community image runs it as a
      standalone agent beside the gateway, which is plug's own default and needs nothing from
      Meerkat. See <a href="/deploy">Deploy</a> for the compose file and the Helm values that
      open it.
    </div>

    <h3>Still to come</h3>
    <p>
      The state is served but not yet shown: a page after login naming what is substituted, a
      strip that stays true while you work, a console screen listing the live sessions, and the
      audit trail of every key deposited and every substitution posed.
    </p>
  `,
})
export class DevModeComponent {}
