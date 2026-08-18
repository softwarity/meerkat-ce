import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, Settings } from '../api.service';
import { DialogsService } from '../shared/dialogs.service';
import { humanDuration } from '../shared/duration';

const TRUST_TTL_CHOICES = ['P1D', 'P7D', 'P14D', 'P30D'];
const SESSION_TTL_CHOICES = ['PT15M', 'PT30M', 'PT1H', 'PT2H', 'PT4H', 'PT8H', 'PT12H', 'P1D'];

// Application-wide security policy (root only): how long a session lives, second
// factor (MFA-04), passkeys (AUTH-15), API tokens (AUTH-16), self-registration
// (AUTH-20), rate limiting (SEC-10), trusted browsers (MFA-03), and the outbound
// e-mail sender the account flows use. A full PUT of /api/settings - the other
// fields ride along untouched.
//
// The session TTL sits here, not on General: how long one stays signed in is a
// security policy, next to the second factor and the trusted browsers that gate
// the very same session.
@Component({
  selector: 'app-security-page',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    LoadingIndicatorComponent,
  ],
  styleUrl: './security-page.component.scss',
  templateUrl: './security-page.component.html',
})
export class SecurityPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  private readonly locale = inject(LOCALE_ID);

  protected human(iso: string): string {
    return humanDuration(iso, this.locale);
  }

  protected readonly loading = signal(true);
  protected readonly saving = signal(false);
  private readonly settings = signal<Settings | null>(null);

  protected readonly mfaRequired = signal(false);
  protected readonly rlLogin = signal(10);
  protected readonly rlTotp = signal(5);
  protected readonly rlWindow = signal('PT15M');
  protected readonly rlWindows = ['PT5M', 'PT15M', 'PT1H'];
  protected readonly passkeysAllowed = signal(true);
  protected readonly apiTokens = signal(true);
  protected readonly trustAllowed = signal(false);
  protected readonly trustTtl = signal('P7D');
  protected readonly ttlChoices = TRUST_TTL_CHOICES;
  protected readonly sessionTTL = signal('');

  // The password policy (AUTH-10). Zero means the rule is not asked for, and
  // a rule that is not asked for never appears on the checklist users see.
  protected readonly pwLength = signal(8);
  protected readonly pwLower = signal(0);
  protected readonly pwUpper = signal(0);
  protected readonly pwDigits = signal(0);
  protected readonly pwSpecial = signal(0);
  // The two rules that are about TIME rather than shape: a password may not
  // come back, and it does not last forever.
  protected readonly pwHistory = signal(0);
  protected readonly pwExpiry = signal(0);

  // The checklist as the sign-up, reset and profile pages will draw it. Shown
  // here because a policy is easy to write and hard to picture: four numbers
  // in four boxes say nothing about what the person on the other side reads.
  protected readonly pwPreview = computed(() => {
    const rules: { label: string; need: number }[] = [
      { label: $localize`:@@Length:Length`, need: this.pwLength() },
      { label: $localize`:@@Lowercase:Lowercase`, need: this.pwLower() },
      { label: $localize`:@@Uppercase:Uppercase`, need: this.pwUpper() },
      { label: $localize`:@@Digits:Digits`, need: this.pwDigits() },
      { label: $localize`:@@Special_characters:Special characters`, need: this.pwSpecial() },
    ];
    return rules.filter((r) => r.need > 0).map((r) => `${r.label}: ${r.need}`);
  });

  // The server raises the length to the sum of the parts rather than store a
  // rule it could never satisfy. Saying so beforehand beats watching the
  // number change by itself after saving.
  protected readonly pwSum = computed(
    () => this.pwLower() + this.pwUpper() + this.pwDigits() + this.pwSpecial(),
  );

  // The presets, plus whatever non-preset value the store already holds so the
  // select never silently drops it.
  protected readonly sessionTtlChoices = computed(() => {
    const current = this.sessionTTL();
    return current && !SESSION_TTL_CHOICES.includes(current)
      ? [current, ...SESSION_TTL_CHOICES]
      : SESSION_TTL_CHOICES;
  });



  constructor() {
    this.api.settings().subscribe({
      next: (s) => {
        this.settings.set(s);
        this.mfaRequired.set(s.mfaRequired);
        this.rlLogin.set(s.rateLimit?.loginAttempts ?? 10);
        this.rlTotp.set(s.rateLimit?.totpAttempts ?? 5);
        this.rlWindow.set(s.rateLimit?.loginWindow || 'PT15M');
        this.passkeysAllowed.set(s.passkeysAllowed);
        this.apiTokens.set(s.apiTokens);
        this.trustAllowed.set(s.trustedBrowser?.allowed ?? false);
        this.trustTtl.set(s.trustedBrowser?.ttl || 'P7D');
        this.sessionTTL.set(s.sessionTTL);
        this.pwLength.set(s.passwordPolicy?.minLength ?? 8);
        this.pwLower.set(s.passwordPolicy?.minLower ?? 0);
        this.pwUpper.set(s.passwordPolicy?.minUpper ?? 0);
        this.pwDigits.set(s.passwordPolicy?.minDigits ?? 0);
        this.pwSpecial.set(s.passwordPolicy?.minSpecial ?? 0);
        this.pwHistory.set(s.passwordPolicy?.history ?? 0);
        this.pwExpiry.set(s.passwordPolicy?.expiryDays ?? 0);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected readonly forcing = signal(false);

  // The answer to a leak, where the question is not which account but all of
  // them. Confirmed, because it reaches every person on the installation at
  // once and there is no undoing it one by one.
  protected async forceChangeForAll(): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Force_a_password_change_for_everyone:Force a password change for everyone?`,
      message: $localize`:@@Force_change_everyone_message:Every local account will have to choose a new password at its next sign-in. Accounts signing in through an authority are not affected, and neither are you.`,
      confirmLabel: $localize`:@@Force_the_change:Force the change`,
      danger: true,
    });
    if (!ok) return;
    this.forcing.set(true);
    this.api.mustChangePasswordForAll().subscribe({
      next: ({ users }) => {
        this.forcing.set(false);
        this.snack.open(
          $localize`:@@COUNT_accounts_must_change:${users}:COUNT: accounts must change their password at the next sign-in`,
          undefined,
          { duration: 4000 },
        );
      },
      error: () => this.forcing.set(false),
    });
  }

  protected save(then?: () => void): void {
    const s = this.settings();
    if (!s) return;
    this.saving.set(true);
    this.api
      .saveSettings({
        ...s,
        mfaRequired: this.mfaRequired(),
        rateLimit: { loginAttempts: this.rlLogin(), loginWindow: this.rlWindow(), totpAttempts: this.rlTotp() },
        passkeysAllowed: this.passkeysAllowed(),
        apiTokens: this.apiTokens(),
        trustedBrowser: { allowed: this.trustAllowed(), ttl: this.trustTtl() },
        sessionTTL: this.sessionTTL().trim(),
        passwordPolicy: {
          minLength: this.pwLength(),
          minLower: this.pwLower(),
          minUpper: this.pwUpper(),
          minDigits: this.pwDigits(),
          minSpecial: this.pwSpecial(),
          history: this.pwHistory(),
          expiryDays: this.pwExpiry(),
        },
      })
      .subscribe({
        next: (saved) => {
          this.settings.set(saved);
          this.saving.set(false);
          this.snack.open($localize`:@@Settings_saved:Settings saved`, undefined, { duration: 2500 });
          then?.();
        },
        error: (err: unknown) => {
          this.saving.set(false);
          const e = err as { error?: { error?: string } };
          this.snack.open(
            typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`,
            undefined,
            { duration: 4000 },
          );
        },
      });
  }
}
