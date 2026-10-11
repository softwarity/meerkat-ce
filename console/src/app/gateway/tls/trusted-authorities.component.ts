import { DatePipe } from '@angular/common';
import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { firstValueFrom } from 'rxjs';
import { ApiService, TrustedAuthority } from '../../api.service';
import { LiveChangesService } from '../../shared/live-changes.service';

// The authorities the gateway trusts when IT calls (SSL-09): an upstream in
// HTTPS signed by a cluster's or a company's own CA. Its own section, and its
// own store, apart from the certificates above: those are what the gateway
// presents, with their keys; these are what it accepts, public certificates
// only.
//
// Added in a dialog: a name, and the certificate chosen, dropped or pasted.
@Component({
  selector: 'app-trusted-authorities',
  imports: [DatePipe, MatButtonModule, MatIconModule, MatTooltipModule],
  templateUrl: './trusted-authorities.component.html',
  styleUrl: './trusted-authorities.component.scss',
})
export class TrustedAuthoritiesComponent {
  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);

  protected readonly list = signal<TrustedAuthority[]>([]);

  constructor() {
    this.load();
    inject(LiveChangesService).on('trusted-authority', () => this.load());
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

  protected remove(a: TrustedAuthority): void {
    this.api.deleteTrustedAuthority(a.id).subscribe({
      next: () => this.load(),
      error: () => this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 }),
    });
  }

  protected expiry(t: number): number {
    return t * 1000;
  }
}
