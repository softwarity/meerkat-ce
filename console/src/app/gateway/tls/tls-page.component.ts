import { CdkDrag, CdkDragDrop, CdkDragPlaceholder, CdkDropList, CdkDropListGroup } from '@angular/cdk/drag-drop';
import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, DestroyRef, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MAT_DIALOG_DATA, MatDialog, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { firstValueFrom, Observable } from 'rxjs';
import {
  AcmeAuthority,
  ApiService,
  CertConflict,
  Certificate,
  CertPlacement,
  TlsSettings,
} from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { LiveChangesService } from '../../shared/live-changes.service';
import { SnippetComponent } from '../../shared/snippet.component';
import { AuthoritiesPanelComponent } from './authorities-panel.component';
import { MeService } from '../../me.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import {
  CertificateDialogComponent,
  CertificateDialogData,
  CertificateDialogResult,
  CertificateDoor,
} from './certificate-dialog.component';

// A pool, then a placement.
//
// A certificate is made or imported ONCE, into the pool: its names are its
// own, read from the material, and a wildcard or a certificate carrying
// several names serves every one of them. It is then placed on the console,
// the application, or both - by dragging it there, or from its menu. A
// certificate placed on a plane is what opens that plane's HTTPS door, and
// taking it off is what closes it: there is no switch to disagree with.
export type Plane = 'console' | 'app';

type Os = 'macos' | 'linux' | 'windows';

@Component({
  selector: 'app-tls-page',
  imports: [
    CdkDrag,
    CdkDragPlaceholder,
    CdkDropList,
    CdkDropListGroup,
    MatButtonModule,
    MatButtonToggleModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatMenuModule,
    MatSelectModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    FormFieldComponent,
    SnippetComponent,
    AuthoritiesPanelComponent,
    EeLockComponent,
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

  // ACME is Enterprise (SSL-05): on the community image the button is locked
  // and no Ask entry is offered.
  protected readonly enterprise = inject(MeService).enterprise;
  protected readonly acmeTip = $localize`:@@Acme_tooltip:ACME is how an authority - Let's Encrypt, ZeroSSL, your own - issues a certificate once it has checked that the name reaches this gateway, and renews it on its own. Set the authorities up here: each one becomes an Ask entry of Add certificate.`;

  // The ACME authorities set up: one Ask entry each in Add certificate.
  protected readonly authorities = signal<AcmeAuthority[]>([]);
  // Which providers do not issue for a name only this network knows.
  private readonly publicProviders = new Set(['letsencrypt', 'letsencrypt-staging', 'zerossl', 'google']);

  protected readonly state = computed(() => this.tls()?.state);
  protected readonly problems = computed(() => this.tls()?.state.problems ?? []);

  protected readonly planes: Plane[] = ['console', 'app'];
  protected readonly onConsole = computed(() => this.certificates().filter((c) => c.console));
  protected readonly onApp = computed(() => this.certificates().filter((c) => c.app));

  // The name this console is reached by: what a first certificate is most
  // often for, and what the commands of the help drawer start with.
  protected readonly here = location.hostname;

  constructor() {
    this.load();
    this.loadAuthorities();
    inject(DestroyRef).onDestroy(() => clearTimeout(this.poll));
    // Somebody else's write (CONSOLE-13), and only the CERTIFICATES: the rest
    // of this screen is a form, and re-applying the settings under somebody
    // typing in it would throw their work away for news they did not ask for.
    inject(LiveChangesService).on('certificate', () => this.loadCertificates());
  }

  // While an authority is being asked, the pool looks again every few
  // seconds: the answer comes in the background, and a screen that waited
  // for somebody to reload would show "Asking" long after it was settled.
  private poll?: ReturnType<typeof setTimeout>;

  private loadCertificates(): void {
    this.api.listCertificates().subscribe({
      next: (list) => {
        this.certificates.set(list);
        clearTimeout(this.poll);
        if (list.some((c) => c.status === 'requesting')) {
          this.poll = setTimeout(() => this.loadCertificates(), 3000);
        }
      },
      error: () => this.certificates.set([]),
    });
  }

  private loadAuthorities(): void {
    this.api.listAcmeAuthorities().subscribe({
      next: (r) => this.authorities.set(r.authorities),
      error: () => this.authorities.set([]),
    });
  }

  private load(): void {
    this.loading.set(true);
    this.loadCertificates();
    this.api.getTls().subscribe({
      next: (t) => {
        this.apply(t);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  // The door state follows the certificates: re-read after a placement, so
  // the links and Force HTTPS say what is open now.
  private reloadState(): void {
    this.api.getTls().subscribe({ next: (t) => this.tls.set(t) });
  }

  private apply(t: TlsSettings): void {
    this.tls.set(t);
  }

  // ── settings ───────────────────────────────────────────────────────────────

  protected toggleRedirect(on: boolean): void {
    this.save({ redirect: on });
  }

  // HSTS (SSL-06) follows Force HTTPS; only its length is chosen, a DAY until
  // somebody says otherwise: a browser keeps the promise for the whole
  // duration even if the certificates are taken away, so the first setting is
  // the one that is cheap to be wrong about.
  protected readonly hstsSeconds = computed(() => this.tls()?.hstsMaxAge || 86400);
  protected readonly hstsDurations = [
    { seconds: 86400, label: $localize`:@@Hsts_day:1 day` },
    { seconds: 7 * 86400, label: $localize`:@@Hsts_week:1 week` },
    { seconds: 30 * 86400, label: $localize`:@@Hsts_month:1 month` },
    { seconds: 182 * 86400, label: $localize`:@@Hsts_six_months:6 months` },
    { seconds: 365 * 86400, label: $localize`:@@Hsts_year:1 year` },
    { seconds: 730 * 86400, label: $localize`:@@Hsts_two_years:2 years` },
  ];

  protected setHsts(seconds: number): void {
    this.save({ hstsMaxAge: seconds });
  }

  protected save(patch: { redirect?: boolean; hstsMaxAge?: number } = {}): void {
    const t = this.tls();
    if (!t) return;
    this.saving.set(true);
    this.api
      .saveTls({
        redirect: patch.redirect ?? t.redirect,
        hstsMaxAge: patch.hstsMaxAge ?? t.hstsMaxAge ?? 0,
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

  // ── the pool ───────────────────────────────────────────────────────────────

  protected async add(door: CertificateDoor, authority?: AcmeAuthority): Promise<void> {
    const res = await firstValueFrom(
      this.dialog
        .open<CertificateDialogComponent, CertificateDialogData, CertificateDialogResult | undefined>(
          CertificateDialogComponent,
          {
            data: {
              door,
              names: this.here,
              authorityName: authority?.name,
              publicAuthority: !!authority && this.publicProviders.has(authority.provider),
            },
            restoreFocus: true,
          },
        )
        .afterClosed(),
    );
    if (!res) return;
    this.create(res, authority).subscribe({
      next: () => this.loadCertificates(),
      error: (err) => this.snack.open(message(err), undefined, { duration: 8000 }),
    });
  }

  private create(res: CertificateDialogResult, authority?: AcmeAuthority): Observable<Certificate> {
    switch (res.door) {
      case 'pem':
        return this.api.importCertificate({ certPem: res.certPem, keyPem: res.keyPem });
      case 'keystore':
        return this.api.importCertificate({ keystore: res.keystore, password: res.password });
      case 'self-signed':
        return this.api.createSelfSigned(res.request!);
      case 'acme':
        return this.api.createAcmeOrder({ authority: authority!.id, names: res.request!.names });
      default:
        return this.api.createSigningRequest(res.request!);
    }
  }

  protected async adopt(c: Certificate): Promise<void> {
    const pem = await firstValueFrom(
      this.dialog
        .open<AdoptDialogComponent, Certificate, string | undefined>(AdoptDialogComponent, {
          data: c,
          restoreFocus: true,
          // Wider than Material's 560px: a PEM line is 64 characters.
          width: '680px',
          maxWidth: '92vw',
        })
        .afterClosed(),
    );
    if (!pem) return;
    this.api.adoptCertificate(c.id, pem).subscribe({
      next: () => {
        this.snack.open($localize`:@@Certificate_adopted:Certificate adopted`, undefined, {
          duration: 3000,
        });
        this.loadCertificates();
      },
      error: (e) => this.snack.open(message(e), undefined, { duration: 8000 }),
    });
  }

  // Deleting destroys the key. A certificate still served says which door
  // goes back to the clear, because that is the consequence nobody reads in
  // "delete".
  protected async remove(c: Certificate): Promise<void> {
    const name = this.nameOf(c);
    const doors = [c.console ? this.planeLabel('console') : '', c.app ? this.planeLabel('app') : '']
      .filter(Boolean)
      .join(', ');
    const away = this.cutsThisPage(c, false);
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_the_certificate:Delete the certificate`,
      message: away
        ? this.cutMessage(name) +
          ' ' +
          $localize`:@@Delete_destroys_key:Its private key is destroyed.`
        : doors
          ? $localize`:@@Delete_served_certificate:${name}:name: is served on ${doors}:doors:. Deleting it destroys its private key, and what it answered for goes back to plain HTTP.`
          : $localize`:@@Delete_spare_certificate:Delete ${name}:name:? Its private key is destroyed.`,
      confirmLabel: away ? this.cutLabel : $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteCertificate(c.id).subscribe({
      next: () => {
        if (away) {
          location.href = away;
          return;
        }
        this.loadCertificates();
        this.reloadState();
      },
      error: (err: HttpErrorResponse) => {
        if (away && err.status === 0) {
          location.href = away;
          return;
        }
        this.snack.open(message(err), undefined, { duration: 6000 });
      },
    });
  }

  // ── the door this page came through ────────────────────────────────────────
  //
  // This page may be reached over the console's HTTPS door, through the very
  // certificate being taken away. Without one carrying the name in the address
  // bar the door closes - or answers with a certificate the browser refuses -
  // and the page stops answering mid-click. So such an action is confirmed
  // first, and once done the browser is taken to the console's plain door,
  // which never moves.

  // The plain address to go to when c leaves the console (stays: whether it
  // stays there, for a move), or '' when this page is not cut off by it.
  private cutsThisPage(c: Certificate, stays: boolean): string {
    if (location.protocol !== 'https:' || !c.console || stays) return '';
    const left = this.onConsole().filter((o) => o.id !== c.id && !o.pending);
    if (left.some((o) => covers(this.namesOf(o), this.here))) return '';
    const port = this.outsidePort('console', 'http');
    const host = this.here.includes(':') ? `[${this.here}]` : this.here;
    return `http://${host}${port && port !== '80' ? ':' + port : ''}${location.pathname}`;
  }

  private cutMessage(name: string): string {
    return $localize`:@@Cuts_this_page:You are on the console over HTTPS, through ${name}:name:: without it, this address stops answering. The console stays open over plain HTTP: this page goes there, where you sign in again.`;
  }

  protected readonly cutLabel = $localize`:@@Continue_over_http:Continue over HTTP`;

  // Ask again, once whatever the authority refused for is fixed.
  protected retry(c: Certificate): void {
    this.api.retryAcmeOrder(c.id).subscribe({
      next: () => this.loadCertificates(),
      error: (err) => this.snack.open(message(err), undefined, { duration: 8000 }),
    });
  }

  protected download(c: Certificate): void {
    const call = c.pending
      ? this.api.downloadSigningRequest(c.id)
      : this.api.downloadCertificate(c.id);
    const file = this.nameOf(c).replace('*', 'wildcard');
    call.subscribe({
      next: (text) => saveFile(file + (c.pending ? '.csr' : '.pem'), text),
      error: (e) => this.snack.open(message(e), undefined, { duration: 6000 }),
    });
  }

  // ── placement ──────────────────────────────────────────────────────────────

  // Dropped somewhere. Into a plane: served there - and taken off the plane it
  // was dragged from, since a drag is a move. Back into the pool: taken off
  // the plane it came from.
  protected dropped<T extends string>(e: CdkDragDrop<T>): void {
    const c = e.item.data as Certificate;
    const from = e.previousContainer.data as Plane | 'pool';
    const to = e.container.data as Plane | 'pool';
    if (from === to) return;
    const next: CertPlacement = { console: c.console, app: c.app };
    if (from !== 'pool') next[from] = false;
    if (to !== 'pool') next[to] = true;
    void this.place(c, next);
  }

  // The menu's way to the same thing, for a keyboard or a touch screen.
  protected toggle(c: Certificate, plane: Plane): void {
    void this.place(c, { console: c.console, app: c.app, [plane]: !c[plane] });
  }

  private async place(c: Certificate, p: CertPlacement): Promise<void> {
    const away = this.cutsThisPage(c, !!p.console);
    if (away && !p.replace) {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Take_it_off_the_console:Take it off the console?`,
        message: this.cutMessage(this.nameOf(c)),
        confirmLabel: this.cutLabel,
        danger: true,
      });
      if (!ok) return;
    }
    this.api.placeCertificate(c.id, p).subscribe({
      next: () => {
        if (away) {
          location.href = away;
          return;
        }
        this.loadCertificates();
        this.reloadState();
      },
      error: async (err: HttpErrorResponse) => {
        // The door closed under its own answer: the connection this request
        // rode on is gone, which is the change having been made.
        if (away && err.status === 0) {
          location.href = away;
          return;
        }
        const conflicts = (err.error?.conflicts ?? []) as CertConflict[];
        if (!conflicts.length) {
          this.snack.open(message(err), undefined, { duration: 8000 });
          return;
        }
        // Somebody already answers for one of these names on that door: say
        // who, and replace only when told to.
        const who = conflicts
          .map((x) => {
            const other = this.certificates().find((o) => o.id === x.id);
            const label = other ? `${this.namesOf(other).join(', ')} (${this.sourceLabel(other).toLowerCase()}, ${this.says(other).toLowerCase()})` : x.id;
            return $localize`:@@Conflict_line:${this.planeLabel(x.plane)}:plane: already serves ${label}:other:, which answers for ${x.names.join(', ')}:names:`;
          })
          .join('; ');
        // What the replaced one answered for and this one does not: those
        // names lose HTTPS on that door, which "replace" does not say.
        const mine = new Set(this.namesOf(c));
        const lost = [
          ...new Set(
            conflicts.flatMap((x) =>
              this.namesOf(this.certificates().find((o) => o.id === x.id) ?? c).filter((n) => !mine.has(n)),
            ),
          ),
        ];
        const ok = await this.dialogs.confirm({
          title: $localize`:@@Replace_the_certificate:Replace the certificate?`,
          message:
            $localize`:@@Replace_conflict_message:${who}:who:. Replacing takes it off that door; it stays in the pool.` +
            (lost.length
              ? ' ' + $localize`:@@Replace_loses_names:${lost.join(', ')}:names: will no longer be served over HTTPS there.`
              : ''),
          confirmLabel: $localize`:@@Replace:Replace`,
        });
        if (ok) void this.place(c, { ...p, replace: true });
      },
    });
  }

  // ── addresses ──────────────────────────────────────────────────────────────

  // The names a plane is reached by over HTTPS: those of the certificates
  // placed there, minus the wildcards - writing *.example.com into an address
  // bar reaches nothing.
  protected namesOn(plane: Plane): string[] {
    const list = plane === 'console' ? this.onConsole() : this.onApp();
    const out: string[] = [];
    for (const c of list) {
      for (const n of [...(c.info.dnsNames ?? []), ...(c.info.ipAddresses ?? [])]) {
        if (!n.startsWith('*') && !out.includes(n)) out.push(n);
      }
    }
    return out;
  }

  // The port a plane's door is reached on from outside.
  protected outsidePort(plane: Plane, scheme: 'http' | 'https'): string {
    const st = this.state();
    if (!st) return '';
    const addr =
      plane === 'console'
        ? scheme === 'http'
          ? st.consolePlainAddr
          : st.consoleAddr
        : scheme === 'http'
          ? st.appPlainAddr
          : st.appAddr;
    return this.outside(this.port(addr));
  }

  protected readonly closedTip = $localize`:@@Https_closed_tip:Closed: no certificate is placed here.`;

  // The console is served over HTTPS, and not one of its certificates carries
  // the name it was reached by: the browser gets the fallback, refuses it, and
  // nothing on this screen would otherwise say why. The name is known - it is
  // the address bar's.
  protected readonly uncovered = computed(() => {
    const list = this.onConsole().filter((c) => !c.pending);
    if (!list.length) return false;
    return !list.some((c) => covers(this.namesOf(c), this.here));
  });

  protected link(plane: Plane, host: string, scheme: 'http' | 'https'): string {
    const st = this.state();
    if (!st) return '';
    const addr =
      plane === 'console'
        ? scheme === 'http'
          ? st.consolePlainAddr
          : st.consoleAddr
        : scheme === 'http'
          ? st.appPlainAddr
          : st.appAddr;
    const p = this.outside(this.port(addr));
    const implied = scheme === 'http' ? '80' : '443';
    const h = host.includes(':') ? `[${host}]` : host;
    return `${scheme}://${p && p !== implied ? h + ':' + p : h}`;
  }

  // The port the world reaches an inside one on, when the runtime says
  // (a Service publishing 9443 as 19443). Otherwise the inside one.
  private outside(inside: string): string {
    const pub = this.tls()?.published?.ports?.[inside];
    return pub ? String(pub) : inside;
  }

  // Said when the runtime was asked and does not publish this HTTPS door: the
  // link would reach nothing, and the fix is in the deployment, not here.
  protected unpublished(plane: Plane): string {
    const pub = this.tls()?.published;
    const st = this.state();
    if (!pub?.source || !st) return '';
    const inside = this.port(plane === 'console' ? st.consoleAddr : st.appAddr);
    return inside && !pub.ports?.[inside] ? inside : '';
  }

  // Whether this plane currently serves HTTPS.
  protected secure(plane: Plane): boolean {
    return plane === 'console' ? !!this.state()?.console : !!this.state()?.app;
  }

  protected port(addr: string | undefined): string {
    const i = (addr ?? '').lastIndexOf(':');
    return i < 0 ? '' : (addr ?? '').slice(i + 1);
  }

  // ── labels ─────────────────────────────────────────────────────────────────

  protected planeLabel(plane: Plane): string {
    return plane === 'console'
      ? $localize`:@@Console:Console`
      : $localize`:@@Application:Application`;
  }

  protected nameOf(c: Certificate): string {
    const all = [...(c.info.dnsNames ?? []), ...(c.info.ipAddresses ?? []), ...(c.csr?.dnsNames ?? [])];
    return all[0] ?? c.id;
  }

  protected namesOf(c: Certificate): string[] {
    if (c.pending && c.csr) return [...(c.csr.dnsNames ?? []), ...(c.csr.ipAddresses ?? [])];
    return [...(c.info.dnsNames ?? []), ...(c.info.ipAddresses ?? [])];
  }

  protected day(ts: number): string {
    return DateTime.fromSeconds(ts).setLocale(this.locale).toLocaleString(DateTime.DATE_MED);
  }

  // What one certificate says about itself, in a line. The wording answers
  // "will this work", never "here is a status code".
  protected says(c: Certificate): string {
    if (c.pending) return $localize`:@@Awaiting_signature:Awaiting signature`;
    if (c.source === 'acme' && !c.issued?.length) {
      if (c.status === 'failed') {
        return $localize`:@@Authority_refused:The authority refused: ${c.error}:error:`;
      }
      if (c.status === 'requesting') {
        return $localize`:@@Asking_the_authority:Asking the authority...`;
      }
      return c.console || c.app
        ? $localize`:@@Waiting_for_the_authority:Waiting for the authority`
        : $localize`:@@Asked_once_placed:Asked as soon as it is placed on a door`;
    }
    const days = Math.floor(DateTime.fromSeconds(c.info.notAfter).diffNow('days').days);
    if (days < 0) return $localize`:@@Expired_on:Expired on ${this.day(c.info.notAfter)}`;
    if (days <= 30) return $localize`:@@Expires_in_N_days:Expires in ${days} days`;
    return $localize`:@@Valid_until_DATE:Valid until ${this.day(c.info.notAfter)}`;
  }

  // The same, in the few words a door's row has room for.
  protected short(c: Certificate): string {
    if (c.source === 'acme' && !c.issued?.length) {
      if (c.status === 'failed') return $localize`:@@Refused_short:Refused - see the pool`;
      return $localize`:@@Waiting_for_the_authority_short:Waiting for the authority`;
    }
    return this.says(c);
  }

  protected level(c: Certificate): string {
    if (c.source === 'acme' && !c.issued?.length && c.status === 'failed') return 'bad';
    if (c.pending || (c.source === 'acme' && !c.issued?.length)) return 'wait';
    const days = Math.floor(DateTime.fromSeconds(c.info.notAfter).diffNow('days').days);
    if (days < 0) return 'bad';
    return days <= 30 ? 'soon' : 'ok';
  }

  protected icon(c: Certificate): string {
    switch (this.level(c)) {
      case 'ok':
        return 'lock';
      case 'soon':
        return 'schedule';
      case 'wait':
        return 'hourglass_top';
      default:
        return 'error';
    }
  }

  protected readonly selfSignedWarning = $localize`:@@Self_signed_tooltip:Browsers will warn about this certificate.`;

  protected sourceLabel(c: Certificate): string {
    switch (c.source) {
      case 'self-signed':
        return $localize`:@@Self_signed:Self-signed`;
      case 'csr':
        return $localize`:@@Signed_on_request:Signed on request`;
      case 'acme':
        return c.authorityName || $localize`:@@Automatic:Automatic`;
      default:
        return $localize`:@@Imported:Imported`;
    }
  }

  // ── the help drawer: HTTPS on a machine of one's own ───────────────────────
  //
  // A laptop or a lab has no public name and no public authority: the names
  // go in the hosts file, a local authority is made and trusted by this
  // machine's browsers (mkcert), it signs ONE certificate for every name, and
  // that is imported here and placed. Help rather than configuration, so in a
  // drawer.
  protected readonly helpOpen = signal(false);
  // What the drawer holds: the local-machine help, or the authorities.
  protected readonly drawer = signal<'local' | 'authorities'>('local');

  protected openDrawer(what: 'local' | 'authorities'): void {
    this.drawer.set(what);
    this.helpOpen.set(true);
  }
  protected readonly os = signal<Os>(guessOs());
  // The names the commands are written for, starting with the one this
  // console is reached by. Only changes the commands; nothing is saved.
  protected readonly localNames = signal(isAddress(location.hostname) ? 'console.local.test' : location.hostname);
  // Where the names should lead: the address this console was reached by when
  // it is one - the gateway is in a VM or on another machine - and this
  // machine otherwise.
  protected readonly address = signal(isAddress(location.hostname) ? location.hostname : '127.0.0.1');

  private readonly shown = computed(() => {
    const list = this.localNames()
      .split(/[\s,;]+/)
      .map((n) => n.trim().toLowerCase())
      .filter(Boolean);
    return list.length ? [...new Set(list)] : ['console.local.test'];
  });

  protected readonly hostsCmd = computed(() => {
    const names = this.shown().filter((n) => n !== 'localhost' && !isAddress(n) && !n.startsWith('*'));
    const ip = this.address().trim() || '127.0.0.1';
    if (!names.length) return '';
    const line = `${ip} ${names.join(' ')}`;
    switch (this.os()) {
      case 'windows':
        return (
          `# PowerShell, run as Administrator\n` +
          `Add-Content -Path "$env:windir\\System32\\drivers\\etc\\hosts" -Value "${line}"\n` +
          `ipconfig /flushdns\n`
        );
      case 'macos':
        return (
          `echo "${line}" | sudo tee -a /etc/hosts\n` +
          `sudo dscacheutil -flushcache; sudo killall -HUP mDNSResponder\n`
        );
      default:
        return `echo "${line}" | sudo tee -a /etc/hosts\n`;
    }
  });

  protected readonly caCmd = computed(() => {
    switch (this.os()) {
      case 'windows':
        return (
          `# PowerShell, with Chocolatey (or: scoop bucket add extras; scoop install mkcert)\n` +
          `choco install mkcert\n` +
          `mkcert -install\n`
        );
      case 'macos':
        return `brew install mkcert nss\nmkcert -install\n`;
      default:
        return (
          `# Debian, Ubuntu (Fedora: sudo dnf install nss-tools)\n` +
          `sudo apt install libnss3-tools\n` +
          `curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"\n` +
          `chmod +x mkcert-v*-linux-amd64\n` +
          `sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert\n` +
          `mkcert -install\n`
        );
    }
  });

  // ONE certificate for every name: imported once, placed on both planes.
  protected readonly certCmd = computed(
    () => `mkcert -cert-file meerkat.pem -key-file meerkat-key.pem ${this.shown().join(' ')}\n`,
  );
}

// The answer of an authority, pasted back - or its file chosen or dropped. A
// dedicated dialog rather than the shared prompt: a certificate is twenty lines
// of base64, and a single-line box would make checking what was pasted
// impossible.
@Component({
  selector: 'app-adopt-dialog',
  imports: [MatButtonModule, MatDialogModule, MatFormFieldModule, MatIconModule, MatInputModule, FormFieldComponent],
  template: `
    <h2 mat-dialog-title i18n="@@Adopt_the_signed_certificate">Adopt the signed certificate</h2>
    <mat-dialog-content
      class="adopt"
      [class.over]="over()"
      (dragover)="dragOver($event)"
      (dragleave)="over.set(false)"
      (drop)="drop($event)"
    >
      <p i18n="@@Adopt_note2">
        The certificate the authority signed for this request: paste it, choose its file, or drop
        it here. It must match the private key this request was made with, which never left.
      </p>
      <div class="file">
        <button matButton="outlined" type="button" (click)="picker.click()">
          <mat-icon>folder_open</mat-icon>
          <ng-container i18n="@@Choose_the_file">Choose the file</ng-container>
        </button>
        <input #picker type="file" accept=".pem,.crt,.cer,.txt" hidden (change)="pick($event)" />
        @if (fileName()) {
          <span class="picked">{{ fileName() }}</span>
        }
      </div>
      <app-form-field i18n-label="@@Certificate_PEM" label="Certificate (PEM)">
        <textarea
          matInput
          rows="10"
          [value]="pem()"
          (input)="pem.set($any($event.target).value)"
          placeholder="-----BEGIN CERTIFICATE-----"
        ></textarea>
      </app-form-field>
      @if (error()) {
        <p class="error">{{ error() }}</p>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!pem().trim()" [mat-dialog-close]="pem()" i18n="@@Adopt">
        Adopt
      </button>
    </mat-dialog-actions>
  `,
  styles: `
    /* A grid, so the field takes the dialog's width: app-form-field sizes
       to its content otherwise, and twenty lines of base64 wrapped in a
       narrow box cannot be checked by eye. */
    .adopt {
      display: grid;
      gap: 4px;
      border-radius: 8px;
      outline: 2px dashed transparent;
      outline-offset: -4px;
    }
    .adopt.over {
      outline-color: var(--mat-sys-primary);
    }
    .adopt p {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
      margin: 0 0 8px;
    }
    .adopt .error {
      color: var(--mat-sys-error);
    }
    .file {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 8px;
    }
    .picked {
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
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
  protected readonly fileName = signal('');
  protected readonly over = signal(false);
  protected readonly error = signal('');

  protected dragOver(e: DragEvent): void {
    if (!e.dataTransfer?.types.includes('Files')) return;
    e.preventDefault();
    this.over.set(true);
  }

  protected async drop(e: DragEvent): Promise<void> {
    e.preventDefault();
    this.over.set(false);
    const file = e.dataTransfer?.files?.[0];
    if (file) await this.read(file);
  }

  protected async pick(e: Event): Promise<void> {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (file) await this.read(file);
  }

  // Only the certificate blocks: an authority often sends the chain in the
  // same file, which is welcome, and sometimes more, which is not.
  private async read(file: File): Promise<void> {
    this.error.set('');
    const certs = (await file.text()).match(/-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g);
    if (!certs) {
      this.error.set($localize`:@@Not_a_certificate_file:${file.name}:name: holds no certificate`);
      return;
    }
    this.pem.set(certs.join('\n') + '\n');
    this.fileName.set(file.name);
  }
}
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

function guessOs(): Os {
  const ua = navigator.userAgent;
  if (/Windows/i.test(ua)) return 'windows';
  if (/Mac/i.test(ua)) return 'macos';
  return 'linux';
}

// The rule x509 applies: an exact match, or a wildcard standing for exactly
// one label - *.example.com covers app.example.com, not example.com.
function covers(names: string[], host: string): boolean {
  const h = host.toLowerCase();
  return names.some((raw) => {
    const n = raw.toLowerCase();
    if (n === h) return true;
    if (!n.startsWith('*.')) return false;
    const rest = h.slice(0, h.length - (n.length - 1));
    return h.endsWith(n.slice(1)) && rest !== '' && !rest.includes('.');
  });
}

function isAddress(name: string): boolean {
  return /^[\d.]+$/.test(name) || name.includes(':');
}
