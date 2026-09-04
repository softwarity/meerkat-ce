import { Component, booleanAttribute, computed, input, model } from '@angular/core';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';

const SCHEME = /^([a-zA-Z][a-zA-Z0-9+.-]*):\/\/(.*)$/;

// One host the caller knows about, offered inside the field.
export interface UrlSuggestion {
  /** The host part exactly as it goes into the field: "service:8080". */
  value: string;
  /** A word beside it - a state, a source. Never part of the value. */
  hint?: string;
  /** Offered, but doubtful: rendered quieter. Order is the caller's. */
  muted?: boolean;
}

// A URL field: a scheme select, "://", and the host/path, in ONE form field.
//
//   <app-url-input i18n-label="@@Upstream" label="Upstream"
//     [protocols]="['http', 'https']" [suggestions]="services()"
//     [value]="url()" (valueChange)="save($event)" />
//
// It owns its <mat-form-field> and composes standard Material inside it - a
// mat-select as a prefix, a matInput, a mat-autocomplete - the same shape as
// app-form-field. It used to BE a MatFormFieldControl instead, wrapping a bare
// <input> the parent's form field adopted: sixty lines reimplementing what
// Material already does (focus, float, describedBy) to gain nothing.
//
// The bound value is a whole URL ("https://svc:8080"); pasting one into the
// host splits its scheme off, and a suggestion carries the host alone so the
// scheme already chosen stands.
@Component({
  selector: 'app-url-input',
  imports: [MatAutocompleteModule, MatFormFieldModule, MatInputModule, MatSelectModule],
  styleUrl: './url-input.component.scss',
  templateUrl: './url-input.component.html',
})
export class UrlInputComponent {
  readonly value = model('');
  readonly label = input('');
  // Material draws the asterisk itself from the control's own required flag,
  // so a mandatory field says so where it is rather than only in a list of
  // what is missing.
  readonly required = input(false, { transform: booleanAttribute });
  readonly hint = input('');
  readonly hintAlign = input<'start' | 'end'>('start');
  // The schemes this field accepts, in preference order: an upstream is
  // http(s), a directory ldap(s). The first one is what a fresh field uses.
  readonly protocols = input<string[]>(['https', 'http']);
  // What the host part suggests once the label floats. It belongs to the
  // caller: "service:8080/base" means nothing on a directory.
  readonly hostPlaceholder = input('');
  // Hosts the caller knows exist, offered INSIDE the field.
  //
  // Beside it, as a second control, they read as a second value: the operator
  // sees an empty box that does not announce it holds a list, and types the
  // url from memory anyway - which is the mistake the list exists to remove.
  // Here, focusing the field IS asking the question.
  readonly suggestions = input<UrlSuggestion[]>([]);

  protected readonly protocol = computed(
    () => split(this.value()).proto ?? this.protocols()[0] ?? 'https',
  );
  protected readonly rest = computed(() => split(this.value()).rest);
  // Keep an out-of-catalogue stored scheme selectable.
  protected readonly protoOptions = computed(() => {
    const list = this.protocols();
    const cur = this.protocol();
    return list.includes(cur) ? list : [cur, ...list];
  });
  // Typing filters. Nothing matching means an upstream outside the catalogue,
  // which is a legitimate thing to type, so the list simply steps aside.
  protected readonly offered = computed(() => {
    const typed = this.rest().trim().toLowerCase();
    const all = this.suggestions();
    return typed ? all.filter((s) => s.value.toLowerCase().includes(typed)) : all;
  });

  protected setProtocol(p: string): void {
    this.commit(p, this.rest());
  }
  protected onInput(raw: string): void {
    const s = split(raw);
    if (s.proto) {
      this.commit(s.proto, s.rest);
    } else {
      this.commit(this.protocol(), raw);
    }
  }
  // A suggestion is a host, so the scheme already chosen stands.
  protected pick(host: string): void {
    this.commit(this.protocol(), host);
  }

  private commit(proto: string, rest: string): void {
    this.value.set(rest ? `${proto}://${rest}` : '');
  }
}

function split(v: string): { proto: string | null; rest: string } {
  const m = SCHEME.exec(v ?? '');
  return m ? { proto: m[1].toLowerCase(), rest: m[2] } : { proto: null, rest: v ?? '' };
}
