import { COMMA, ENTER, SPACE } from '@angular/cdk/keycodes';
import { Component, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, DoorSettings, DoorView, TlsSettings } from '../../api.service';

// What a plane's door says beyond its mode (SSL-14): its PUBLIC names, the
// full names a browser reaches it by, and - behind a proxy - where that proxy
// reaches the gateway from.
//
// One full name each, never a wildcard, whoever serves them: they are what a
// mailed link is written with, and a wildcard would vouch for names that may
// no longer be ours. Served by this gateway, a name added needs a certificate
// placed here that covers it (the certificate itself may be a wildcard).
//
// Each chip added or removed is saved at once, like the switches beside it:
// the gateway answers with the whole view as it keeps it, or with the
// sentence that says why it refused.
@Component({
  selector: 'app-door-names',
  imports: [MatButtonModule, MatChipsModule, MatFormFieldModule, MatIconModule, MatTooltipModule],
  templateUrl: './door-names.component.html',
  styleUrl: './door-names.component.scss',
})
export class DoorNamesComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  readonly plane = input.required<'console' | 'app'>();
  readonly door = input.required<DoorView>();
  readonly saved = output<TlsSettings>();

  protected readonly saving = signal(false);
  // A name is never more than letters, digits, dots and hyphens: a space or a
  // comma ends one, so a list typed or pasted in one go is several chips.
  protected readonly separators = [ENTER, COMMA, SPACE];

  protected readonly namesInfo = $localize`:@@Door_names_info:The exact names browsers use to reach this plane, no wildcards. When Meerkat serves HTTPS, each one needs a certificate here that covers it.`;
  protected readonly addressesInfo = $localize`:@@Door_proxy_addresses_info2:Meerkat reads the visitor's address and scheme from the X-Forwarded-For and X-Forwarded-Proto headers, trusted only from these addresses. Empty, any private address counts as the proxy.`;

  protected addName(event: MatChipInputEvent): void {
    if (this.saving()) return;
    const typed = split(event.value);
    if (!typed.length) return;
    this.save({ names: [...this.door().names, ...typed] }, () => event.chipInput.clear());
  }

  protected removeName(name: string): void {
    if (this.saving()) return;
    this.save({ names: this.door().names.filter((n) => n !== name) });
  }

  protected addAddress(event: MatChipInputEvent): void {
    if (this.saving()) return;
    const typed = split(event.value);
    if (!typed.length) return;
    this.save({ proxyAddresses: [...this.door().proxyAddresses, ...typed] }, () => event.chipInput.clear());
  }

  protected removeAddress(address: string): void {
    if (this.saving()) return;
    this.save({ proxyAddresses: this.door().proxyAddresses.filter((a) => a !== address) });
  }

  // On a refusal the typed text stays in the field, to be corrected rather
  // than typed again.
  private save(patch: Partial<DoorSettings>, done?: () => void): void {
    const d = this.door();
    this.saving.set(true);
    this.api
      .saveDoor(this.plane(), { proxied: d.proxied, names: d.names, proxyAddresses: d.proxyAddresses, ...patch })
      .subscribe({
        next: (t) => {
          this.saving.set(false);
          done?.();
          this.saved.emit(t);
        },
        error: (e) => {
          this.saving.set(false);
          const err = e as { error?: { error?: string }; message?: string };
          this.snack.open(
            err?.error?.error ?? err?.message ?? $localize`:@@Something_went_wrong:Something went wrong`,
            undefined,
            { duration: 6000 },
          );
        },
      });
  }
}

function split(raw: string): string[] {
  return raw
    .split(/[\s,]+/)
    .map((n) => n.trim())
    .filter((n) => n !== '');
}
