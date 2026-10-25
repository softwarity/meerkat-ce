import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, PlatformAuthorities, TrustedAuthority } from '../../api.service';
import { LiveChangesService } from '../../shared/live-changes.service';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { MeService } from '../../me.service';

// The authorities the gateway trusts when IT calls (SSL-09): an upstream in
// HTTPS signed by a cluster's or a company's own CA. Its own section, and its
// own store, apart from the certificates above: those are what the gateway
// presents, with their keys; these are what it accepts, public certificates
// only.
//
// Added in a dialog: a name, and the certificate chosen, dropped or pasted -
// or, on a cluster, picked among the authorities it publishes.
@Component({
  selector: 'app-trusted-authorities',
  imports: [MatButtonModule, MatIconModule, MatProgressSpinnerModule, MatTooltipModule, ZonedDatePipe],
  templateUrl: './trusted-authorities.component.html',
  styleUrl: './trusted-authorities.component.scss',
})
export class TrustedAuthoritiesComponent {
  private readonly api = inject(ApiService);
  protected readonly zone = inject(MeService).timezone;
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);

  protected readonly list = signal<TrustedAuthority[]>([]);
  // What the cluster publishes, asked once the section opens: the search
  // dials every route in https, which takes seconds, so the button shows at
  // once, disabled with a spinner, and the dialog opens on the answer instead
  // of asking again. Not on a cluster, the button goes.
  protected readonly platform = signal<PlatformAuthorities | undefined>(undefined);
  protected readonly searching = signal(true);

  constructor() {
    this.load();
    this.search();
    inject(LiveChangesService).on('trusted-authority', () => this.load());
  }

  private search(): void {
    this.searching.set(true);
    this.api.platformAuthorities().subscribe({
      next: (r) => {
        this.platform.set(r);
        this.searching.set(false);
      },
      error: () => this.searching.set(false),
    });
  }

  private load(): void {
    this.api.listTrustedAuthorities().subscribe({ next: (l) => this.list.set(l), error: () => this.list.set([]) });
  }

  protected async add(): Promise<void> {
    const { TrustDialogComponent } = await import('./trust-dialog.component');
    const created = await firstValueFrom(
      this.dialog
        .open(TrustDialogComponent, {
          restoreFocus: true,
          // Wider than Material's 560px: a PEM line is 64 characters.
          width: '680px',
          maxWidth: '92vw',
        })
        .afterClosed(),
    );
    if (!created) return;
    this.load();
    this.snack.open($localize`:@@Trust_added:Authority trusted`, undefined, { duration: 2000 });
  }

  protected async fromCluster(): Promise<void> {
    const { PlatformTrustDialogComponent } = await import('./platform-trust-dialog.component');
    const taken = await firstValueFrom(
      this.dialog
        .open(PlatformTrustDialogComponent, { restoreFocus: true, width: '680px', maxWidth: '92vw', data: this.platform() })
        .afterClosed(),
    );
    if (!taken) return;
    this.load();
    this.search(); // what is trusted now is no longer offered
    this.snack.open($localize`:@@Trust_added:Authority trusted`, undefined, { duration: 2000 });
  }

  protected remove(a: TrustedAuthority): void {
    this.api.deleteTrustedAuthority(a.id).subscribe({
      next: () => this.load(),
      error: () => this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 }),
    });
  }
}
