import { NotificationReader, NotifyLevel } from "../api.service";

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

// Where one reader's mail stands, as the mail queue keeps it: accepted by the
// relay is the most the gateway ever knows, never "delivered".
export function mailText(r: NotificationReader): string {
  switch (r.mail) {
    case "pending":
      return $localize`:@@Notify_mail_pending:Mail to send`;
    case "sending":
      return $localize`:@@Notify_mail_sending:Mail being sent`;
    case "accepted":
      return $localize`:@@Notify_mail_accepted:Mail accepted by the relay`;
    case "failed":
      return r.mailCode
        ? $localize`:@@Notify_mail_failed_code:Mail refused (${r.mailCode}:CODE:)`
        : $localize`:@@Notify_mail_failed:Mail failed`;
    case "unknown":
      return $localize`:@@Notify_mail_unknown:Mail outcome unknown`;
    case "cancelled":
      return $localize`:@@Notify_mail_cancelled:Mail cancelled`;
    case "noaddress":
      return $localize`:@@Notify_mail_noaddress:No mail address`;
    default:
      return "";
  }
}
