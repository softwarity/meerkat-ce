import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatListModule } from '@angular/material/list';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { firstValueFrom } from 'rxjs';
import { ApiService, HttpsSwitch, PlatformAuthorities, PlatformAuthority } from '../../api.service';
import { SnippetComponent } from '../../shared/snippet.component';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { MeService } from '../../me.service';

// The authorities the cluster publishes (SSL-09), found by the gateway rather
// than fetched by hand: the files Kubernetes and OpenShift mount, the
// cert-manager issuers, the trust-manager Bundles. Under each, what it signs
// here - the routes in https whose upstream it signed, the services the
// platform records it signed - and the one that signs a route refused today
// is picked in advance. Indicative: nothing is trusted without Trust.
//
// Taking one sends its reference only: the gateway reads it again from its
// source. Then a second step offers the routes still in http whose service now
// answers in https with a certificate the gateway accepts - tried by the
// gateway, port and name included - each to switch or leave.
//
// Closes with the number of authorities trusted.
@Component({
  selector: 'app-platform-trust-dialog',
  imports: [LoadingIndicatorComponent, MatButtonModule, MatCheckboxModule, MatDialogModule, MatIconModule, MatListModule, SnippetComponent, ZonedDatePipe],
  templateUrl: './platform-trust-dialog.component.html',
  styleUrl: './platform-trust-dialog.component.scss',
})
export class PlatformTrustDialogComponent {
  private readonly api = inject(ApiService);
  protected readonly zone = inject(MeService).timezone;
  private readonly ref = inject(MatDialogRef<PlatformTrustDialogComponent, number>);
  // The section's search, when it has one: opened on it, not asked again.
  private readonly given = inject<PlatformAuthorities | undefined>(MAT_DIALOG_DATA, { optional: true }) ?? undefined;

  protected readonly result = signal<PlatformAuthorities | undefined>(undefined);
  protected readonly picked = signal<string[]>([]);
  protected readonly taking = signal(false);
  // What a reference could not be taken for, by reference.
  protected readonly failed = signal<Record<string, string>>({});
  private taken = 0;

  // The second step: routes to switch to https, once an authority is taken.
  protected readonly switches = signal<HttpsSwitch[] | undefined>(undefined);
  protected readonly toSwitch = signal<string[]>([]);
  protected readonly switching = signal(false);
  protected readonly switchFailed = signal<Record<string, string>>({});

  // The notes worth saying: a source present but out of the gateway's rights.
  // An absent one is only said when nothing was found at all.
  protected readonly blocked = computed(() => (this.result()?.notes ?? []).filter((n) => !n.absent));

  protected readonly sourceLabel: Record<PlatformAuthority['source'], string> = {
    openshift: $localize`:@@Trust_src_openshift:OpenShift service CA`,
    'cert-manager': $localize`:@@Trust_src_cert_manager:cert-manager issuer`,
    'trust-manager': $localize`:@@Trust_src_trust_manager:trust-manager bundle`,
    cluster: $localize`:@@Trust_src_cluster:Kubernetes cluster CA: signs the API server, rarely the services`,
  };

  // What publishes an authority, when the cluster publishes none: a CA issuer
  // of cert-manager, the most common way services get their certificates.
  protected readonly example = `# An authority cert-manager signs the services with. Its certificate and
# key live in a Secret of cert-manager's namespace, which the gateway never
# reads: it learns the certificate from one the issuer signs.
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: company-ca
spec:
  ca:
    secretName: company-ca
`;

  constructor() {
    if (this.given) {
      this.show(this.given);
      return;
    }
    this.api.platformAuthorities().subscribe({
      next: (r) => this.show(r),
      error: () => this.result.set({ kubernetes: true, found: [], notes: [], unmatched: [] }),
    });
  }

  private show(r: PlatformAuthorities): void {
    this.result.set(r);
    this.picked.set(r.found.filter((a) => a.preselect).map((a) => a.ref));
  }

  protected routes(a: PlatformAuthority) {
    return a.signs.filter((s) => s.kind === 'route');
  }

  // One line per SERVICE, not per Certificate: a service may hold several
  // (Meerkat's internal plane has two here). Named by its Service in its
  // namespace, from the names the certificate carries: a bare name or a
  // <name>.<namespace>[.svc...] one says it; a public name (s3.example.com)
  // does not, and is shown whole only when nothing else is there.
  protected services(a: PlatformAuthority): string[] {
    const seen = new Set<string>();
    for (const s of a.signs) {
      if (s.kind !== 'service') continue;
      const ns = s.name.split('/')[0];
      const names = s.names ?? [];
      const inside = names.find((n) => !n.includes('.') || n.split('.')[1] === ns);
      seen.add(inside ? `${ns}/${inside.split('.')[0]}` : names[0] ? `${ns}/${names[0]}` : s.name);
    }
    return [...seen].sort();
  }

  protected toggle(ref: string, on: boolean): void {
    this.picked.update((p) => (on ? [...p, ref] : p.filter((r) => r !== ref)));
  }

  protected async take(): Promise<void> {
    this.taking.set(true);
    const failed: Record<string, string> = {};
    for (const ref of this.picked()) {
      try {
        await firstValueFrom(this.api.takePlatformAuthority(ref));
        this.taken++;
      } catch (e) {
        failed[ref] = (e as HttpErrorResponse).error?.error ?? $localize`:@@Request_failed:Request failed`;
      }
    }
    this.failed.set(failed);
    if (Object.keys(failed).length) {
      // Keep only what failed picked, to try again or leave.
      this.picked.set(Object.keys(failed));
      this.taking.set(false);
      return;
    }
    // Trusted: which routes in http could now go in https?
    const switches = await firstValueFrom(this.api.httpsSwitch()).catch(() => [] as HttpsSwitch[]);
    this.taking.set(false);
    if (!switches.length) {
      this.ref.close(this.taken);
      return;
    }
    this.switches.set(switches);
    this.toSwitch.set(switches.map((s) => s.routeId));
  }

  // Each route switched the way the route editor saves it: read, the target
  // changed, written back with the revision it was read at.
  protected async switchRoutes(): Promise<void> {
    this.switching.set(true);
    const failed: Record<string, string> = {};
    for (const s of this.switches() ?? []) {
      if (!this.toSwitch().includes(s.routeId)) continue;
      try {
        const route = await firstValueFrom(this.api.getRoute(s.routeId));
        await firstValueFrom(this.api.putRoute({ ...route, upstream: s.to }));
      } catch (e) {
        failed[s.routeId] = (e as HttpErrorResponse).error?.error ?? $localize`:@@Request_failed:Request failed`;
      }
    }
    this.switching.set(false);
    this.switchFailed.set(failed);
    if (!Object.keys(failed).length) this.ref.close(this.taken);
  }

  protected close(): void {
    this.ref.close(this.taken);
  }
}
