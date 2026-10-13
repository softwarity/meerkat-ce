import { COMMA, ENTER, SPACE } from '@angular/cdk/keycodes';
import { Component, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatChipInputEvent, MatChipsModule } from '@angular/material/chips';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, TlsSettings } from '../../api.service';

// The names a door is ALSO reached by through a proxy that terminates TLS in
// front of the gateway - nginx, an ingress, a cloud balancer (AUTH-22).
//
// Behind such a proxy no request arrives on the gateway's own TLS, so every
// mail that proves a mailbox - a reset, a sign-up, a new address - carried a
// code. A name declared here is the operator vouching for it: it is ours, and
// somebody serves it over HTTPS. The mailed link may then point at it. That
// is all it does: no certificate, no redirect, no HSTS - the gateway has
// nothing to present for it, so it promises nothing on it.
//
// Each chip added or removed is saved at once, like the switches beside it:
// the gateway answers with the list as it keeps it (lowercased, each once),
// or with the sentence that says why a name is refused.
@Component({
  selector: 'app-proxied-names',
  imports: [MatButtonModule, MatChipsModule, MatFormFieldModule, MatIconModule, MatTooltipModule],
  templateUrl: './proxied-names.component.html',
  styleUrl: './proxied-names.component.scss',
})
export class ProxiedNamesComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  readonly plane = input.required<'console' | 'app'>();
  readonly names = input.required<string[]>();
  // The whole TLS view the gateway answered with, for the page to show.
  readonly saved = output<TlsSettings>();

  protected readonly saving = signal(false);
  // A name is never more than letters, digits, dots and hyphens: a space or a
  // comma ends one, so a list typed or pasted in one go is several chips.
  protected readonly separators = [ENTER, COMMA, SPACE];

  protected readonly info = $localize`:@@Proxied_names_info:Names a proxy in front of this gateway (nginx, an ingress, a load balancer) serves over HTTPS. A password reset or an address confirmation asked under one of them mails a link back to it instead of a code. The gateway has no certificate for them: they open no HTTPS door here, and are never redirected or sent HSTS. One full name each, no wildcard: a wildcard would vouch for names that may no longer be yours.`;

  // Not while a save is out: Enter then the blur that follows would send the
  // same name twice. The grid is not disabled for it - that takes the focus
  // away from somebody typing the next one.
  protected add(event: MatChipInputEvent): void {
    if (this.saving()) return;
    const typed = event.value
      .split(/[\s,]+/)
      .map((n) => n.trim())
      .filter((n) => n !== '');
    if (!typed.length) return;
    this.save([...this.names(), ...typed], () => event.chipInput.clear());
  }

  protected remove(name: string): void {
    if (this.saving()) return;
    this.save(this.names().filter((n) => n !== name));
  }

  // On a refusal the typed text stays in the field, to be corrected rather
  // than typed again.
  private save(names: string[], done?: () => void): void {
    this.saving.set(true);
    this.api.saveProxiedNames(this.plane(), names).subscribe({
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
