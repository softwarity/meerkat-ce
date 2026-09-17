import { Component, inject, input, OnInit, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Subject, debounceTime, distinctUntilChanged, switchMap } from 'rxjs';
import { ApiService, BankIcon, Route } from '../api.service';
import { SvgIconComponent } from './svg-icon.component';

// One module as it is edited (PORTAL-01): which UI route it opens, and how it
// shows in the bar. `icon` is an SVG string; label and homeLabel override the
// route's name (homeLabel only for a parent, on its own "home" row).
export interface ModuleDraft {
  routeId: string;
  icon: string;
  label: string;
  homeLabel: string;
  description: string;
  disabled: boolean;
}

export interface ModuleFormData {
  title: string;
  module: ModuleDraft;
  routes: Route[];
  isParent: boolean;
  // Set when editing an EXISTING module (not a new one): enables the quick
  // actions (reorder, disable, add sub-module, delete). null while adding.
  existing: boolean;
  canUp: boolean;
  canDown: boolean;
}

// The module editor, shown in a DRAWER on the portal page (not a modal): the
// extra width lets the icon palette live inline - searched and filtered right
// here - and, when a hand-written SVG is wanted, the same area becomes a
// textarea. Emits the edited module, or closes.
@Component({
  selector: 'app-module-editor',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
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
  readonly removed = output<void>();

  private readonly api = inject(ApiService);

  protected readonly routeId = signal('');
  protected readonly icon = signal('');
  protected readonly label = signal('');
  protected readonly homeLabel = signal('');
  protected readonly description = signal('');
  protected readonly disabled = signal(false);

  protected readonly iconMode = signal<'search' | 'svg'>('search');
  protected readonly query = signal('');
  protected readonly results = signal<BankIcon[]>([]);

  private readonly q$ = new Subject<string>();

  protected readonly noRoutes = $localize`:@@Portal_no_ui_routes:No UI route exists yet - a module opens a UI route.`;

  ngOnInit(): void {
    // input() is not bound in field initializers - seed here.
    const m = this.data().module;
    this.routeId.set(m.routeId);
    this.icon.set(m.icon);
    this.label.set(m.label);
    this.homeLabel.set(m.homeLabel);
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

  protected save(): void {
    if (!this.routeId()) return;
    this.saved.emit({
      routeId: this.routeId(),
      icon: this.icon().trim(),
      label: this.label().trim(),
      homeLabel: this.homeLabel().trim(),
      description: this.description().trim(),
      disabled: this.disabled(),
    });
  }

  protected toggleDisabled(v: boolean): void {
    this.disabled.set(v);
    this.toggledDisabled.emit(v);
  }
}
