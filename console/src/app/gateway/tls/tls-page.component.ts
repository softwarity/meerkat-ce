import { NgTemplateOutlet } from '@angular/common';
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
import { AcmeSettings, ApiService, Certificate, TlsSettings } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { SecretFieldComponent } from '../../shared/secret-field.component';
import {
  CertificateDialogComponent,
  CertificateDialogData,
  CertificateDialogResult,
  CertificateDoor,
} from './certificate-dialog.component';

// One name, one certificate. That is the whole model.
//
// The console answers to one name and has one certificate. The application
// answers to as many hosts as it serves and has one per host. Material used by
// both is added twice - two entries and two keys are cheaper to understand
// than one shared pile plus the rule that works out which name each piece
// covers.
//
// There is no "switch HTTPS on": having a certificate is what opens the door,
// and deleting it is what closes it. A switch that can be on with nothing
// behind it is a switch that lies.
export interface Entry {
  host: string;
  plane: 'console' | 'app';
  cert?: Certificate;
  automatic: boolean;
}

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
    NgTemplateOutlet,
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

  protected readonly consoleName = signal('');
  protected readonly appNames = signal<string[]>([]);
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
  // Whether the authority is somebody's own rather than a public one: an
  // internal step-ca reaches internal names perfectly well, so the warning
  // below only applies to the public case.
  protected readonly customAuthority = computed(() => this.directoryChoice() === 'custom');
  protected readonly problems = computed(() => this.tls()?.state.problems ?? []);
  protected readonly issued = computed(() => Object.entries(this.tls()?.issued ?? {}));

  private readonly automatic = computed(() => new Set(this.acme().domains ?? []));

  protected readonly consoleEntry = computed<Entry>(() => {
    const host = this.consoleName();
    return {
      host,
      plane: 'console',
      cert: this.certificates().find((c) => c.plane === 'console'),
      automatic: this.automatic().has(host),
    };
  });

  // Every declared name, plus any host that has a certificate but slipped out
  // of the list: losing a row would lose the only way to delete its material.
  protected readonly appEntries = computed<Entry[]>(() => {
    const certs = this.certificates().filter((c) => c.plane === 'app');
    const hosts = [...this.appNames()];
    for (const c of certs) {
      if (!hosts.includes(c.host)) hosts.push(c.host);
    }
    return hosts.map((host) => ({
      host,
      plane: 'app' as const,
      cert: certs.find((c) => c.host === host),
      automatic: this.automatic().has(host),
    }));
  });

  protected readonly allEntries = computed(() => [this.consoleEntry(), ...this.appEntries()]);

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
    this.consoleName.set(t.consoleName);
    this.appNames.set([...(t.appNames ?? [])]);
    const known = this.directories.some((d) => d.value === t.acme.directoryUrl);
    this.directoryChoice.set(known ? t.acme.directoryUrl : 'custom');
  }

  // ── names ──────────────────────────────────────────────────────────────────

  protected renameConsole(value: string): void {
    this.consoleName.set(value.trim().toLowerCase());
  }

  protected async addName(): Promise<void> {
    const host = await this.dialogs.prompt({
      title: $localize`:@@Add_a_name:Add a name`,
      label: $localize`:@@Host:Host`,
      confirmLabel: $localize`:@@Add:Add`,
    });
    if (!host) return;
    const clean = host.trim().toLowerCase();
    if (this.appNames().includes(clean)) return;
    this.appNames.set([...this.appNames(), clean]);
    this.save();
  }

  protected async removeName(e: Entry): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Remove_the_name:Remove the name`,
      message: e.cert
        ? $localize`:@@Remove_name_with_cert:Remove ${e.host}? Its certificate is deleted with it, and the host stops answering over HTTPS.`
        : $localize`:@@Remove_name_plain:Remove ${e.host}?`,
      confirmLabel: $localize`:@@Remove:Remove`,
      danger: true,
    });
    if (!ok) return;
    this.appNames.set(this.appNames().filter((h) => h !== e.host));
    if (e.cert) {
      this.api.deleteCertificate(e.cert.id).subscribe({
        next: () => this.save(),
        error: (err) => this.snack.open(message(err), undefined, { duration: 6000 }),
      });
      return;
    }
    this.save();
  }

  protected toggleRedirect(on: boolean): void {
    this.save({ redirect: on });
  }

  // A name is automatic when it is in the authority's closed list. The tick
  // box IS that membership - there is no separate list to keep in step.
  protected toggleAutomatic(host: string, on: boolean): void {
    const domains = new Set(this.acme().domains ?? []);
    if (on) {
      domains.add(host);
    } else {
      domains.delete(host);
    }
    this.acme.set({ ...this.acme(), domains: [...domains] });
    this.save();
  }

  protected save(patch: { redirect?: boolean } = {}): void {
    const t = this.tls();
    if (!t) return;
    const acme = { ...this.acme() };
    if (this.directoryChoice() !== 'custom') acme.directoryUrl = this.directoryChoice();
    this.saving.set(true);
    this.api
      .saveTls({
        consoleName: this.consoleName(),
        appNames: this.appNames(),
        redirect: patch.redirect ?? t.redirect,
        acme,
      })
      .subscribe({
        next: (saved) => {
          this.apply(saved);
          this.saving.set(false);
        },
        error: (e) => {
          this.saving.set(false);
          this.snack.open(message(e), undefined, { duration: 6000 });
        },
      });
  }

  // ── certificates ───────────────────────────────────────────────────────────

  // The form knows the host already: it was declared once on this screen, and
  // typing it a second time is exactly where the two drift apart.
  protected add(door: CertificateDoor, e: Entry): Promise<void> {
    return this.replace(door, e);
  }

  private create(res: CertificateDialogResult, e: Entry) {
    const base = { plane: e.plane, host: e.host };
    return res.door === 'import'
      ? this.api.importCertificate({
          ...base,
          certPem: res.certPem,
          keyPem: res.keyPem,
          keystore: res.keystore,
          password: res.password,
        })
      : res.door === 'self-signed'
        ? this.api.createSelfSigned({ ...base, ...res.request })
        : this.api.createSigningRequest({ ...base, ...res.request });
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

  // Replacing, not deleting. Nobody wants a host left without a certificate:
  // what they want is a different one - the old is expiring, or the wrong file
  // went in. So the new one is created FIRST and the old dropped only once it
  // is in, which means a failed import leaves the host still served.
  protected async replace(door: CertificateDoor, e: Entry): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<CertificateDialogComponent, CertificateDialogData, CertificateDialogResult | undefined>(
          CertificateDialogComponent,
          { data: { door, host: e.host, plane: e.plane }, restoreFocus: true },
        )
        .afterClosed(),
    );
    if (!res) return;
    const old = e.cert;
    this.create(res, e).subscribe({
      next: () => {
        if (!old) {
          this.load();
          return;
        }
        this.api.deleteCertificate(old.id).subscribe({
          next: () => this.load(),
          error: (err) => this.snack.open(message(err), undefined, { duration: 6000 }),
        });
      },
      error: (err) => this.snack.open(message(err), undefined, { duration: 8000 }),
    });
  }

  // Removing the certificate and leaving the name: the host stops being served
  // over HTTPS, and the entry goes back to offering the three doors.
  protected async dropCertificate(e: Entry): Promise<void> {
    if (!e.cert) return;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Remove_the_certificate:Remove the certificate`,
      message: $localize`:@@Remove_certificate_message:${e.host} stops answering over HTTPS, and the private key is destroyed. The name stays.`,
      confirmLabel: $localize`:@@Remove:Remove`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteCertificate(e.cert.id).subscribe({
      next: () => this.load(),
      error: (err) => this.snack.open(message(err), undefined, { duration: 6000 }),
    });
  }

  protected download(c: Certificate): void {
    const call = c.pending
      ? this.api.downloadSigningRequest(c.id)
      : this.api.downloadCertificate(c.id);
    call.subscribe({
      next: (text) => saveFile(c.host + (c.pending ? '.csr' : '.pem'), text),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  // ── addresses ──────────────────────────────────────────────────────────────

  // The two addresses OF ONE NAME. They belong to the entry the way the
  // certificate does: an operator reading a host wants to click it, not to
  // work out which of several names the section's single link meant.
  protected link(e: Entry, scheme: 'http' | 'https'): string {
    const st = this.state();
    if (!st) return '';
    const addr =
      e.plane === 'console'
        ? scheme === 'http'
          ? st.consolePlainAddr
          : st.consoleAddr
        : scheme === 'http'
          ? st.appPlainAddr
          : st.appAddr;
    const p = this.port(addr);
    const implied = scheme === 'http' ? '80' : '443';
    return `${scheme}://${p && p !== implied ? e.host + ':' + p : e.host}`;
  }

  // A wildcard stands for names rather than being one, so there is nothing to
  // click: writing *.apps.example.com into an address bar reaches nothing.
  protected linkable(e: Entry): boolean {
    return !!e.host && !e.host.startsWith('*');
  }

  // Whether this entry's plane currently serves HTTPS.
  protected secure(e: Entry): boolean {
    return e.plane === 'console' ? !!this.state()?.console : !!this.state()?.app;
  }

  protected port(addr: string | undefined): string {
    const i = (addr ?? '').lastIndexOf(':');
    return i < 0 ? '' : (addr ?? '').slice(i + 1);
  }

  // ── labels ─────────────────────────────────────────────────────────────────

  // A name that exists only on this machine, or only inside a private network:
  // no label at all, or one of the suffixes reserved for local use. A public
  // authority has to connect to the name to prove control, so it can never
  // issue for one of these.
  protected privateName(host: string): boolean {
    const h = host.replace(/^\*\./, '');
    return !h.includes('.') || /\.(local|internal|test|localhost|home\.arpa)$/.test(h);
  }

  protected day(ts: number): string {
    return DateTime.fromSeconds(ts).setLocale(this.locale).toLocaleString(DateTime.DATE_MED);
  }

  // What one entry says about itself, in a line. The wording answers "will
  // this work", never "here is a status code".
  protected says(e: Entry): string {
    if (!e.cert) {
      return e.automatic
        ? $localize`:@@Waiting_for_the_authority:No certificate yet - the authority will fetch one on the first visit`
        : $localize`:@@No_certificate_here:No certificate: this name is not served over HTTPS`;
    }
    if (e.cert.pending) {
      return $localize`:@@Awaiting_signature:Awaiting signature`;
    }
    if (e.cert.mismatch) {
      return $localize`:@@Does_not_cover:This certificate does not answer for ${e.host}`;
    }
    const days = Math.floor(DateTime.fromSeconds(e.cert.info.notAfter).diffNow('days').days);
    if (days < 0) return $localize`:@@Expired_on:Expired on ${this.day(e.cert.info.notAfter)}`;
    if (days <= 30) {
      return $localize`:@@Expires_in_N_days:Expires in ${days} days`;
    }
    return $localize`:@@Valid_until_DATE:Valid until ${this.day(e.cert.info.notAfter)}`;
  }

  protected level(e: Entry): string {
    if (!e.cert) return e.automatic ? 'wait' : 'none';
    if (e.cert.pending) return 'wait';
    if (e.cert.mismatch) return 'bad';
    const days = Math.floor(DateTime.fromSeconds(e.cert.info.notAfter).diffNow('days').days);
    if (days < 0) return 'bad';
    return days <= 30 ? 'soon' : 'ok';
  }

  protected icon(e: Entry): string {
    switch (this.level(e)) {
      case 'ok':
        return 'lock';
      case 'soon':
        return 'schedule';
      case 'wait':
        return 'autorenew';
      case 'bad':
        return 'error';
      default:
        return 'lock_open_right';
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
