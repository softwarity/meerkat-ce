import { Component, computed, effect, inject, input, linkedSignal, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { forkJoin } from 'rxjs';
import { ApiService, BusinessAccess, Group } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { LiveChangesService } from '../../shared/live-changes.service';
import { BusinessAccessFormComponent } from '../business-access-form.component';
import { IdentityDrawer } from '../identity-drawer';

// One group of an organisation, in the right drawer of the page that hosts
// the groups or members matrix (opened by clicking the group's name): its
// name and description, when the organisation picks one group per session its
// working hours (TENANT-04), and its deletion. Its roles stay in the matrix.
@Component({
  selector: 'app-group-editor',
  imports: [
    MatButtonModule,
    MatDividerModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    LoadingIndicatorComponent,
    BusinessAccessFormComponent,
    EeLockComponent,
  ],
  templateUrl: './group-editor.component.html',
  styleUrl: '../user-editor/user-editor.component.scss',
})
export class GroupEditorComponent {
  readonly tenantId = input.required<string>();
  readonly groupId = input.required<string>();

  private readonly api = inject(ApiService);
  private readonly dialogs = inject(DialogsService);
  protected readonly drawer = inject(IdentityDrawer);

  protected readonly group = signal<Group | null>(null);
  // Exclusive mode, and the hours that apply without the group's own.
  protected readonly exclusive = signal(false);
  protected readonly inheritedHours = signal<BusinessAccess>({ inherited: false });

  protected readonly name = linkedSignal(() => this.group()?.name ?? '');
  protected readonly description = linkedSignal(() => this.group()?.description ?? '');
  protected readonly hours = linkedSignal<BusinessAccess>(() => this.group()?.businessAccess ?? { inherited: true });

  protected readonly dirty = computed(() => {
    const g = this.group();
    return (
      !!g &&
      (this.name().trim() !== g.name ||
        this.description().trim() !== (g.description ?? '') ||
        JSON.stringify(this.hours()) !== JSON.stringify(g.businessAccess ?? { inherited: true }))
    );
  });

  protected readonly saving = signal(false);
  protected readonly error = signal('');

  constructor() {
    effect(() => this.load(this.tenantId(), this.groupId()));
    effect(() => {
      const tenantId = this.tenantId();
      forkJoin([this.api.getTenant(tenantId), this.api.settings()]).subscribe(([t, s]) => {
        this.exclusive.set(t.groupMode === 'SINGLE');
        this.inheritedHours.set(
          t.businessAccess?.inherited === false ? t.businessAccess : (s.businessAccess ?? { inherited: false }),
        );
      });
    });
    // Somebody else's write, or this drawer's own save, read back.
    inject(LiveChangesService).on('group', () => this.load(this.tenantId(), this.groupId()));
  }

  private load(tenantId: string, groupId: string): void {
    this.api.listGroups(tenantId).subscribe((groups) => {
      const g = groups.find((x) => x.id === groupId);
      if (g) this.group.set(g);
      else this.drawer.close(); // deleted meanwhile: nothing left to show
    });
  }

  protected save(): void {
    const g = this.group();
    if (!g || !this.name().trim()) return;
    this.saving.set(true);
    this.error.set('');
    this.api
      .updateGroup({
        ...g,
        name: this.name().trim(),
        description: this.description().trim(),
        roles: g.roles ?? [],
        businessAccess: this.hours(),
      })
      .subscribe({
        next: (saved) => {
          this.saving.set(false);
          this.group.set(saved);
        },
        error: (e) => {
          this.saving.set(false);
          this.error.set(typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`);
        },
      });
  }

  // Confirmed, then gone: the matrices hear of it on the live channel, and the
  // drawer has nothing left to show.
  protected async remove(): Promise<void> {
    const g = this.group();
    if (!g) return;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_group_NAME:Delete group "${g.name}:NAME:"?`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteGroup(this.tenantId(), g.id).subscribe({
      next: () => this.drawer.close(),
      error: (e) =>
        this.error.set(typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`),
    });
  }
}
