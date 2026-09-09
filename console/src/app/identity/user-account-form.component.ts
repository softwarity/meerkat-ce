import { Component, model } from '@angular/core';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

// Who the person is: the login one types, the name one reads, the address one
// was given.
//
// Presentational, and shared by the drawer that CREATES an account and the one
// that edits it - the same three widgets in the same order, so an account is
// not born through one form and corrected through another. A name that could
// only be set at creation would leave every typo permanent.
@Component({
  selector: 'app-user-account-form',
  imports: [MatFormFieldModule, MatInputModule],
  template: `
    <div class="grid">
      <!-- One line, never resizable: an identity field on a textarea looks like
           an input and is invisible to the browser's autofill, which otherwise
           offers the signed-in admin's own name and address while they fill in
           somebody else's account. -->
      <mat-form-field>
        <mat-label i18n="@@Username">Username</mat-label>
        <textarea
          matInput
          rows="1"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          [value]="username()"
          (input)="username.set($any($event.target).value)"
          (keydown.enter)="$event.preventDefault()"
        ></textarea>
      </mat-form-field>
      <mat-form-field>
        <mat-label i18n="@@Full_name">Full name</mat-label>
        <textarea
          matInput
          rows="1"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          [value]="fullname()"
          (input)="fullname.set($any($event.target).value)"
          (keydown.enter)="$event.preventDefault()"
        ></textarea>
      </mat-form-field>
      <!-- Full width, alone on its line: an address is the longest thing on
           this form and the one a support call reads back out loud - a
           220px box turns it into a slot to scroll through. -->
      <mat-form-field class="email">
        <mat-label i18n="@@Email">Email</mat-label>
        <textarea
          matInput
          rows="1"
          spellcheck="false"
          autocapitalize="off"
          autocorrect="off"
          [value]="email()"
          (input)="email.set($any($event.target).value)"
          (keydown.enter)="$event.preventDefault()"
        ></textarea>
      </mat-form-field>
    </div>
  `,
  styles: `
    .grid {
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }
    mat-form-field {
      flex: 1 1 200px;
    }
    .email {
      flex: 1 1 100%;
    }
    textarea {
      resize: none;
    }
  `,
})
export class UserAccountFormComponent {
  readonly username = model('');
  readonly fullname = model('');
  readonly email = model('');
}
