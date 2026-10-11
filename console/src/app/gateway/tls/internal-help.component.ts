import { Component, computed, effect, input, output, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { SnippetComponent } from '../../shared/snippet.component';

// Where a certificate for the inside of the platform comes from, per platform:
// how to have one made (or delegate it to the platform), how to mount it, and
// how the others come to trust its authority. Help rather than configuration,
// so a drawer, like the local-machine one.
//
// Every service of the cluster serves HTTPS the same way, so the templates
// are written for any of them, <service> and <namespace> left to fill in.
// "Only Meerkat" fills them with THIS gateway's names - its Service, its
// namespace, its release, read from the runtime - and adds what is its own:
// the chart value that mounts it, and its Internal door.
export type InternalPlatform = 'kubernetes' | 'openshift' | 'own-ca' | 'swarm' | 'compose';

@Component({
  selector: 'app-internal-help',
  imports: [MatButtonModule, MatButtonToggleModule, MatIconModule, MatSlideToggleModule, SnippetComponent],
  templateUrl: './internal-help.component.html',
  styleUrl: './internal-help.component.scss',
})
export class InternalHelpComponent {
  // The internal plane as the runtime names it, and which runtime it is.
  readonly origin = input('');
  readonly source = input('');
  readonly closed = output<void>();

  protected readonly platform = signal<InternalPlatform>('kubernetes');
  // The templates for any service, or filled in for this gateway.
  protected readonly only = signal(false);

  constructor() {
    // Opened on the platform this gateway runs on; a choice made here wins.
    effect(() => {
      const s = this.source();
      untracked(() =>
        this.platform.set(
          s === 'openshift' ? 'openshift' : s === 'swarm' ? 'swarm' : s === 'compose' || s === 'docker' ? 'compose' : 'kubernetes',
        ),
      );
    });
  }

  private readonly host = computed(() => {
    try {
      return new URL(this.origin()).hostname;
    } catch {
      return '';
    }
  });
  // On Kubernetes the host is <service>.<namespace>.svc.<domain>. The domain
  // is the cluster's, the same for every service: kept in the templates.
  private readonly self = computed(() => {
    const parts = this.host().split('.');
    const svcAt = parts.indexOf('svc');
    if (svcAt < 2) return { svc: 'meerkat-meerkat-internal', ns: 'default', domain: 'cluster.local' };
    return { svc: parts[0], ns: parts[1], domain: parts.slice(svcAt + 1).join('.') || 'cluster.local' };
  });
  private readonly k8s = computed(() =>
    this.only() ? this.self() : { svc: '<service>', ns: '<namespace>', domain: this.self().domain },
  );
  // The chart names the Service <release>-meerkat-internal.
  private readonly release = computed(() => this.self().svc.replace(/-meerkat-internal$/, '').replace(/-internal$/, ''));
  protected readonly k8sNames = computed(() => {
    const { svc, ns, domain } = this.k8s();
    return [svc, `${svc}.${ns}`, `${svc}.${ns}.svc`, `${svc}.${ns}.svc.${domain}`];
  });
  // On Swarm and compose the host is the service's name; a stack's service is
  // also reached by its short name inside the stack.
  protected readonly dockerNames = computed(() => {
    if (!this.only()) return this.platform() === 'swarm' ? ['<stack>_<service>', '<service>'] : ['<service>'];
    const h = this.host() || 'meerkat';
    const short = h.includes('_') ? h.slice(h.indexOf('_') + 1) : '';
    return short ? [h, short] : [h];
  });
  protected readonly secret = computed(() => `${this.k8s().svc}-tls`);
  // Where a service reads its pair: the gateway's own place, or one to choose.
  private readonly dir = computed(() => (this.only() ? '/etc/meerkat/tls/internal' : '/etc/tls'));

  private helm(set: string): string {
    return `helm upgrade ${this.release()} meerkat/meerkat -n ${this.self().ns} --reuse-values \\\n  --set ${set}`;
  }

  protected readonly certManager = computed(() => {
    const { ns } = this.k8s();
    return `# certificate.yaml - cert-manager issues it into a Secret, and renews it
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${this.secret()}
  namespace: ${ns}
spec:
  secretName: ${this.secret()}
  dnsNames:
${this.k8sNames().map((n) => `    - ${n}`).join('\n')}
  issuerRef:
    name: cluster-ca          # your ClusterIssuer
    kind: ClusterIssuer`;
  });
  protected readonly certManagerApply = computed(
    () => `kubectl apply -f certificate.yaml\n${this.helm(`internalTls.secretName=${this.secret()}`)}`,
  );
  // A service's own Deployment, mounting the Secret where it reads its pair.
  protected readonly k8sMount = computed(
    () => `# In the service's Deployment: the Secret, mounted read-only
spec:
  template:
    spec:
      volumes:
        - name: tls
          secret:
            secretName: ${this.secret()}
      containers:
        - name: ${this.k8s().svc}
          volumeMounts:
            - name: tls
              mountPath: ${this.dir()}
              readOnly: true`,
  );
  protected readonly trustManager = computed(
    () => `# bundle.yaml - trust-manager copies the authority into every namespace
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  name: cluster-ca
spec:
  sources:
    - secret:
        name: cluster-ca      # the Secret holding your issuer's ca.crt
        key: ca.crt
  target:
    configMap:
      key: ca.crt             # mount it in the services, and trust it`,
  );

  protected readonly openshift = computed(() => this.helm('internalTls.openshiftServingCert=true'));
  protected readonly openshiftAnnotate = computed(
    () => `# OpenShift's service CA issues the Secret for the Service, and renews it
oc -n ${this.k8s().ns} annotate service ${this.k8s().svc} \\
  service.beta.openshift.io/serving-cert-secret-name=${this.secret()}`,
  );
  protected readonly openshiftTrust = computed(
    () => `# In each calling service's namespace: OpenShift writes its service CA
# into the ConfigMap, under service-ca.crt. Mount it, and trust it.
apiVersion: v1
kind: ConfigMap
metadata:
  name: service-ca
  annotations:
    service.beta.openshift.io/inject-cabundle: "true"`,
  );

  private openssl(names: string[]): string {
    return `# An authority of your own, once. ca.key signs for any name: keep it safe.
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \\
  -keyout ca.key -out ca.crt -days 3650 -subj "/CN=Internal CA"

# The service's certificate, for the names the others call it by.
openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \\
  -keyout tls.key -out tls.csr -subj "/CN=${names[0]}"
openssl x509 -req -in tls.csr -CA ca.crt -CAkey ca.key -CAcreateserial \\
  -out tls.crt -days 365 \\
  -extfile <(printf "subjectAltName=${names.map((n) => `DNS:${n}`).join(',')}")`;
  }

  protected readonly ownCa = computed(() => {
    const { ns } = this.k8s();
    return `${this.openssl(this.k8sNames())}

kubectl -n ${ns} create secret tls ${this.secret()} --cert=tls.crt --key=tls.key
${this.only() ? this.helm(`internalTls.secretName=${this.secret()}`) + '\n' : ''}
# Renewing: sign again, then replace the Secret - a service that reads its
# files again serves it on its own, as Meerkat does.
kubectl -n ${ns} create secret tls ${this.secret()} --cert=tls.crt --key=tls.key \\
  --dry-run=client -o yaml | kubectl apply -f -`;
  });
  protected readonly ownCaTrust = computed(
    () => `# Hand ca.crt to the calling services, in their namespace, and trust it.
kubectl -n <their namespace> create configmap internal-ca --from-file=ca.crt`,
  );

  protected readonly swarm = computed(() => {
    const service = this.dockerNames()[0];
    const name = this.only() ? 'meerkat-internal' : '<service>';
    return `${this.openssl(this.dockerNames())}

docker secret create ${name}-crt tls.crt
docker secret create ${name}-key tls.key
docker service update \\
  --secret-add source=${name}-crt,target=${this.dir()}/tls.crt \\
  --secret-add source=${name}-key,target=${this.dir()}/tls.key \\
  ${service}

# A Swarm secret cannot change: renewing is a new pair of secrets and another
# update, which restarts the tasks.`;
  });

  protected readonly compose = computed(
    () => `${this.openssl(this.dockerNames())}
mkdir -p tls && mv tls.crt tls.key tls/

# docker-compose.yml - the folder, mounted read-only where the service reads it
services:
  ${this.dockerNames()[0]}:
    volumes:
      - ./tls:${this.dir()}:ro

# Renewing: write the new tls.crt and tls.key into ./tls.`,
  );
  protected readonly dockerTrust = `# Give ca.crt to the calling services - a volume, or a secret - and trust it.`;
}
