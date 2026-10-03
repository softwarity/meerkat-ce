import { MatSidenavModule } from '@angular/material/sidenav';
import { MatCardModule } from '@angular/material/card';
import { Component, DestroyRef, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { RouterLink } from '@angular/router';
import { ApiService, PlugState } from '../api.service';
import { EeLockComponent } from '../shared/ee-lock.component';
import { FormFieldComponent } from '../shared/form-field.component';
import { SnippetComponent } from '../shared/snippet.component';

// The developer tunnel (DEV-11): Infra, Plug.
//
// plug runs a developer's local process as a member of the cluster - its
// service under its real name, reaching the others by theirs - through a
// tunnel this gateway opens. The page is the infrastructure half: the switch,
// the address developers type, and the commands they run, stamped with that
// address so they are copied rather than translated. Developer mode
// (Application, General) is the other condition, and the page says so rather
// than offering it: it is the application administrator's.
type Os = 'macos' | 'linux' | 'windows';

@Component({
  selector: 'app-plug-page',
  imports: [
    MatCardModule,
    EeLockComponent,
    FormFieldComponent,
    MatButtonModule,
    MatButtonToggleModule,
    MatIconModule,
    MatInputModule,
    MatSlideToggleModule,
    MatSidenavModule,
    RouterLink,
    SnippetComponent,
  ],
  templateUrl: './plug-page.component.html',
  styleUrl: './plug-page.component.scss',
})
export class PlugPageComponent {
  private readonly api = inject(ApiService);
  // The help drawer: what a developer runs on their machine.
  protected readonly helpOpen = signal(false);
  private readonly snack = inject(MatSnackBar);
  private readonly destroyRef = inject(DestroyRef);

  protected readonly state = signal<PlugState | undefined>(undefined);
  protected readonly saving = signal(false);
  protected readonly error = signal('');
  // The address as being typed; saved with its own button, since saving on
  // every keystroke is a setting rewritten a dozen times per hostname. It
  // starts on the defaults - localhost and the tunnel's own port, which is
  // what a port-forward or a published Docker port gives on the developer's
  // own machine - so the commands below read as commands from the start.
  protected readonly host = signal('localhost');
  protected readonly port = signal<number | null>(22222);
  protected readonly os = signal<Os>(this.guessOs());
  // What the developer calls this cluster on their machine. The installer
  // names the profile after the host; "localhost" or an IP says nothing once
  // a second cluster is installed, so the page offers a rename - optional,
  // local to the commands, never saved.
  protected readonly profile = signal('profile1');
  protected readonly profileName = computed(() => this.profile().trim() || this.addr().host);
  protected readonly renameCmd = computed(
    () => `plug rn ${this.addr().host} ${this.profileName()}\n`,
  );

  protected readonly enabled = computed(() => !!this.state()?.enabled);
  protected readonly addressDirty = computed(() => {
    const s = this.state();
    if (!s) return false;
    return (
      this.addr().host !== (s.host || 'localhost') ||
      this.addr().port !== (s.port || s.defaultPort)
    );
  });

  // What the commands carry: the fields AS THEY ARE, so typing the published
  // address rewrites every command below while it is typed - the page is also
  // where somebody works out what that address is. An emptied field falls
  // back to its default rather than to a blank inside a command.
  protected readonly addr = computed(() => ({
    host: this.host().trim() || 'localhost',
    port: this.port() || this.state()?.defaultPort || 22222,
  }));

  protected readonly installCmd = computed(() => {
    const { host, port } = this.addr();
    const opts = `-p ${port} -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null`;
    return this.os() === 'windows'
      ? `ssh -n ${opts} get@${host} install-windows | bash -s -- ${host} ${port}\n`
      : `ssh ${opts} get@${host} install | sh\n`;
  });

  // The installer names the profile after the host, which is why keygen and
  // pubkey take it: with a second cluster installed, the bare verb would ask.
  protected readonly keyCmd = computed(() => {
    const host = this.profileName();
    const copy = { macos: ' | pbcopy', linux: ' | xclip -selection clipboard', windows: ' | clip' }[
      this.os()
    ];
    return `plug keygen -p ${host}\nplug pubkey -p ${host}${copy}\n`;
  });

  protected readonly runCmd = computed(
    () => `plug -p ${this.profileName()} -s my-service:8080:3000 npm run start\n`,
  );

  protected readonly keyPage = computed(() => `${this.state()?.dataOrigin ?? ''}/profile/dev/key`);

  constructor() {
    this.load();
  }

  private guessOs(): Os {
    const ua = navigator.userAgent;
    if (/Windows/i.test(ua)) return 'windows';
    if (/Mac/i.test(ua)) return 'macos';
    return 'linux';
  }

  private load() {
    this.api
      .plugSetting()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((s) => this.take(s));
  }

  private take(s: PlugState) {
    this.state.set(s);
    this.host.set(s.host || 'localhost');
    this.port.set(s.port || s.defaultPort);
  }

  protected setPort(raw: string) {
    const n = Number(raw);
    this.port.set(raw.trim() === '' || Number.isNaN(n) ? null : Math.round(n));
  }

  // The switch APPLIES ON CLICK, like every other switch in Infra; the
  // address keeps its Save button.
  protected toggle(on: boolean) {
    this.save({ enabled: on, host: this.state()?.host, port: this.state()?.port });
  }

  protected saveAddress() {
    this.save({ enabled: this.enabled(), host: this.addr().host, port: this.addr().port });
  }

  private save(cfg: { enabled: boolean; host?: string; port?: number }) {
    this.saving.set(true);
    this.error.set('');
    this.api
      .setPlugSetting(cfg)
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (s) => {
          this.take(s);
          this.saving.set(false);
          this.snack.open($localize`:@@Plug_saved:Saved`, undefined, { duration: 2500 });
        },
        // The gateway's own sentence, and the switch put back where the
        // gateway still holds it.
        error: (e: { error?: { error?: string } }) => {
          this.saving.set(false);
          this.error.set(e.error?.error ?? $localize`:@@Plug_save_failed:The setting could not be saved.`);
          this.load();
        },
      });
  }
}
