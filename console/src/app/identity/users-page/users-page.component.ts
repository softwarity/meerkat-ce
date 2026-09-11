import { Component, computed, inject, signal } from "@angular/core";
import { MatButtonModule } from "@angular/material/button";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatIconModule } from "@angular/material/icon";
import { MatInputModule } from "@angular/material/input";
import { MatSidenavModule } from "@angular/material/sidenav";
import { MatSnackBar } from "@angular/material/snack-bar";
import { MatTableModule } from "@angular/material/table";
import { MatTooltipModule } from "@angular/material/tooltip";
import { toSignal } from "@angular/core/rxjs-interop";
import { ActivatedRoute, Router } from "@angular/router";
import { LoadingIndicatorComponent } from "@softwarity/loading-indicator";
import { RowActionsDirective } from "@softwarity/row-actions";
import { forkJoin } from "rxjs";
import { ApiService, Settings, User } from "../../api.service";
import { MeService } from "../../me.service";
import { DialogsService } from "../../shared/dialogs.service";
import { FormFieldComponent } from "../../shared/form-field.component";
import { UserCreateComponent } from "../user-create.component";
import {
  UserEditorComponent,
  UserEditorView,
} from "../user-editor/user-editor.component";

// mfaText renders the resolved global second-factor policy - the label a user's
// "Inherited" resolves to (the user record sits directly under global).
function mfaText(required: boolean): string {
  return required
    ? $localize`:@@MFA_required:Required`
    : $localize`:@@MFA_optional:Optional`;
}

// Users administration - root scope. The table is a plain list; clicking a row
// opens the user's options in a right drawer (the same pattern as routes).
@Component({
  selector: "app-users-page",
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
    FormFieldComponent,
    UserCreateComponent,
    UserEditorComponent,
  ],
  // Escape gives back one level, like a click outside. Bound to the DOCUMENT
  // and not to the drawer container: turning a page inside the drawer removes
  // the button that had focus, the key then lands on <body>, and a handler on
  // the container would answer only sometimes - worse than never.
  host: { "(document:keydown.escape)": "onEscape()" },
  templateUrl: "./users-page.component.html",
  styleUrl: "./users-page.component.scss",
})
export class UsersPageComponent {
  // What the one drawer holds. A new account and an existing one are two
  // states of the same panel; `panel` says which of the account's three pages
  // is on screen, and lives here rather than in the editor because the click
  // outside is answered here.
  protected readonly creating = signal(false);
  protected readonly panel = signal<UserEditorView>("account");

  // One click, one level. Material's own backdrop handler shuts the drawer
  // whole, so the drawer refuses (disableClose) and the order is decided here:
  // the sub-page first, the drawer under it after.
  protected onBackdrop(): void {
    if (this.panel() !== "account") {
      this.panel.set("account");
      return;
    }
    if (this.creating()) {
      this.creating.set(false);
      return;
    }
    this.onClose();
  }

  // An open overlay - a dialog, a select's panel - owns the key first: it is
  // what the person is looking at, and closing the drawer under it would
  // answer a question they did not ask.
  protected onEscape(): void {
    if (document.querySelector(".cdk-overlay-container .cdk-overlay-pane")) return;
    if (!this.creating() && !this.editing()) return;
    this.onBackdrop();
  }

  private readonly api = inject(ApiService);
  private readonly me = inject(MeService);
  private readonly snack = inject(MatSnackBar);
  private readonly router = inject(Router);
  private readonly dialogs = inject(DialogsService);

  protected readonly loading = signal(true);
  protected readonly users = signal<User[]>([]);
  private readonly settings = signal<Settings | null>(null);
  protected readonly columns = ["identity", "summary"];

  // Free text over everything one knows a person by: the login one types, the
  // name one reads and the address one was given. Anything else (a capability,
  // a state) is a column with its own badge, and reads faster than a query.
  protected readonly query = signal("");
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLowerCase();
    if (!q) return this.users();
    return this.users().filter((u) =>
      [u.username, u.fullname, u.email].some((f) =>
        (f ?? "").toLowerCase().includes(q),
      ),
    );
  });

  // The URL owns the drawer (F5-proof): /users/:id edits that user, /users
  // closes it. The row is looked up in the loaded list.
  private readonly params = toSignal(inject(ActivatedRoute).paramMap);
  protected readonly editing = computed(() => {
    const id = this.params()?.get("id");
    return id ? (this.users().find((u) => u.id === id) ?? null) : null;
  });

  // Always on the account page: a drawer that reopened on the security page
  // somebody left three accounts ago would answer a question nobody asked.
  protected openUser(u: User): void {
    this.panel.set("account");
    void this.router.navigate(["/application/users", u.id]);
  }
  protected readonly globalMfaLabel = computed(() =>
    mfaText(!!this.settings()?.mfaRequired),
  );

  protected meId(): string {
    return this.me.user()?.id ?? "";
  }

  constructor() {
    this.load();
  }

  protected load(): void {
    this.loading.set(true);
    forkJoin({
      users: this.api.listUsers(),
      settings: this.api.settings(),
    }).subscribe({
      next: ({ users, settings }) => {
        this.users.set(users);
        this.settings.set(settings);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected create(): void {
    this.creating.set(true);
  }

  // Created: the drawer goes back to the table rather than straight to the new
  // account's settings - the one-time password is on screen in its dialog, and
  // sliding a second panel under it would ask somebody to read it in a hurry.
  protected onCreated(): void {
    this.creating.set(false);
    this.load();
  }

  // A field changed in the drawer: refresh the row. The drawer follows on its
  // own - `editing` reads the fresh user out of the list.
  protected onUserSaved(fresh: User): void {
    this.users.update((list) =>
      list.map((u) => (u.id === fresh.id ? fresh : u)),
    );
  }

  // Fired both by the editor's close button and by the drawer's own close
  // (backdrop, escape): act once, when a user is still open.
  protected onClose(): void {
    this.panel.set("account");
    if (!this.editing()) return;
    void this.router.navigate(["/application/users"]);
    this.load();
  }

  // The superpowers (RBAC-05), as clickable badges on the row. Filtered by
  // what this installation IS: a power over a notion the console never
  // mentions is a badge nobody can act on - see allCapabilities.
  protected readonly capabilities = computed(() =>
    this.allCapabilities.filter(
      (c) => c.key !== "tenantCreator" || this.me.multiTenant(),
    ),
  );

  private readonly allCapabilities = [
    {
      key: "root" as const,
      label: "root",
      tooltip: $localize`:@@Tooltip_root:Administers the whole gateway: routes, users, tenants, settings`,
    },
    {
      key: "infraAdmin" as const,
      label: $localize`:@@infra_admin:infra admin`,
      tooltip: $localize`:@@Tooltip_infra_admin:Administers the routing plane: routes and the built-in pages`,
    },
    {
      key: "appAdmin" as const,
      label: $localize`:@@app_admin:app admin`,
      tooltip: $localize`:@@Tooltip_app_admin:Administers the application identity: users, roles, settings`,
    },
    {
      key: "dev" as const,
      label: "dev",
      tooltip: $localize`:@@Tooltip_dev:Unlocks the developer tooling: dev keys, service substitution (plug)`,
    },
    // Single-organisation installations never show this one: there is one
    // organisation, nobody names it, and a second cannot be created - so the
    // power grants nothing and the badge would only invite a click that the
    // server refuses.
    {
      key: "tenantCreator" as const,
      label: $localize`:@@tenant_creator:tenant creator`,
      tooltip: $localize`:@@Tooltip_tenant_creator:May create tenants, and owns the tenants they create`,
    },
  ];

  protected toggleCapability(
    u: User,
    key:
      "root" | "dev" | "tenantCreator" | "infraAdmin" | "appAdmin",
    event: Event,
  ): void {
    event.stopPropagation(); // the row click opens the drawer - not this
    this.patchUser(u, { [key]: !u[key] });
  }

  // Enabling/disabling belongs to the row (like a route's pause/play), not to
  // the detail panel: it is a list-level decision, one click from the table.
  // Disabling locks someone out, so it asks first; enabling does not.
  protected async toggleEnabled(u: User, event: Event): Promise<void> {
    event.stopPropagation();
    if (u.enabled) {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Disable_user_USERNAME:Disable user "${u.username}:USERNAME:"?`,
        message: $localize`:@@Disable_user_message:They will be signed out and will not be able to sign in again until you enable them.`,
        confirmLabel: $localize`:@@Disable:Disable`,
        danger: true,
      });
      if (!ok) return;
    }
    this.patchUser(u, { enabled: !u.enabled });
  }

  private patchUser(u: User, patch: Partial<User>): void {
    this.api.updateUser({ ...u, ...patch }).subscribe({
      next: (fresh) =>
        this.users.update((list) =>
          list.map((x) => (x.id === fresh.id ? fresh : x)),
        ),
      error: (err) => {
        const e = err as { error?: { error?: string } };
        this.snack.open(
          typeof e?.error?.error === "string"
            ? e.error.error
            : $localize`:@@Request_failed:Request failed`,
          undefined,
          { duration: 4000 },
        );
      },
    });
  }
}
