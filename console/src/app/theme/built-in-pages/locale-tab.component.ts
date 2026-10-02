import { Component, computed, effect, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatMenuModule } from '@angular/material/menu';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, LocaleString } from '../../api.service';
import { BuiltInPagesScope } from './built-in-pages.scope';

// The Locale tab: the strings of the page on the right, in the language on the
// left (I18N-05).
//
// Beside the page, and that is the whole idea. Nobody translates three hundred
// keys from a list well - ours has fourteen holes in nineteen languages, which
// is what happens when you try. Five strings with the screen that shows them
// is a different task.
//
// Each row carries the English wording as its placeholder, because a key name
// is not a sentence, and the shipped wording under it once it has been changed
// - so putting one back is a decision made with both in view.
// What each kind is called on screen. The keys come from the server; a kind it
// does not know shows its own name rather than nothing, so a new one added over
// there is visible here the day it appears instead of silently untitled.
const GROUP_LABELS: Record<string, string> = {
  title: $localize`:@@Locale_group_title:Titles`,
  message: $localize`:@@Locale_group_message:Messages`,
  label: $localize`:@@Locale_group_label:Labels and buttons`,
  hint: $localize`:@@Locale_group_hint:Help`,
  error: $localize`:@@Locale_group_error:Errors`,
};

@Component({
  selector: 'app-locale-tab',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatMenuModule,
    MatTooltipModule,
  ],
  templateUrl: './locale-tab.component.html',
  styleUrl: './locale-tab.component.scss',
})
export class LocaleTabComponent {
  protected readonly scope = inject(BuiltInPagesScope);
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  protected readonly strings = signal<LocaleString[]>([]);
  protected readonly loading = signal(false);
  protected readonly saving = signal(false);
  // Key to the value on screen, for the rows that were touched. Only these are
  // sent: a screen saves what it edited, not the whole catalogue.
  protected readonly edits = signal<Record<string, string>>({});
  protected readonly dirty = computed(() => Object.keys(this.edits()).length > 0);
  protected readonly holes = computed(() => this.strings().filter((s) => s.missing).length);

  // The strings cut into runs of one kind. The ORDER is the server's - the
  // export and the screen would otherwise answer differently about the same
  // catalogue - so this only walks the list and marks where a run changes.
  protected readonly sections = computed(() => {
    const out: { group: string; label: string; items: LocaleString[] }[] = [];
    for (const s of this.strings()) {
      const last = out[out.length - 1];
      if (last && last.group === s.group) last.items.push(s);
      else out.push({ group: s.group, label: GROUP_LABELS[s.group] ?? s.group, items: [s] });
    }
    return out;
  });
  // A language added here, as opposed to one the binary ships. It changes what
  // a reset does: there is nothing underneath to come back to, so it removes
  // the language.
  protected readonly added = computed(() => {
    const code = this.scope.previewLocale();
    const l = this.scope.locales().find((x) => x.code === code);
    return !!l && !l.embedded;
  });

  constructor() {
    // The strings follow the two things above them: the language and the page.
    effect(() => {
      const code = this.scope.previewLocale();
      const tpl = this.scope.template();
      if (!code || !tpl) return;
      this.loading.set(true);
      this.edits.set({});
      this.api.localeStrings(code, tpl).subscribe({
        next: (r) => {
          this.strings.set(r.strings);
          this.loading.set(false);
        },
        error: () => this.loading.set(false),
      });
    });
  }

  // Why the number is there, said in full: the count alone reads as a score.
  protected sharedTip(s: LocaleString): string {
    return $localize`:@@Locale_shared_tip:Shared by ${s.screens}:n: screens: a change here applies to all of them.`;
  }

  protected valueOf(s: LocaleString): string {
    const e = this.edits();
    return s.key in e ? e[s.key] : s.value;
  }

  protected edit(key: string, value: string): void {
    this.edits.update((e) => ({ ...e, [key]: value }));
  }

  // Put one string back to what the product ships. An empty value is how the
  // server is told to drop the override, so that is what is sent.
  protected revert(s: LocaleString): void {
    this.edit(s.key, '');
  }

  protected save(): void {
    const code = this.scope.previewLocale();
    if (!code || !this.dirty()) return;
    this.saving.set(true);
    this.api.saveLocale(code, this.edits(), { template: this.scope.template() }).subscribe({
      next: (r) => {
        this.strings.set(r.strings);
        this.edits.set({});
        this.saving.set(false);
        // The pages behind are served by the gateway, not by this frame: they
        // have to be asked again to speak the new wordings.
        this.scope.version.update((v) => v + 1);
        this.scope.reloadLocales();
      },
      error: (e: { error?: { error?: string } }) => {
        this.saving.set(false);
        this.snack.open(
          e.error?.error ?? $localize`:@@Locale_save_failed:The wordings could not be saved.`,
          undefined,
          { duration: 6000 },
        );
      },
    });
  }

  // Everything this installation changed in this language, back to what we
  // ship - or, for a language added here, the language itself. Confirmed,
  // because it is not one string but all of them.
  protected resetAll(): void {
    const code = this.scope.previewLocale();
    if (!code) return;
    // Read before the call: the list reloads underneath, and "was this language
    // added here" has to be the answer from before it disappeared.
    const wasAdded = this.added();
    if (wasAdded && !confirm(this.deleteWarning(code))) return;
    this.api.resetLocale(code).subscribe({
      next: () => {
        this.edits.set({});
        this.scope.version.update((v) => v + 1);
        this.scope.reloadLocales();
        // A language that was ADDED is gone now: the screen cannot keep showing
        // it, so it falls back to English the way the pages do.
        if (wasAdded) {
          this.scope.previewLocale.set('en');
          return;
        }
        this.api.localeStrings(code, this.scope.template()).subscribe({
          next: (r) => this.strings.set(r.strings),
        });
      },
      error: (e: { error?: { error?: string } }) =>
        this.snack.open(e.error?.error ?? 'reset failed', undefined, { duration: 6000 }),
    });
  }

  // Export what this language holds: this screen's strings, or the lot. A file
  // is the unit somebody sends to a translator, so it carries the wordings as
  // they render - not only the overrides, which would be unreadable alone.
  protected export(all: boolean): void {
    const code = this.scope.previewLocale();
    this.api.localeStrings(code, all ? undefined : this.scope.template()).subscribe({
      next: (r) => {
        const out: Record<string, string> = {};
        for (const s of r.strings) out[s.key] = s.value;
        const name = all ? `meerkat-${code}.json` : `meerkat-${code}-${r.scope}.json`;
        this.download(name, JSON.stringify({ code, scope: r.scope, strings: out }, null, 2));
      },
    });
  }

  private download(name: string, body: string): void {
    const url = URL.createObjectURL(new Blob([body], { type: 'application/json' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.click();
    URL.revokeObjectURL(url);
  }

  // Import a file. It replaces this language's layer WHOLE when it carries the
  // whole catalogue: a file is the complete picture of what it says, and
  // merging it would leave behind entries it deliberately dropped.
  protected import(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) return;
    file.text().then((text) => {
      let parsed: { code?: string; scope?: string; strings?: Record<string, string> };
      try {
        parsed = JSON.parse(text);
      } catch {
        this.snack.open(
          $localize`:@@Locale_import_not_json:That file is not JSON.`,
          undefined,
          { duration: 6000 },
        );
        return;
      }
      const entries = parsed.strings ?? {};
      const code = parsed.code || this.scope.previewLocale();
      const whole = parsed.scope === 'all';
      if (!Object.keys(entries).length) {
        this.snack.open(
          $localize`:@@Locale_import_empty:That file carries no strings.`,
          undefined,
          { duration: 6000 },
        );
        return;
      }
      if (!confirm(this.importWarning(code, Object.keys(entries).length, whole))) return;
      this.api.saveLocale(code, entries, { full: whole, template: this.scope.template() }).subscribe({
        next: (r) => {
          this.strings.set(r.strings);
          this.edits.set({});
          this.scope.version.update((v) => v + 1);
          this.scope.reloadLocales();
        },
        error: (e: { error?: { error?: string } }) =>
          this.snack.open(e.error?.error ?? 'import failed', undefined, { duration: 8000 }),
      });
    });
  }

  private deleteWarning(code: string): string {
    return $localize`:@@Locale_delete_confirm:Delete ${code}:code: and everything written in it?`;
  }

  private importWarning(code: string, n: number, whole: boolean): string {
    return whole
      ? $localize`:@@Locale_import_whole:Replace everything ${code}:code: holds with the ${n}:n: strings in this file?`
      : $localize`:@@Locale_import_partial:Apply ${n}:n: strings to ${code}:code:?`;
  }
}
