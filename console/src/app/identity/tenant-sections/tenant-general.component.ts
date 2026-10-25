import { Component, computed, effect, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, BusinessAccess, Settings } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { BusinessAccessFormComponent } from '../business-access-form.component';
import { UserFieldsFormComponent } from '../user-fields-form.component';
import { MeService } from '../../me.service';
import { TenantScope } from '../tenant-scope';
import { TenantsService } from '../../shared/tenants.service';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';

// The tenant's General section (a child route of the tenant layout): identity,
// the organisation's own fields (MODEL-03) and working hours, committed
// together with Save. The layout owns the tenant
// signal - a save pushes the fresh copy back so the left nav's name follows.
@Component({
  selector: 'app-tenant-general',
  imports: [
    EeLockComponent,
    ZonedDatePipe,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    FormFieldComponent,
    BusinessAccessFormComponent,
    UserFieldsFormComponent,
  ],
  styleUrl: './tenant-general.component.scss',
  templateUrl: './tenant-general.component.html',
})
export class TenantGeneralComponent {
  private readonly api = inject(ApiService);
  private readonly tenants = inject(TenantsService);
  private readonly snack = inject(MatSnackBar);
  protected readonly scope = inject(TenantScope);
  // Root and the app-admins write an organisation's fields; its own
  // administrators read them - the services behind the gateway trust them.
  private readonly me = inject(MeService);
  protected readonly zone = this.me.timezone;
  protected readonly fieldsReadonly = computed(() => !this.me.isAppAdmin());

  protected readonly saving = signal(false);
  private readonly settings = signal<Settings | null>(null);

  // Editable copies, re-seeded whenever the layout loads another tenant.
  protected readonly name = signal('');
  protected readonly enabled = signal(true);
  protected readonly description = signal('');
  protected readonly businessAccess = signal<BusinessAccess>({ inherited: true });
  protected readonly fields = signal<Record<string, string>>({});
  // '' in the store means "default cumulative"; the select surfaces it as MULTIPLE.
  // Not edited here any more - the group mode moved to the Groups screen,
  // which is what it describes. It is still carried so a save round-trips the
  // whole organisation instead of blanking it.
  protected readonly groupMode = signal('MULTIPLE');

  protected readonly globalBusinessAccess = computed<BusinessAccess>(
    () => this.settings()?.businessAccess ?? { inherited: false },
  );

  protected readonly dirty = computed(() => {
    const t = this.scope.tenant();
    if (!t) return false;
    return (
      this.name().trim() !== t.name ||
      this.enabled() !== t.enabled ||
      this.description().trim() !== t.description ||
      this.groupMode() !== (t.groupMode || 'MULTIPLE') ||
      JSON.stringify(this.businessAccess()) !== JSON.stringify(t.businessAccess) ||
      JSON.stringify(sorted(this.fields())) !== JSON.stringify(sorted(t.fields ?? {}))
    );
  });

  constructor() {
    effect(() => {
      const t = this.scope.tenant();
      if (!t) return;
      this.name.set(t.name);
      this.enabled.set(t.enabled);
      this.description.set(t.description);
      this.groupMode.set(t.groupMode || 'MULTIPLE');
      this.businessAccess.set(t.businessAccess);
      this.fields.set(t.fields ?? {});
    });
    this.api.settings().subscribe({ next: (s) => this.settings.set(s) });
  }

  protected save(): void {
    const t = this.scope.tenant();
    if (!t) return;
    this.saving.set(true);
    this.api
      .updateTenant({
        ...t,
        name: this.name().trim(),
        enabled: this.enabled(),
        description: this.description().trim(),
        groupMode: this.groupMode(),
        businessAccess: this.businessAccess(),
        fields: this.fields(),
      })
      .subscribe({
        next: (saved) => {
          this.saving.set(false);
          this.scope.tenant.set(saved);
          // And the rail's drawer, which shows the NAME: a stale one there is
          // worse than a stale entry, because nothing looks broken.
          this.tenants.replace(saved);
          this.snack.open($localize`:@@Tenant_NAME_saved:Tenant "${saved.name}:NAME:" saved`, undefined, { duration: 2500 });
        },
        error: (err) => {
          this.saving.set(false);
          const e = err as { error?: { error?: string } };
          this.snack.open(
            typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
            undefined,
            { duration: 4000 },
          );
        },
      });
  }
}

// Key order is not a change: the form rebuilds the object it edits.
function sorted(m: Record<string, string>): [string, string][] {
  return Object.entries(m).sort(([a], [b]) => a.localeCompare(b));
}
