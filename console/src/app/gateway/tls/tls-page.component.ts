import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { firstValueFrom } from 'rxjs';
import { AcmeSettings, ApiService, Certificate, Coverage, TlsSettings } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { SecretFieldComponent } from '../../shared/secret-field.component';
import {
  CertificateDialogComponent,
  CertificateDialogData,
  CertificateDialogResult,
  CertificateDoor,
} from './certificate-dialog.component';

// TLS (SSL-01 to SSL-03, SSL-05).
//
// The screen is built around the question an operator actually has, which is
// never "which certificates do I own" but "will my console answer, and will my
// application answer". So it is two zones, one per plane, and each one starts
// from the NAMES that plane must answer to.
//
// The two are not symmetrical, and that is the whole reason they are apart: a
// console has ONE name, an application gateway fronts MANY - that is its job.
// A single flat list left the operator doing the matching in their head.
@Component({
  selector: 'app-tls-page',
  imports: [
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatMenuModule,
    MatSelectModule,
    MatSlideToggleModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    FormFieldComponent,
    SecretFieldComponent,
  ],
  styleUrl: './tls-page.component.scss',
  templateUrl: './tls-page.component.html',
})
export class TlsPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);
  private readonly locale = inject(LOCALE_ID);

  protected readonly loading = signal(true);
  protected readonly saving = signal(false);
  protected readonly certificates = signal<Certificate[]>([]);
  protected readonly tls = signal<TlsSettings | null>(null);

  // The name lists, edited as text: one per line is how an operator holds a
  // list of hosts in their head, and a chip editor for three entries is more
  // ceremony than the three entries are worth.
  protected readonly consoleNames = signal('');
  protected readonly appNames = signal('');

  protected readonly acme = signal<AcmeSettings>(blankAcme());
  protected readonly eabSecretSet = signal(false);

  protected readonly directories = [
    { value: '', label: $localize`:@@Lets_Encrypt:Let's Encrypt` },
    {
      value: 'https://acme-staging-v02.api.letsencrypt.org/directory',
      label: $localize`:@@Lets_Encrypt_staging:Let's Encrypt (staging)`,
    },
    { value: 'custom', label: $localize`:@@Another_authority:Another authority` },
  ];
  protected readonly directoryChoice = signal('');

  protected readonly state = computed(() => this.tls()?.state);
  protected readonly problems = computed(() => this.tls()?.state.problems ?? []);
  protected readonly issued = computed(() => Object.entries(this.tls()?.issued ?? {}));

  protected readonly consoleCoverage = computed(() =>
    (this.tls()?.coverage ?? []).filter((c) => c.plane === 'console'),
  );
  protected readonly appCoverage = computed(() =>
    (this.tls()?.coverage ?? []).filter((c) => c.plane === 'app'),
  );
  // One row per declared name, both planes, for the authority's tick boxes.
  protected readonly allCoverage = computed(() => this.tls()?.coverage ?? []);

  // Whether THIS page is being read over plain HTTP. The control plane never
  // redirects - that is what keeps it reachable when a certificate breaks - so
  // the console offers the move rather than the server imposing it.
  protected readonly overPlainHTTP = signal(
    typeof location !== 'undefined' && location.protocol === 'http:',
  );
  protected readonly offerHTTPS = computed(() => !!this.state()?.admin && this.overPlainHTTP());
  protected readonly warnOnMove = computed(() =>
    this.certificates().some((c) => !c.pending && c.info.selfSigned),
  );

  constructor() {
    this.load();
  }

  private load(): void {
    this.loading.set(true);
    this.api.listCertificates().subscribe({
      next: (list) => this.certificates.set(list),
      error: () => this.certificates.set([]),
    });
    this.api.getTls().subscribe({
      next: (t) => {
        this.apply(t);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  private apply(t: TlsSettings): void {
    this.tls.set(t);
    this.acme.set({ ...t.acme });
    this.eabSecretSet.set(t.eabSecretSet);
    this.consoleNames.set((t.consoleNames ?? []).join('\n'));
    this.appNames.set((t.appNames ?? []).join('\n'));
    const known = this.directories.some((d) => d.value === t.acme.directoryUrl);
    this.directoryChoice.set(known ? t.acme.directoryUrl : 'custom');
  }

  // ── the two doors ──────────────────────────────────────────────────────────

  protected toggle(plane: 'data' | 'admin', on: boolean): void {
    const t = this.tls();
    if (!t) return;
    this.save({ data: plane === 'data' ? on : t.data, admin: plane === 'admin' ? on : t.admin });
  }

  protected toggleRedirect(on: boolean): void {
    this.save({ redirect: on });
  }

  // A name is automatic when it is in the authority's closed list. The tick
  // box IS that membership - there is no third list to keep in step.
  protected toggleAutomatic(name: string, on: boolean): void {
    const domains = new Set(this.acme().domains ?? []);
    if (on) {
      domains.add(name);
    } else {
      domains.delete(name);
    }
    this.acme.set({ ...this.acme(), domains: [...domains] });
    this.save({});
  }

  protected saveNames(): void {
    this.save({});
  }

  protected saveAcme(): void {
    this.save({});
  }

  private save(patch: { data?: boolean; admin?: boolean; redirect?: boolean }): void {
    const t = this.tls();
    if (!t) return;
    const acme = { ...this.acme() };
    if (this.directoryChoice() !== 'custom') acme.directoryUrl = this.directoryChoice();
    this.saving.set(true);
    this.api
      .saveTls({
        data: patch.data ?? t.data,
        admin: patch.admin ?? t.admin,
        redirect: patch.redirect ?? t.redirect,
        consoleNames: lines(this.consoleNames()),
        appNames: lines(this.appNames()),
        acme,
      })
      .subscribe({
        next: (saved) => {
          this.apply(saved);
          this.saving.set(false);
          if (saved.state.problems.length === 0) {
            this.snack.open($localize`:@@TLS_saved:TLS settings saved`, undefined, {
              duration: 2500,
            });
          }
        },
        error: (e) => {
          this.saving.set(false);
          this.snack.open(message(e), undefined, { duration: 6000 });
        },
      });
  }

  // ── the certificates ───────────────────────────────────────────────────────

  // The add menu is cloned per plane so the generation form arrives already
  // filled with the names that plane must answer to: the operator declared
  // them once, and typing them a second time is where the two drift apart.
  protected async add(door: CertificateDoor, plane: 'console' | 'app'): Promise<void> {
    const uncovered = (plane === 'console' ? this.consoleCoverage() : this.appCoverage())
      .filter((c) => c.status !== 'covered')
      .map((c) => c.name);
    const declared = plane === 'console' ? lines(this.consoleNames()) : lines(this.appNames());
    const res = await firstValueFrom(
      this.dialog
        .open<CertificateDialogComponent, CertificateDialogData, CertificateDialogResult | undefined>(
          CertificateDialogComponent,
          { data: { door, hosts: (uncovered.length ? uncovered : declared).join(' ') }, restoreFocus: true },
        )
        .afterClosed(),
    );
    if (!res) return;
    const call =
      res.door === 'import'
        ? this.api.importCertificate({
            name: res.name,
            certPem: res.certPem,
            keyPem: res.keyPem,
            keystore: res.keystore,
            password: res.password,
          })
        : res.door === 'self-signed'
          ? this.api.createSelfSigned(res.request!)
          : this.api.createSigningRequest(res.request!);
    call.subscribe({
      next: () => this.load(),
      error: (e) => this.snack.open(message(e), undefined, { duration: 8000 }),
    });
  }

  protected async adopt(c: Certificate): Promise<void> {
    const pem = await firstValueFrom(
      this.dialog
        .open<AdoptDialogComponent, Certificate, string | undefined>(AdoptDialogComponent, {
          data: c,
          restoreFocus: true,
        })
        .afterClosed(),
    );
    if (!pem) return;
    this.api.adoptCertificate(c.id, pem).subscribe({
      next: () => {
        this.snack.open($localize`:@@Certificate_adopted:Certificate adopted`, undefined, {
          duration: 3000,
        });
        this.load();
      },
      error: (e) => this.snack.open(message(e), undefined, { duration: 8000 }),
    });
  }

  protected setDefault(c: Certificate): void {
    this.api.updateCertificate(c.id, { name: c.name, default: !c.default }).subscribe({
      next: () => this.load(),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  protected async rename(c: Certificate): Promise<void> {
    const name = await this.dialogs.prompt({
      title: $localize`:@@Rename_certificate:Rename certificate`,
      label: $localize`:@@Name:Name`,
      confirmLabel: $localize`:@@Rename:Rename`,
      initial: c.name,
    });
    if (!name || name === c.name) return;
    this.api.updateCertificate(c.id, { name, default: c.default }).subscribe({
      next: () => this.load(),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  protected async remove(c: Certificate): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_certificate:Delete certificate`,
      message: $localize`:@@Delete_certificate_message:Delete "${c.name}"? Its private key is destroyed with it, and any host it was the only certificate for stops answering.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteCertificate(c.id).subscribe({
      next: () => this.load(),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  protected download(c: Certificate): void {
    const call = c.pending
      ? this.api.downloadSigningRequest(c.id)
      : this.api.downloadCertificate(c.id);
    call.subscribe({
      next: (text) => saveFile(c.name + (c.pending ? '.csr' : '.pem'), text),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  // ── addresses ──────────────────────────────────────────────────────────────

  // The host to build a link on. For the console it is the one the operator is
  // already using; for the application it is the first name they declared,
  // because the gateway is rarely reached by the console's own hostname.
  private hostFor(plane: 'console' | 'app'): string {
    if (plane === 'app') {
      const first = lines(this.appNames())[0];
      if (first && !first.startsWith('*')) return first;
    }
    return location.hostname;
  }

  protected link(plane: 'console' | 'app', scheme: 'http' | 'https'): string {
    const st = this.state();
    if (!st) return '';
    const addr =
      plane === 'console'
        ? scheme === 'http'
          ? st.adminPlainAddr
          : st.adminAddr
        : scheme === 'http'
          ? st.dataPlainAddr
          : st.dataAddr;
    const p = this.port(addr);
    const implied = scheme === 'http' ? '80' : '443';
    const host = this.hostFor(plane);
    return `${scheme}://${p && p !== implied ? host + ':' + p : host}`;
  }

  // The address is ":9443" or "0.0.0.0:9443"; what an operator types is the
  // port, so that is what gets used.
  protected port(addr: string | undefined): string {
    const i = (addr ?? '').lastIndexOf(':');
    return i < 0 ? '' : (addr ?? '').slice(i + 1);
  }

  protected openOverHTTPS(): void {
    location.assign(this.link('console', 'https') + location.pathname);
  }

  // ── labels ─────────────────────────────────────────────────────────────────

  protected hosts(c: Certificate): string[] {
    const info = c.pending && c.csr ? c.csr : c.info;
    return [...(info.dnsNames ?? []), ...(info.ipAddresses ?? [])];
  }

  protected day(ts: number): string {
    return DateTime.fromSeconds(ts).setLocale(this.locale).toLocaleString(DateTime.DATE_MED);
  }

  // What a coverage row says, in one line. The wording answers "will this
  // work", never "here is a status code".
  protected coverText(c: Coverage): string {
    switch (c.status) {
      case 'covered':
        return $localize`:@@Covered_by:Covered by "${c.certName}" until ${this.day(c.notAfter!)}`;
      case 'expiring':
        return $localize`:@@Expires_soon_on:"${c.certName}" expires on ${this.day(c.notAfter!)}`;
      case 'expired':
        return $localize`:@@Its_certificate_expired:"${c.certName}" expired on ${this.day(c.notAfter!)}`;
      case 'acme':
        return $localize`:@@Waiting_for_the_authority:No certificate yet - the authority will fetch one on the first visit`;
      default:
        return $localize`:@@Nothing_answers_for_this_name:Nothing answers for this name`;
    }
  }

  protected coverIcon(c: Coverage): string {
    switch (c.status) {
      case 'covered':
        return 'check_circle';
      case 'expiring':
        return 'schedule';
      case 'acme':
        return 'autorenew';
      default:
        return 'error';
    }
  }

  protected readonly selfSignedWarning = $localize`:@@Self_signed_tooltip:Nobody vouches for this one but itself: browsers will warn.`;

  protected sourceLabel(c: Certificate): string {
    switch (c.source) {
      case 'self-signed':
        return $localize`:@@Self_signed:Self-signed`;
      case 'csr':
        return $localize`:@@Signed_on_request:Signed on request`;
      default:
        return $localize`:@@Imported:Imported`;
    }
  }

  protected validity(c: Certificate): { text: string; level: 'ok' | 'soon' | 'gone' } {
    if (c.pending) return { text: $localize`:@@Awaiting_signature:Awaiting signature`, level: 'soon' };
    const days = Math.floor(DateTime.fromSeconds(c.info.notAfter).diffNow('days').days);
    if (days < 0) return { text: $localize`:@@Expired:Expired`, level: 'gone' };
    if (days <= 30) {
      return { text: $localize`:@@Expires_in_N_days:Expires in ${days} days`, level: 'soon' };
    }
    return {
      text: $localize`:@@Valid_until_DATE:Valid until ${this.day(c.info.notAfter)}`,
      level: 'ok',
    };
  }
}

// The answer of an authority, pasted back. A dedicated dialog rather than the
// shared prompt: a certificate is twenty lines of base64, and a single-line box
// would make checking what was pasted impossible.
@Component({
  selector: 'app-adopt-dialog',
  imports: [MatButtonModule, MatDialogModule, MatFormFieldModule, MatInputModule, FormFieldComponent],
  template: `
    <h2 mat-dialog-title i18n="@@Adopt_the_signed_certificate">Adopt the signed certificate</h2>
    <mat-dialog-content class="adopt">
      <p i18n="@@Adopt_note">
        Paste what the authority signed from this request. Meerkat checks it against the private
        key that never left: a certificate from another order is refused here rather than at the
        first handshake.
      </p>
      <app-form-field i18n-label="@@Certificate_PEM" label="Certificate (PEM)">
        <textarea
          matInput
          rows="10"
          [value]="pem()"
          (input)="pem.set($any($event.target).value)"
          placeholder="-----BEGIN CERTIFICATE-----"
        ></textarea>
      </app-form-field>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!pem().trim()" [mat-dialog-close]="pem()" i18n="@@Adopt">
        Adopt
      </button>
    </mat-dialog-actions>
  `,
  styles: `
    .adopt {
      min-width: 560px;
    }
    .adopt p {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
      margin: 0 0 12px;
    }
    .adopt textarea {
      font-family: var(--mk-mono);
      font-size: 0.75rem;
    }
  `,
})
export class AdoptDialogComponent {
  protected readonly data = inject<Certificate>(MAT_DIALOG_DATA);
  protected readonly ref = inject(MatDialogRef<AdoptDialogComponent, string>);
  protected readonly pem = signal('');
}

function blankAcme(): AcmeSettings {
  return {
    enabled: false,
    directoryUrl: '',
    email: '',
    domains: [],
    rootCa: '',
    eabKeyId: '',
    eabHmacKey: '',
    acceptTos: false,
  };
}

function lines(text: string): string[] {
  return text
    .split(/[\s,;]+/)
    .map((v) => v.trim())
    .filter(Boolean);
}

// The server's sentence, not a generic one: every refusal on this screen names
// what failed and what is allowed, and replacing that with "an error occurred"
// would throw away the only useful part.
function message(e: unknown): string {
  const err = e as { error?: { error?: string }; message?: string };
  return err?.error?.error ?? err?.message ?? $localize`:@@Something_went_wrong:Something went wrong`;
}

function saveFile(name: string, text: string): void {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/x-pem-file' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}
