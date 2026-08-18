import { Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, AuthProvider } from '../../api.service';

// The accounts held here, as an authority one can open. It used to be a row
// with nothing behind it, while the two questions it owns - may someone sign
// themselves up, and is the form guarded - lived in Settings, Security, next
// to policies about sessions and second factors.
//
// They belong here: every other authority answers "may a first arrival create
// an account" on its own screen, and this one is the same question asked of
// the same list. The captcha comes with it because it guards THIS authority's
// public form and nothing else - an arrival through a directory was already
// made to prove itself over there.
@Component({
  selector: 'app-local-provider-editor',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatSelectModule, MatSlideToggleModule],
  styleUrl: './local-provider-editor.component.scss',
  template: `
    <div class="head">
      <h2 i18n="@@Local_accounts">Local accounts</h2>
      <button matIconButton (click)="closed.emit()" i18n-aria-label="@@Close" aria-label="Close">
        <mat-icon>close</mat-icon>
      </button>
    </div>

    <section>
      <mat-form-field class="policy">
        <mat-label i18n="@@Self_registration">Self-registration</mat-label>
        <mat-select [value]="autoCreate()" (selectionChange)="setAutoCreate($event.value)">
          <mat-option value="" i18n="@@Inherited_application">Inherited from the application</mat-option>
          <mat-option value="yes" i18n="@@Allowed">Allowed</mat-option>
          <mat-option value="no" i18n="@@Refused">Refused</mat-option>
        </mat-select>
      </mat-form-field>
      <p class="hint text-muted" i18n="@@Local_sign_up_hint">
        A sign-up form on the sign-in page, with the address confirmed by e-mail. Needs a mail
        relay (Infra, Mail relay).
      </p>

      <!-- Disabled rather than hidden: a switch that disappears leaves nothing
           to explain why, and someone looking for the captcha would think it
           had been removed. -->
      <mat-slide-toggle
        [checked]="captcha()"
        [disabled]="!signUpOpen()"
        (change)="setCaptcha($event.checked)"
      >
        <ng-container i18n="@@Anti_robot_check">Anti-robot check</ng-container>
      </mat-slide-toggle>
      <p class="hint text-muted" i18n="@@Local_captcha_hint">
        Guards that form, and only it.
      </p>
    </section>
  `,
})
export class LocalProviderEditorComponent {
  readonly provider = input.required<AuthProvider>();
  readonly saved = output<AuthProvider>();
  readonly closed = output<void>();

  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  // Optimistic, and snapped back by the emitted provider on failure: these are
  // two switches, and waiting for a round trip to move them reads as a bug.
  protected readonly autoCreate = signal('');
  protected readonly captcha = signal(true);
  // Whether there is a form to guard at all: this authority's own answer, or
  // the application-wide default when it has none.
  protected readonly signUpOpen = computed(() =>
    this.autoCreate() === 'yes' || (this.autoCreate() === '' && this.appDefault()),
  );

  constructor() {
    queueMicrotask(() => {
      this.autoCreate.set(this.provider().autoCreate ?? '');
      this.captcha.set(this.provider().captcha ?? true);
    });
    this.api.settings().subscribe({ next: (s) => this.appDefault.set(s.selfRegistration) });
  }

  // What the application decides for an authority that has no answer.
  protected readonly appDefault = signal(false);

  protected setAutoCreate(value: string): void {
    this.autoCreate.set(value);
    this.save();
  }

  protected setCaptcha(on: boolean): void {
    this.captcha.set(on);
    this.save();
  }

  private save(): void {
    this.api
      .saveAuthProvider({ ...this.provider(), autoCreate: this.autoCreate(), captcha: this.captcha() })
      .subscribe({
        next: (p) => this.saved.emit(p),
        error: (err) => {
          this.autoCreate.set(this.provider().autoCreate ?? '');
          this.captcha.set(this.provider().captcha ?? true);
          this.snack.open(errMsg(err), undefined, { duration: 4000 });
        },
      });
  }
}

function errMsg(err: unknown): string {
  const e = err as { error?: { error?: string } };
  return typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`;
}
