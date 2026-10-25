import { Component, computed } from "@angular/core";
import { httpResource } from "@angular/common/http";
import { MatExpansionModule } from "@angular/material/expansion";
import { Edition } from "../api.service";
import { SnippetComponent } from "../shared/snippet.component";

// How a backend service tells its users something (NOTIF-05), in the drawer
// of the screen that lists what was sent.
//
// The scheduler's explanation, asked of this API: nothing on this screen
// PUBLISHES - the services do, with a credential set under Access - so an
// operator arriving with "how does my service send one of these" needs the
// call it makes, written down where they are looking. Same shape, same look
// (the scheduler's stylesheet), same words for the same things.
@Component({
  selector: "app-notifications-api",
  imports: [MatExpansionModule, SnippetComponent],
  templateUrl: "./notifications-api.component.html",
  styleUrl: "../scheduler/scheduler-api.component.scss",
})
export class NotificationsApiComponent {
  // The internal plane, by the name a service reaches it under from INSIDE
  // the cluster, as the scheduler's panel names it.
  private readonly editionRes = httpResource<Edition>(() => "/api/edition");
  protected readonly origin = computed(() => {
    const e = this.editionRes.value();
    return e?.internalOrigin || `http://meerkat:${e?.internalPort || "7070"}`;
  });

  protected readonly publishCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Publish: 201, its id and how many people received it.
# notification.json is the message below, without its comments.
curl -X POST ${this.origin()}/api/notifications -H "$AUTH" \\
  -H "Content-Type: application/json" --data @notification.json
# {"id":"ntf_b5605a88...","recipients":3}

# The same key again while it is live: 200, the same notification,
# nobody is told twice.
# {"id":"ntf_b5605a88...","recipients":3,"existing":true}`,
  );

  // Annotated, and named .jsonc for it: the values a field takes belong
  // beside the field.
  protected readonly payload = `{
  "tenant": "acme",               // tenant id or name, optional when
                                  // there is only one. "*": a copy
                                  // to every tenant
  "roles": ["station_ops"],       // only the members holding one of
  "users": ["alice"],             // these roles, or these members.
                                  // Neither: every member

  "title": "Station 42 is down",  // required, plain text
  "body": "No reading since 08:10. The on-call team was paged.",
  "level": "error",               // info (default), success, warning,
                                  // error

  "detail": {                     // where it leads: a screen of a
    "service": "station-ui",      // service, named as in the cluster,
    "path": "/stations/42"        // and the path as that service sees it
  },
  "actions": [                    // buttons, five at most
    {
      "label": "Restart",
      "service": "stations-api",
      "path": "/stations/42/restart",
      "method": "POST",           // GET by default
      "body": { "reason": "notification" }
                                  // JSON, not with GET. The reader
                                  // can see it: no secret
    },
    { "label": "Runbook", "url": "https://wiki.example.com/stations" }
  ],

  "mail": true,                   // also by mail (needs a mail relay)
  "expires": "P7D",               // a duration or a date: 30 days by
                                  // default, 90 at most
  "key": "station-42-down",       // sent again while live: the same
                                  // notification, nobody told twice
  "meta": { "station": "42" },    // your own labels, to find and
                                  // withdraw it. Never shown

  "ack": "single",                // ask the readers to acknowledge it:
                                  // none (default), single, all
  "onAck": {                      // your service, called at each
                                  // acknowledgement
    "service": "stations-api",
    "path": "/hooks/ack"
  }
}`;

  protected readonly everyCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

curl -X POST ${this.origin()}/api/notifications -H "$AUTH" \\
  -H "Content-Type: application/json" \\
  -d '{"tenant":"*","title":"Maintenance tonight at 22:00","key":"maint-1025"}'
# 201: one copy per tenant, each with its own id, and the tenants left out.
# {"recipients":41,
#  "tenants":[
#    {"tenant":"038ae636...","name":"Acme","id":"ntf_1c8e...","recipients":38},
#    {"tenant":"f5a445af...","name":"Globex","id":"ntf_2687...","recipients":3}],
#  "skipped":[
#    {"tenant":"9c1d07e2...","name":"Initech",
#     "reason":"notifications are off, turned off by the tenant admin"}]}`,
  );

  protected readonly readCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Your live notifications, newest first, filtered by metadata.
curl "${this.origin()}/api/notifications?meta.station=42" -H "$AUTH"
curl "${this.origin()}/api/notifications?meta.station=~^4[0-9]$" -H "$AUTH"
# [{"id":"ntf_b5605a88...","tenantId":"038ae636...","sender":"stations",
#   "title":{"":"Station 42 is down"},"meta":{"station":"42"},"recipients":3,
#   "ack":"single","ackState":"pending","acknowledged":0, ...}]

# Withdraw one: 204. It leaves every reader's page at once.
curl -X DELETE ${this.origin()}/api/notifications/ntf_b5605a88 -H "$AUTH"

# Withdraw every one the filter matches; one meta condition at least.
curl -X DELETE "${this.origin()}/api/notifications?meta.station=42" -H "$AUTH"
# {"withdrawn":["ntf_1c8e...","ntf_2687..."]}`,
  );

  protected readonly acksCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Who acknowledged it, when, and who has not yet.
curl ${this.origin()}/api/notifications/ntf_b5605a88/acks -H "$AUTH"
# {"id":"ntf_b5605a88...","ack":"single","ackState":"done","acknowledged":1,
#  "ackBy":{"id":"eb7f08a2...","username":"bob","name":"Bob Durand"},
#  "acks":[{"id":"eb7f08a2...","username":"bob","name":"Bob Durand",
#           "at":1792945051}],
#  "missing":[], ...}

# 409 when it does not ask for an acknowledgement.`,
  );

  protected readonly refused = `HTTP/1.1 422 Unprocessable Entity

{"error":"level \\"urgent\\": allowed are info, success, warning, error"}`;

  protected readonly incoming = `POST /hooks/ack HTTP/1.1
Content-Type: application/json
X-Remote-User: meerkat                 # the caller, with no role, passed
                                       # the way your route passes identity
Meerkat-Notification: ntf_b5605a88...  # the notification
Meerkat-Attempt: 1                     # 2 or 3: sent again after a failure
User-Agent: meerkat-notifications

{"notification":"ntf_b5605a88...","tenant":"038ae636...","tenantName":"Acme",
 "key":"station-42-down","meta":{"station":"42"},"ack":"single",
 "by":{"id":"eb7f08a2...","username":"bob","name":"Bob Durand"},
 "at":"2026-10-25T16:17:31Z","acknowledged":1,"recipients":3,"complete":true}`;
}
