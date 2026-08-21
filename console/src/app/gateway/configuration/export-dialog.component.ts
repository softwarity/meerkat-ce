import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatChipsModule } from '@angular/material/chips';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Router } from '@angular/router';
import { ApiService, ConfigLiteral, ConfigReport, ConfigSection } from '../../api.service';

export interface ExportChoice {
  format: 'yaml' | 'zip';
}

// What leaves, said BEFORE it leaves.
//
// This was a whole screen (the Import/export tab) and it never needed to be:
// it is a sentence, and its moment is the click on Export. Three things are
// worth that moment, and only the second is a warning:
//
//   what the file carries, so nobody opens it to find out;
//   which secrets stay BEHIND because they are stored as values here - the
//     fields will be empty wherever the file lands, and this is the last second
//     anyone can do something about it;
//   which vault names it expects on the other side.
//
// And the choice of form, which is the media rule stated as a button: a plain
// YAML is text and carries no picture, a package carries them beside it.
@Component({
  selector: 'app-export-dialog',
  imports: [MatButtonModule, MatChipsModule, MatDialogModule, MatIconModule, MatTooltipModule],
  styles: [
    `
      mat-dialog-content {
        max-width: min(640px, 84vw);
      }
      dl {
        display: grid;
        grid-template-columns: max-content 1fr;
        gap: 4px 16px;
        margin: 0 0 16px;
        font-size: 0.85rem;
      }
      dt {
        color: var(--mat-sys-on-surface-variant);
      }
      dd {
        margin: 0;
      }
      dd.absent {
        color: var(--mat-sys-on-surface-variant);
      }
      .note {
        display: flex;
        gap: 12px;
        padding: 12px 16px;
        border-radius: 8px;
        background: var(--mat-sys-surface-container);
        margin-bottom: 16px;
      }
      .note mat-icon {
        flex-shrink: 0;
        color: var(--mat-sys-on-surface-variant);
      }
      .note.warn mat-icon {
        color: var(--mat-sys-error);
      }
      .note.caution mat-icon {
        color: var(--mat-sys-tertiary);
      }
      .note .aside {
        margin-top: 4px;
        color: var(--mat-sys-on-surface-variant);
      }
      .note p {
        margin: 0;
        font-size: 0.85rem;
      }
      .note ul {
        margin: 6px 0 0;
        padding-left: 18px;
        font-size: 0.85rem;
      }
      .mono {
        font-family: var(--mk-mono);
        font-size: 0.8rem;
      }
      .link {
        background: none;
        border: 0;
        padding: 0;
        font: inherit;
        color: var(--mat-sys-primary);
        cursor: pointer;
        text-decoration: underline;
      }
      mat-dialog-actions {
        flex-wrap: nowrap;
        gap: 8px;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Export_WHAT">Export {{ data.name }}</h2>
    <mat-dialog-content>
      @if (report(); as r) {
        @if (r.contents.length) {
          <dl>
            <dt i18n="@@It_carries">It carries</dt>
            <dd>{{ carried(r.contents) }}</dd>
          </dl>
        }
        <!-- Raised out of the fact list, because it is the sentence someone
             downloading this is most likely to be wrong about: an export
             reproduces a gateway's CONFIGURATION somewhere else, it does not
             bring an installation back. Amber, not red - red belongs to the
             secrets below, which are the ones that need an action now. -->
        <div class="note caution">
          <mat-icon>info</mat-icon>
          <div>
            <p i18n="@@Not_carried_list2">
              This is not a backup. It does not carry users, organisations and their members,
              sessions, the vault, the audit trail, personal tokens or signing keys.
            </p>
            <p class="aside" i18n="@@Backup_is_the_snapshot">
              What restores this installation is the snapshot, on the tab next door.
            </p>
          </div>
        </div>
        @if (r.literals.length) {
          <div class="note warn">
            <mat-icon>key_off</mat-icon>
            <div>
              <p i18n="@@Export_literals_note">
                These secrets are stored here as values rather than vault references, so they will
                NOT be in the file. Wherever it lands, the fields will be empty.
              </p>
              <ul>
                @for (l of r.literals; track l.holder + l.id + l.field) {
                  <li>
                    <!-- Where it LIVES, and a way to get there. "mail relay
                         password" names a value without saying which screen
                         holds it, which leaves the reader with a warning they
                         cannot act on - and acting on it is the whole reason
                         to show it before the download. -->
                    <button class="link" (click)="goTo(l)">{{ where(l) }}</button>
                    <span class="mono">{{ l.field }}</span>
                  </li>
                }
              </ul>
              <p class="aside" i18n="@@Literals_remedy">
                Put each one in the vault and reference it by $name: the file then carries the
                name, the value stays here, and the field is filled on the other side by whatever
                that vault holds.
              </p>
            </div>
          </div>
        }
        @if (r.refs.length) {
          <div class="note">
            <mat-icon>vpn_key</mat-icon>
            <div>
              <p i18n="@@Export_refs_note2">
                The file references these vault entries. On the other side, one that already
                exists is used as it stands; one that is missing is created EMPTY and reported,
                and whatever references it stays inert until someone fills it.
              </p>
              <mat-chip-set>
                @for (name of r.refs; track name) {
                  <mat-chip disableRipple>{{ name }}</mat-chip>
                }
              </mat-chip-set>
            </div>
          </div>
        }
        @if (hasImage()) {
          <div class="note">
            <mat-icon>image</mat-icon>
            <p i18n="@@Export_image_note">
              This configuration has a picture. A plain file is text and leaves it out - importing
              that file elsewhere keeps whatever picture is already in place. Take the package to
              carry the images with it.
            </p>
          </div>
        }
      }
    </mat-dialog-content>
    <!-- Short labels, and the sentence they abbreviate is in the note right
         above: three buttons wide enough to wrap turn a choice into a puzzle.
         What each one leaves out is on hover, not in its name. -->
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      @if (hasImage()) {
        <button
          matButton="tonal"
          (click)="close('yaml')"
          i18n-matTooltip="@@Plain_yaml_tip"
          matTooltip="Text only: the images stay behind, and an import keeps whatever is in place."
        >
          <mat-icon>description</mat-icon>
          <ng-container i18n="@@Plain_YAML">Plain YAML</ng-container>
        </button>
        <button
          matButton="filled"
          (click)="close('zip')"
          cdkFocusInitial
          i18n-matTooltip="@@Package_tip"
          matTooltip="A zip: the configuration reads and diffs, the images sit beside it as files."
        >
          <mat-icon>folder_zip</mat-icon>
          <ng-container i18n="@@Package">Package</ng-container>
        </button>
      } @else {
        <button matButton="filled" (click)="close('yaml')" cdkFocusInitial>
          <mat-icon>download</mat-icon>
          <ng-container i18n="@@Download">Download</ng-container>
        </button>
      }
    </mat-dialog-actions>
  `,
})
export class ExportDialogComponent {
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
  private readonly ref = inject(MatDialogRef<ExportDialogComponent, ExportChoice>);
  protected readonly data = inject<{ name: string; id?: string }>(MAT_DIALOG_DATA);

  protected readonly report = signal<ConfigReport | null>(null);
  protected readonly hasImage = computed(
    () => this.report()?.contents.some((c) => c.kind === 'image') ?? false,
  );

  constructor() {
    // The report is about what is RUNNING. A saved configuration is a copy of a
    // past state, so its literals and refs are the ones it was taken with -
    // close enough to warn about, and the file itself is the truth either way.
    this.api.configReport().subscribe({ next: (r) => this.report.set(r) });
  }

  protected close(format: 'yaml' | 'zip'): void {
    this.ref.close({ format });
  }

  // The screen that owns the value, in the console's own words rather than in
  // the exporter's: "mailrelay" is a section of a document, "Mail relay" is a
  // place someone can go.
  protected where(l: ConfigLiteral): string {
    if (l.holder === 'mailrelay') return $localize`:@@Mail_relay:Mail relay`;
    const authority = $localize`:@@Authentication:Authentication`;
    return l.label ? `${authority}, ${l.label}` : authority;
  }

  protected goTo(l: ConfigLiteral): void {
    this.ref.close();
    void this.router.navigate(
      l.holder === 'mailrelay'
        ? ['/infra/mail-relay']
        : ['/infra/auth-providers', ...(l.id ? [l.id] : [])],
    );
  }

  // "4 routes, 2 roles, the mail relay, 13 settings" - the nature of what is
  // inside and how much of it, on one line.
  protected carried(sections: ConfigSection[]): string {
    return sections
      .filter((s) => s.kind !== 'image')
      .map((s) =>
        s.kind === 'mailRelay'
          ? this.family(s.kind, s.count)
          : `${s.count} ${this.family(s.kind, s.count).toLocaleLowerCase()}`,
      )
      .join(', ');
  }

  protected family(kind: string, count: number): string {
    const one = count === 1;
    switch (kind) {
      case 'route':
        return one ? $localize`:@@Route:Route` : $localize`:@@Routes:Routes`;
      case 'role':
        return one ? $localize`:@@Role:Role` : $localize`:@@Roles:Roles`;
      case 'authProvider':
        return one ? $localize`:@@Authority:Authority` : $localize`:@@Authorities:Authorities`;
      case 'theme':
        return one ? $localize`:@@Theme:Theme` : $localize`:@@Themes:Themes`;
      case 'mailRelay':
        return $localize`:@@Mail_relay:Mail relay`;
      // Organisations and groups travel too (CFG-01), and they were reading as
      // "1 setting" through the fallback: an inventory that miscounts what it
      // carries is worse than no inventory.
      case 'tenant':
        return one ? $localize`:@@Organisation:Organisation` : $localize`:@@Organisations:Organisations`;
      case 'group':
        return one ? $localize`:@@Group:Group` : $localize`:@@Groups:Groups`;
      default:
        return one ? $localize`:@@Setting:Setting` : $localize`:@@Settings:Settings`;
    }
  }
}
