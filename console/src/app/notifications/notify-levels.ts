import { NotifyLevel } from "../api.service";

// Who turned a tenant's notifications off, for the tooltip or the line that
// says why a switch cannot be turned on (NOTIF-05).
export function offByText(level: NotifyLevel | undefined): string {
  switch (level) {
    case "app-admin":
      return $localize`:@@Notify_off_by_app_admin:Turned off by the application admin`;
    case "tenant-admin":
      return $localize`:@@Notify_off_by_tenant_admin:Turned off by the tenant admin`;
    default:
      return $localize`:@@Notify_off_by_root:Turned off by root`;
  }
}
