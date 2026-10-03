import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatRadioModule } from '@angular/material/radio';
import { ApiService, ConfigPullResult, ConfigRemote } from '../../api.service';
import { PlanTableComponent } from './plan-table.component';

export interface PickLocationData {
  // 'pull' reads a location into a new saved configuration; 'push' sends one to
  // a location. The window is the same list; the sentences are not.
  way: 'pull' | 'push';
  // What is being pushed, for the push wording.
  configuration?: string;
  // The location already bound, preselected - so "change the destination" opens
  // on the current answer rather than on nothing.
  selected?: string;
  // Names already taken, to keep a pull from proposing one.
  taken?: string[];
}

export type PickLocationOutcome = { remoteId: string; name: string };

// Which location, asked once.
//
// The choice is then REMEMBERED on the row, which is the point: this window is
// for the first time and for a change of destination, not for every push. A
// dialog that reappeared at every push would default to nothing, and a push
// whose destination defaults to nothing is one wrong click away from another
// customer's directory.
@Component({
  selector: 'app-pick-location-dialog',
  imports: [
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatRadioModule,
  ],
  styles: [
    `
      mat-radio-button {
        display: block;
      }
      .where {
        margin: -6px 0 10px 32px;
        font-family: var(--mk-mono, monospace);
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-form-field {
        width: 100%;
        margin-top: 8px;
      }
      .empty {
        margin: 0;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-hint.clash {
        color: var(--mat-sys-error);
      }
      .note {
        margin: 4px 0 0;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>
      @if (data.way === 'pull') {
        <ng-container i18n="@@Import_from_git">Import from git</ng-container>
      } @else {
        <ng-container i18n="@@Export_to_git">Export to git</ng-container>
      }
    </h2>
    <mat-dialog-content>
      @if (remotes().length === 0) {
        <p class="empty" i18n="@@No_location_yet">
          No git location is set up yet. Add one from Git locations first.
        </p>
      } @else {
        <mat-radio-group [value]="picked()" (change)="pick($event.value)">
          @for (r of remotes(); track r.id) {
            <mat-radio-button [value]="r.id">{{ r.name }}</mat-radio-button>
            <p class="where">{{ where(r) }}</p>
          }
        </mat-radio-group>

        @if (data.way === 'pull') {
          <mat-form-field>
            <mat-label i18n="@@Save_it_as">Save it as</mat-label>
            <input matInput [value]="name()" (input)="name.set($any($event.target).value)" />
            <mat-hint [class.clash]="clash()">
              @if (clash()) {
                <ng-container i18n="@@Name_taken">That name is already used.</ng-container>
              }
            </mat-hint>
          </mat-form-field>
          <!-- The one line that stays, because it is what makes this safe to
               try: everything else about the act is said by the button. The
               push side had a note explaining that the choice is remembered -
               UI behaviour, told to somebody who will find out by using it. -->
          <p class="note" i18n="@@Pull_applies_nothing">
            Nothing is applied: it is saved beside what is running.
          </p>
        }
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!ready()" (click)="confirm()" cdkFocusInitial>
        @if (data.way === 'pull') {
          <ng-container i18n="@@Pull">Pull</ng-container>
        } @else {
          <ng-container i18n="@@Push">Push</ng-container>
        }
      </button>
    </mat-dialog-actions>
  `,
})
export class PickLocationDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<PickLocationDialogComponent, PickLocationOutcome>);
  protected readonly data = inject<PickLocationData>(MAT_DIALOG_DATA);

  protected readonly remotes = signal<ConfigRemote[]>([]);
  protected readonly picked = signal('');
  protected readonly name = signal('');

  // A pull names what it creates, and the location's own name is the obvious
  // default: it is what the operator called that platform.
  protected readonly clash = computed(() =>
    (this.data.taken ?? []).some(
      (t) => t.trim().toLocaleLowerCase() === this.name().trim().toLocaleLowerCase(),
    ),
  );

  protected readonly ready = computed(() => {
    if (!this.picked()) return false;
    if (this.data.way === 'push') return true;
    return !!this.name().trim() && !this.clash();
  });

  constructor() {
    this.api.configRemotes().subscribe({
      next: (list) => {
        this.remotes.set(list);
        const start = this.data.selected && list.some((r) => r.id === this.data.selected)
          ? this.data.selected
          : list.length === 1
            ? list[0].id
            : '';
        if (start) this.pick(start);
      },
      error: () => this.remotes.set([]),
    });
  }

  protected pick(id: string): void {
    this.picked.set(id);
    if (this.data.way !== 'pull') return;
    const r = this.remotes().find((o) => o.id === id);
    if (r && !this.name().trim()) this.name.set(r.name);
  }

  protected where(r: ConfigRemote): string {
    return r.url + ' (' + r.branch + ')' + (r.dir ? ' / ' + r.dir : '');
  }

  protected confirm(): void {
    this.ref.close({ remoteId: this.picked(), name: this.name().trim() });
  }
}

// What a pull brought, and what activating it would change.
//
// Shown straight after the pull rather than left for a second click, because
// "what would this do to my gateway" is the only question anybody has at that
// moment - and the answer has to be read BEFORE the switch, which is the whole
// loop this feature exists for: pull, read, activate, read again.
@Component({
  selector: 'app-pull-result-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule, PlanTableComponent],
  styles: [
    `
      .from {
        margin: 0 0 4px;
        font-size: 0.85rem;
      }
      .rev {
        margin: 0 0 12px;
        font-family: var(--mk-mono, monospace);
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .note {
        display: flex;
        gap: 12px;
        padding: 12px 16px;
        border-radius: 8px;
        background: var(--mat-sys-surface-container);
        margin: 12px 0 0;
      }
      .note mat-icon {
        flex-shrink: 0;
        color: var(--mat-sys-on-surface-variant);
      }
      .note p {
        margin: 0;
        font-size: 0.85rem;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Pulled_from_git">Pulled from git</h2>
    <mat-dialog-content>
      <p class="from" i18n="@@Pulled_into">
        Saved as {{ data.result.configuration.name }}, from {{ data.location }}.
      </p>
      @if (data.result.configuration.remoteRev; as rev) {
        <p class="rev">{{ rev }}</p>
      }
      @if (data.result.planError) {
        <div class="note">
          <mat-icon>error</mat-icon>
          <p>{{ data.result.planError }}</p>
        </div>
      } @else if (data.result.plan; as plan) {
        <p class="from" i18n="@@What_activating_would_change">
          What serving it would change:
        </p>
        <app-plan-table
          [plan]="plan"
          i18n-nothing="@@Pull_no_change"
          nothing="This gateway already matches what the repository holds."
        />
      } @else {
        <!-- No plan sent back: nothing would change, said rather than left
             as a heading over an empty list. -->
        <div class="note">
          <mat-icon>check_circle</mat-icon>
          <p i18n="@@Pull_no_change">This gateway already matches what the repository holds.</p>
        </div>
      }
      <div class="note">
        <mat-icon>info</mat-icon>
        <p i18n="@@Pull_applied_nothing">
          Nothing was applied. This gateway still serves what it served - use Set as current
          when the list above says what you expected.
        </p>
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton="filled" mat-dialog-close cdkFocusInitial i18n="@@Close">Close</button>
    </mat-dialog-actions>
  `,
})
export class PullResultDialogComponent {
  protected readonly data = inject<{ result: ConfigPullResult; location: string }>(MAT_DIALOG_DATA);
}
