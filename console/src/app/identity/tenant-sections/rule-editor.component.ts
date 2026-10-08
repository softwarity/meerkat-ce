import { Component, computed, input, linkedSignal, output } from '@angular/core';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { AuthProvider, Group, GroupRule } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';

// One group rule, in the right drawer of the rules screen, written as a
// sentence. The two halves stay apart in the reader's head: what has to be
// true upstream, and what it grants here.
@Component({
  selector: 'app-rule-editor',
  imports: [
    MatAutocompleteModule,
    MatButtonModule,
    MatDividerModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    FormFieldComponent,
  ],
  styleUrl: './rule-editor.component.scss',
  templateUrl: './rule-editor.component.html',
})
export class RuleEditorComponent {
  // null creates one.
  readonly rule = input<GroupRule | null>(null);
  readonly tenantName = input('');
  readonly groups = input<Group[]>([]);
  readonly authorities = input<AuthProvider[]>([]);
  // The group names the authorities have actually been heard to say.
  readonly reported = input<string[]>([]);

  readonly saved = output<Partial<GroupRule>>();
  readonly deleted = output<GroupRule>();
  readonly closed = output<void>();

  // Re-read whenever another rule is opened in the same drawer.
  protected readonly providerId = linkedSignal(() => this.rule()?.providerId ?? '');
  protected readonly external = linkedSignal(() => this.rule()?.external ?? '');
  protected readonly groupId = linkedSignal(() => this.rule()?.groupId ?? '');

  // What the field offers as you type: the names really reported, filtered.
  protected readonly suggestions = computed(() => {
    const typed = this.external().trim().toLowerCase();
    const all = this.reported();
    return typed ? all.filter((n) => n.toLowerCase().includes(typed)) : all;
  });

  // A rule matching every authority AND every group would admit anyone the
  // gateway ever authenticates, so the server refuses it. Say so here rather
  // than letting someone hit Save to find out.
  protected readonly canSave = computed(
    () => this.providerId().trim() !== '' || this.external().trim() !== '',
  );

  protected save(): void {
    this.saved.emit({
      providerId: this.providerId().trim(),
      external: this.external().trim(),
      groupId: this.groupId(),
    });
  }
}
