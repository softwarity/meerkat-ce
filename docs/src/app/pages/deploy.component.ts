import { Component } from '@angular/core';

@Component({
  selector: 'app-deploy',
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
          font-size: 0.86em;
          color: var(--text-primary);
          white-space: pre;
        }
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 16px 0;
      }
      th,
      td {
        text-align: left;
        padding: 8px 10px;
        border-bottom: 1px solid var(--border-color);
        vertical-align: top;
      }
      th {
        font-size: 0.8em;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        color: var(--text-secondary);
      }
    `,
  ],
  template: `
    <h2>Deploy</h2>

    <p>
      One binary, two ports. <strong>8080 is the application plane</strong> - what your users
      reach. <strong>9090 is the control plane</strong> - the admin console. Only the first one
      belongs on the internet, and they are two Services in the chart for exactly that reason:
      an Ingress in front of your applications must not be able to reach the console by
      accident.
    </p>

    <p>
      Everything the gateway knows - routes, accounts, the sealed vault, the certificates -
      lives in one directory. Give it a volume and you have made the gateway restartable; back
      that volume up and you have backed up the gateway.
    </p>

    <h3>Docker Compose</h3>
    <p>The community image, in full:</p>
    <pre><code>services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080"   # application plane
      - "9090:9090"   # control plane - keep this one internal
    environment:
      # Read ONCE, on the first start, to create the admin account.
      MEERKAT_ADMIN_PASSWORD: your-first-password
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:</code></pre>

    <h3>Opening the developer tunnel (Enterprise)</h3>
    <p>
      The tunnel plugs a developer's machine into your stack: a service they run on their laptop
      answers, for them, under its cluster name, and everything else goes on talking to it as if
      it were still deployed. It is
      <a href="https://github.com/softwarity/plug" target="_blank" rel="noopener">plug</a>
      compiled into the gateway - the community image runs plug beside the gateway instead,
      which is plug's own default and needs nothing from Meerkat.
    </p>
    <p>
      The Enterprise file is the one above plus three things: the Enterprise image, a port, and
      the socket that lets the agent give a name to a machine.
    </p>
    <pre><code>services:
  meerkat:
    image: ghcr.io/softwarity/meerkat-ee:latest
    ports:
      - "8080:8080"
      - "9090:9090"
      - "22222:22222"   # the developer tunnel
    environment:
      MEERKAT_ADMIN_PASSWORD: your-first-password
      # MEERKAT_PRODUCTION: "1"   # see below
    volumes:
      - meerkat-data:/data
      - /var/run/docker.sock:/var/run/docker.sock</code></pre>
    <div class="callout">
      <strong>The socket is a real grant.</strong> Whoever can act as this container can act on
      this Docker daemon. Mount it on the clusters your developers already trust, and nowhere
      else. Without it the gateway runs exactly as before: the tunnel writes one line saying
      which resource it lacks, and serves as usual.
    </div>

    <h3>Kubernetes, with Helm</h3>
    <pre><code>helm install meerkat ./deploy/helm/meerkat \\
  --set admin.password='your-first-password'

# Enterprise, with the tunnel open:
helm install meerkat ./deploy/helm/meerkat \\
  --set image.repository=ghcr.io/softwarity/meerkat-ee \\
  --set admin.password='your-first-password' \\
  --set plug.enabled=true</code></pre>
    <p>
      <code>plug.enabled</code> is what grants this gateway's ServiceAccount the right to
      <strong>manage Services in its own namespace</strong> - and nothing else. That is what
      lets a session point a cluster name at a developer's machine and put it back afterwards.
      The agent cannot read a secret, touch a pod, or see another namespace. Off, the chart
      grants nothing at all.
    </p>

    <h3>Declaring a gateway production</h3>
    <p>
      <code>MEERKAT_PRODUCTION</code> (<code>production: true</code> in the chart) closes the
      whole developer surface here - the tooling, the API docs, the UI test mode, the tunnel -
      whatever the database says.
    </p>
    <p>
      It is worth setting even where nobody expects a tunnel, and the reason is the interesting
      one: <strong>the stored switch travels</strong>. A configuration export, a restored
      backup, a database copied from staging can each carry developer mode ON into production,
      and nobody notices until there is a tunnel port in front of customers. The environment
      does not travel - it is a property of where the process runs.
    </p>
    <p>
      It only goes one way: it closes a surface, it never opens one an operator turned off. The
      console shows the switch disabled <em>with the reason</em> rather than hiding it, because
      someone looking for tooling that is not there has to learn why.
    </p>

    <h3>What each variable does</h3>
    <table>
      <tr>
        <th>Variable</th>
        <th>Default</th>
        <th>What it decides</th>
      </tr>
      <tr>
        <td><code>MEERKAT_ADMIN_PASSWORD</code></td>
        <td>-</td>
        <td>The first admin account, on the FIRST start only. Never read again.</td>
      </tr>
      <tr>
        <td><code>MEERKAT_DATA</code></td>
        <td><code>data</code></td>
        <td>Where the gateway keeps everything it knows.</td>
      </tr>
      <tr>
        <td><code>MEERKAT_ADDR</code> / <code>MEERKAT_ADMIN_ADDR</code></td>
        <td><code>:8080</code> / <code>:9090</code></td>
        <td>The two planes.</td>
      </tr>
      <tr>
        <td><code>MEERKAT_TLS_ADDR</code> / <code>MEERKAT_ADMIN_TLS_ADDR</code></td>
        <td><code>:8443</code> / <code>:9443</code></td>
        <td>
          The HTTPS doors, opened when a certificate exists for a name. Having one IS the
          activation - there is no switch that could say "on" while nothing is served.
        </td>
      </tr>
      <tr>
        <td><code>MEERKAT_PLUG_ADDR</code></td>
        <td><code>:22222</code></td>
        <td>
          Moves the developer tunnel's port. It does not turn it off: closing the developer
          surface does.
        </td>
      </tr>
      <tr>
        <td><code>MEERKAT_PRODUCTION</code></td>
        <td>unset</td>
        <td>Declares this gateway production and closes the developer surface for good.</td>
      </tr>
      <tr>
        <td><code>MEERKAT_TENANCY</code></td>
        <td><code>single</code></td>
        <td>
          One implicit organisation, or several (Enterprise). Chosen once, at the first start;
          the console owns it afterwards.
        </td>
      </tr>
    </table>
  `,
})
export class DeployComponent {}
