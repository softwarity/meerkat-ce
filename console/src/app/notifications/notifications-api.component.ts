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
# {"id":"5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0","recipients":3}

# The same id again, before it expires: 200, the same notification,
# nobody is told twice. "state" says when it was withdrawn or replaced.
# Without an id: 422, it is required.
# {"id":"5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0","recipients":3,"existing":true}`,
  );

  // Annotated, and named .jsonc for it: the values a field takes belong
  // beside the field.
  protected readonly payload = `{
  "id": "5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0",
                                  // required, a UUID of your choice:
                                  // retry, replace, withdraw, follow
                                  // acks and mails
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
  "exercise": false,              // true: sent to practise, labelled
                                  // EXERCISE wherever it shows

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
  "replaces": "4d1e9a77-0b3c-4f0e-8d21-6a1b9c3e5f42",
                                  // an earlier one of yours, by id:
                                  // it ends as replaced
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
  -d '{"id":"maint-1025","tenant":"*","title":"Maintenance tonight at 22:00"}'
# 201: one copy per tenant under the one id, and the tenants left out.
# {"id":"maint-1025","recipients":41,
#  "tenants":[
#    {"tenant":"038ae636...","name":"Acme","recipients":38},
#    {"tenant":"f5a445af...","name":"Globex","recipients":3}],
#  "skipped":[
#    {"tenant":"9c1d07e2...","name":"Initech",
#     "reason":"notifications are off, turned off by the tenant admin"}]}`,
  );

  protected readonly readCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Your live notifications, newest first, filtered by metadata.
curl "${this.origin()}/api/notifications?meta.station=42" -H "$AUTH"
curl "${this.origin()}/api/notifications?meta.station=~^4[0-9]$" -H "$AUTH"
# [{"id":"5f0c2b8e-...","tenantId":"038ae636...","sender":"stations",
#   "title":{"":"Station 42 is down"},"meta":{"station":"42"},"recipients":3,
#   "ack":"single","ackState":"pending","acknowledged":0, ...}]

# Withdraw one by its id.
curl -X DELETE ${this.origin()}/api/notifications/5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0 -H "$AUTH"
# {"withdrawn":["5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0"]}

# Withdraw every one the filter matches; one meta condition at least.
curl -X DELETE "${this.origin()}/api/notifications?meta.station=42" -H "$AUTH"
# {"withdrawn":["5f0c2b8e-...","9b2d7c41-..."]}`,
  );

  protected readonly replaceCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Replace: publish the new one, naming the earlier one by its id.
curl -X POST ${this.origin()}/api/notifications -H "$AUTH" \\
  -H "Content-Type: application/json" \\
  -d '{"id":"job-7-80","title":"Job 7 at 80%","replaces":"job-7-40"}'
# {"id":"job-7-80","recipients":3,"replaced":["job-7-40"]}

# Withdraw: 200 and the id it ended now; "already" when it had ended.
curl -X DELETE ${this.origin()}/api/notifications/job-7-80 -H "$AUTH"
# {"withdrawn":["job-7-80"]}

# An id not received yet: 202, kept for 30 days, or what expires says.
curl -X DELETE "${this.origin()}/api/notifications/job-8?expires=P7D" -H "$AUTH"
# {"withdrawn":[],"pending":"job-8"}

# 400: an id with a character it may not carry.`,
  );

  protected readonly acksCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Who acknowledged it, when, and who has not yet.
curl ${this.origin()}/api/notifications/5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0/acks -H "$AUTH"
# {"id":"5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0","ack":"single","ackState":"done","acknowledged":1,
#  "ackBy":{"id":"eb7f08a2...","username":"bob","name":"Bob Durand"},
#  "acks":[{"id":"eb7f08a2...","username":"bob","name":"Bob Durand",
#           "at":1792945051}],
#  "missing":[], ...}

# 409 when it does not ask for an acknowledgement.`,
  );

  protected readonly mails = `// GET /api/notifications: a notification sent with mail counts its
// readers' mails by state.
{"id": "5f0c2b8e-4d1e-4b2a-9a77-02c6f2c4e1a0", "mail": true,
 "mails": {"accepted": 38, "failed": 1, "noaddress": 2}}

// pending    to send, now or at its next attempt
// sending    a node is sending it
// accepted   accepted by the relay
// failed     refused for good, or refused for now for 24 hours
// unknown    the connection went before the relay answered
// cancelled  withdrawn, replaced or expired first
// noaddress  the reader has no address`;

  protected readonly refused = `HTTP/1.1 422 Unprocessable Entity

{"error":"level \\"urgent\\": allowed are info, success, warning, error"}`;

  protected readonly incoming = `POST /hooks/ack HTTP/1.1
Content-Type: application/json
X-Remote-User: meerkat                 # the caller, with no role, passed
                                       # the way your route passes identity
Meerkat-Notification: 5f0c2b8e-...    # the id you gave
Meerkat-Attempt: 1                     # 2 or 3: sent again after a failure
User-Agent: meerkat-notifications

{"notification":"5f0c2b8e-...","tenant":"038ae636...","tenantName":"Acme",
 "meta":{"station":"42"},"ack":"single",
 "by":{"id":"eb7f08a2...","username":"bob","name":"Bob Durand"},
 "at":"2026-10-25T16:17:31Z","acknowledged":1,"recipients":3,"complete":true}`;
}
