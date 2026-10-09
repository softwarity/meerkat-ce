import { Component, inject, input, OnInit, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Subject, debounceTime, distinctUntilChanged, switchMap } from 'rxjs';
import { ApiService, BankIcon, Route } from '../api.service';
import { FileButtonComponent, FileDropDirective } from '../shared/file-pick';
import { SvgIconComponent } from './svg-icon.component';

// One entry as it is edited (PORTAL-01): a module - which UI route it opens -
// or a container, which has no route and holds modules; and how it shows in
// the bar. `icon` is an SVG string; a module's label overrides its route's
// name, a container's label is its only name.
export interface ModuleDraft {
  routeId: string;
  icon: string;
  label: string;
  description: string;
  disabled: boolean;
}

// What is edited: a top-level module, a container, or a module inside one.
export type EntryKind = 'module' | 'container' | 'sub';

export interface ModuleFormData {
  title: string;
  module: ModuleDraft;
  routes: Route[];
  kind: EntryKind;
  // Whether a BAR is being drawn. An icon is a thing a bar has; a flat menu
  // has a name and an order, so offering it there would be offering a setting
  // nothing reads.
  bar: boolean;
  // Set when editing an EXISTING module (not a new one): enables the quick
  // actions (reorder, disable, add sub-module, delete). null while adding.
  existing: boolean;
  canUp: boolean;
  canDown: boolean;
  // Whether the entry's surface runs across (a row of tabs) rather than down
  // (a rail, or the links list): the move arrows point that way.
  horizontal: boolean;
  // Where a module can be moved to: the containers, by their index and label
  // (not the one it is in). `under` is the container it is in now, null at the
  // top level. A container itself stays at the top level: the catalogue has
  // two levels.
  parents: { index: number; name: string }[];
  under: number | null;
}

// The module editor, shown in a DRAWER on the portal page (not a modal): the
// extra width lets the icon palette live inline - searched and filtered right
// here - and, when a hand-written SVG is wanted, the same area becomes a
// textarea. Emits the edited module, or closes.
@Component({
  selector: 'app-module-editor',
  imports: [
    FileButtonComponent,
    FileDropDirective,
    MatButtonModule,
    MatButtonToggleModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
    MatMenuModule,
    MatSelectModule,
    MatSlideToggleModule,
    MatTooltipModule,
    SvgIconComponent,
  ],
  templateUrl: './module-editor.component.html',
  styleUrl: './module-editor.component.scss',
})
export class ModuleEditorComponent implements OnInit {
  readonly data = input.required<ModuleFormData>();
  readonly saved = output<ModuleDraft>();
  readonly closed = output<void>();
  // Quick actions on an existing module, applied at once by the page.
  readonly moved = output<-1 | 1>();
  readonly toggledDisabled = output<boolean>();
  readonly addedSub = output<void>();
  // Moved under another module (its index), or detached to the top level.
  readonly relocated = output<number | null>();
  readonly removed = output<void>();

  private readonly api = inject(ApiService);

  protected readonly routeId = signal('');
  protected readonly icon = signal('');
  protected readonly label = signal('');
  protected readonly description = signal('');
  protected readonly disabled = signal(false);

  protected readonly iconMode = signal<'search' | 'svg'>('search');
  protected readonly query = signal('');
  protected readonly results = signal<BankIcon[]>([]);

  private readonly q$ = new Subject<string>();

  protected readonly noRoutes = $localize`:@@Portal_no_ui_routes:No UI route yet: a module opens one.`;

  ngOnInit(): void {
    // input() is not bound in field initializers - seed here.
    const m = this.data().module;
    this.routeId.set(m.routeId);
    this.icon.set(m.icon);
    this.label.set(m.label);
    this.description.set(m.description);
    this.disabled.set(m.disabled);

    this.q$
      .pipe(
        debounceTime(180),
        distinctUntilChanged(),
        switchMap((q) => this.api.searchIcons(q)),
      )
      .subscribe((r) => this.results.set(r));
    this.api.searchIcons('').subscribe((r) => this.results.set(r));
  }

  protected routeName(): string {
    return this.data().routes.find((r) => r.id === this.routeId())?.name ?? '';
  }

  protected onQuery(v: string): void {
    this.query.set(v);
    this.q$.next(v);
  }

  protected setMode(v: 'search' | 'svg'): void {
    if (v) this.iconMode.set(v);
  }

  protected pick(svg: string): void {
    this.icon.set(svg);
  }

  // An SVG file, chosen or dropped, lands in the paste box as if pasted: the
  // same text, read the same way when saved.
  protected readonly svgError = signal('');
  protected async loadSvg(file: File | undefined): Promise<void> {
    this.svgError.set('');
    if (!file) return;
    const text = (await file.text()).trim();
    if (!/<svg[\s>]/i.test(text)) {
      this.svgError.set($localize`:@@Portal_not_svg:${file.name}:NAME: is not an SVG file.`);
      return;
    }
    this.icon.set(text);
  }

  // A module needs its route, a container its label: neither has anything
  // else to be found by.
  protected canSave(): boolean {
    return this.data().kind === 'container' ? !!this.label().trim() : !!this.routeId();
  }

  protected save(): void {
    if (!this.canSave()) return;
    this.saved.emit({
      routeId: this.data().kind === 'container' ? '' : this.routeId(),
      icon: this.icon().trim(),
      label: this.label().trim(),
      description: this.description().trim(),
      disabled: this.disabled(),
    });
  }

  protected readonly moveUp = $localize`:@@Move_up:Move up`;
  protected readonly moveDown = $localize`:@@Move_down:Move down`;
  protected readonly moveLeft = $localize`:@@Move_left:Move left`;
  protected readonly moveRight = $localize`:@@Move_right:Move right`;
  protected readonly noContainer = $localize`:@@Portal_no_container:No container yet: add one first`;
  protected readonly labelHint = $localize`:@@Portal_module_label_hint:Empty uses the route's name.`;
  protected readonly containerLabelHint = $localize`:@@Portal_container_label_hint:Required: a container has no route to name it.`;



  protected toggleDisabled(v: boolean): void {
    this.disabled.set(v);
    this.toggledDisabled.emit(v);
  }
}
