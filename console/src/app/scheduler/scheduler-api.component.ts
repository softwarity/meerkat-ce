import { Component, computed } from "@angular/core";
import { httpResource } from "@angular/common/http";
import { MatButtonModule } from "@angular/material/button";
import { MatExpansionModule } from "@angular/material/expansion";
import { MatIconModule } from "@angular/material/icon";
import { RouterLink } from "@angular/router";
import { Edition } from "../api.service";
import { SnippetComponent } from "../shared/snippet.component";

// How a backend service gets itself called on a timer (SCHED-01), in the
// drawer of the screen that shows the result.
//
// It is here rather than only in the documentation for one reason: nothing on
// this screen CREATES a schedule - a schedule runs as an account, and the
// administrators reading this are not those accounts - so an operator arriving
// with "how do I get one of these" has no next step to click. The next step is
// a call their service makes, with a token minted on this very console, and
// this is where it is written down.
@Component({
  selector: "app-scheduler-api",
  imports: [
    MatButtonModule,
    MatExpansionModule,
    MatIconModule,
    RouterLink,
    SnippetComponent,
  ],
  templateUrl: "./scheduler-api.component.html",
  styleUrl: "./scheduler-api.component.scss",
})
export class SchedulerApiComponent {
  // Where the APPLICATIONS answer - asked of the gateway, never worked out
  // from this console's address: it is only named here to show what a service
  // RECEIVES, the two planes being two origins.
  private readonly editionRes = httpResource<Edition>(() => "/api/edition");
  protected readonly dataOrigin = computed(
    () => this.editionRes.value()?.dataOrigin || "https://apps.example.com",
  );
  // The control plane, by the name a service reaches it under from INSIDE the
  // cluster - not the address this browser is at, which is an operator's way
  // in and often not routable from a pod.
  protected readonly origin = "http://meerkat:9090";

  protected readonly mintCmd = `# Perimeter "schedules" - it opens this API and nothing else on this
# port. The clear value is shown once.
#
# The same thing over the API, as root:
curl -X POST http://meerkat:9090/api/admin-tokens \\
  -H "Authorization: Bearer $ROOT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"stations","scope":"schedules","days":365}'`;

  protected readonly writeCmd = `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Create: the gateway hands back the id.
curl -X POST http://meerkat:9090/api/schedules -H "$AUTH" \\
  -H "Content-Type: application/json" --data @schedule.json   # without the comments
# {"id":"sch_9f2c...","name":"station 42, hourly poll","nextAt":1789999188, ...}

# Change it, on that id. Remove it: 204.
curl -X PUT    http://meerkat:9090/api/schedules/sch_9f2c -H "$AUTH" \\
  -H "Content-Type: application/json" --data @schedule.json
curl -X DELETE http://meerkat:9090/api/schedules/sch_9f2c -H "$AUTH"

# Did not keep the id? Ask for it back by what you filed it under.
curl "http://meerkat:9090/api/schedules?meta.station=42" -H "$AUTH"`;

  // Annotated, and named .jsonc for it: the values a field takes belong beside
  // the field, not three paragraphs below.
  protected readonly payload = `{
  "name": "station 42, hourly poll",
  "roles": ["station_fetch"],     // what the CALL carries; "role" for a single one.
                                  // No account behind it: it goes out as "meerkat"
                                  // holding these, and the route's rule reads them
  "routeId": "stations-api",      // a route of this gateway, never a URL
  "method": "POST",               // the verb of the OUTGOING call, POST by default
  "path": "/stations/42/poll",

  "headers": {                    // what YOUR service asked to be told, beyond
    "X-Action": "fetch",          // the ones the gateway writes itself.
    "X-Api-Key": "\${stations-key}"
  },                              // A "\${name}" value is a VAULT REFERENCE,
                                  // resolved at the call: the secret is never stored
  "body": {                       // an object or an array goes out as it stands,
    "kind": "poll",               // application/json; a JSON STRING goes out as
    "region": "west",             // its CONTENT, which is how a form or an XML
    "station": "42"               // document is sent
  },

  "every": "PT1H",                // a cadence: PT30M, PT6H, P1D. One minute at least

  "overlap": "skip",              // the turn that arrives while the last call is
                                  // still out: "skip" lets it go (and moves it to
                                  // the next cadence), "run" calls anyway
  "catchUp": 3600,                // seconds: how LATE a missed turn may still go
                                  // out. A gateway down all night fires the last
                                  // hourly turn on waking, not twelve. 0 = whenever
  "timeout": "PT30S",             // how long we wait for the ANSWER, not the work.
                                  // Two minutes at most: longer work answers 202

  "metadata": {                   // yours: how you find this one again. Never
    "svc": "stations",            // sent with the call - and where an "owner"
    "station": "42",              // goes if you want one, it is a key like
    "env": "prod"                 // any other
  }
}`;

  protected readonly once = `{
  "name": "panier 4471, relance",     // a DELAYED ACTION: the trigger is
  "role": "orders_notify",            // something that happened, not a calendar
  "routeId": "orders-api",
  "path": "/jobs/remind",

  "at": "2026-10-03T04:00:00Z",       // ONE date, RFC 3339 with its offset.
                                      // The call goes out then, once, and the
                                      // schedule is finished - nothing is armed
                                      // after it. No timezone beside it: the
                                      // date carries its own

  "metadata": { "cart": "4471" }      // yours: how you find it again
}`;

  protected readonly calendar = `{
  "name": "monday report",
  "role": "reporting",            // one role, said in the singular
  "routeId": "stations-api",
  "path": "/jobs/report",

  "cron": "0 3 * * MON",          // instead of "every": minute hour day month weekday
  "timezone": "Europe/Paris",     // where that calendar is read; UTC by default

  "metadata": { "kind": "report" }
}`;

  protected readonly readCmd = `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Everything on this gateway, narrowed by what you ask for.
curl http://meerkat:9090/api/schedules -H "$AUTH"

# The service's own filing system, asked for in its own terms.
curl "http://meerkat:9090/api/schedules?meta.station=42" -H "$AUTH"
curl "http://meerkat:9090/api/schedules?meta.kind=poll&meta.region=west" -H "$AUTH"

# An expression, for a fleet: every station numbered under a hundred.
curl "http://meerkat:9090/api/schedules?meta.station=~^[0-9]{1,2}$" -H "$AUTH"

# Pause one, let it fire again, owe a turn now.
curl -X POST http://meerkat:9090/api/schedules/sch_9f2c/pause  -H "$AUTH"
curl -X POST http://meerkat:9090/api/schedules/sch_9f2c/resume -H "$AUTH"
curl -X POST http://meerkat:9090/api/schedules/sch_9f2c/run    -H "$AUTH"`;

  protected readonly runsCmd = `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# What each turn did, newest first.
curl "http://meerkat:9090/api/schedules/sch_9f2c/runs" -H "$AUTH"
# [{"id":"run-17906f3c9d2a4e10-9f2c11","runId":"GvbDqbNG__YTOY00","attempt":1,
#   "node":"meerkat-7d9c","endedAt":1790770191,"state":"done","status":200, ...}]

# One of them again: a NEW run, today's payload, and the history says
# which one it replays. Without a body, the same call is "run now".
curl -X POST http://meerkat:9090/api/schedules/sch_9f2c/run -H "$AUTH" \\
  -H "Content-Type: application/json" \\
  -d '{"replayOf":"run-17906f3c9d2a4e10-9f2c11"}'`;

  protected readonly incoming = computed(
    () => `POST /stations/42/poll HTTP/1.1
Host: ${this.dataOrigin().replace(/^https?:\/\//, "")}
X-Remote-User: meerkat                # the caller - however YOUR route hands
X-Roles: station_fetch                # identity over: headers, REMOTE_USER, a JWT
Meerkat-Job: sch_9f2c                 # which schedule
Meerkat-Job-Run: GvbDqbNG__YTOY00     # which TURN - the same on every attempt:
                                      # ignore one you have already acted on
Meerkat-Job-Attempt: 1                # 2 or more: a gateway stopped before your
                                      # answer came, and another sent it again
User-Agent: meerkat-scheduler`,
  );

  protected readonly notYet = `HTTP/1.1 424 Failed Dependency
Retry-After: 600                      # seconds, or an HTTP date

the daily extract is not published yet   # kept on this run, and read
                                         # from the history afterwards`;

  protected readonly reportCmd = `# You answered 202, so the run is yours until you close it - reported with
# your OWN token, on the same API as everything else.
curl -X PATCH http://meerkat:9090/api/schedules/sch_9f2c/run \\
  -H "Authorization: Bearer $MEERKAT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"run":"GvbDqbNG__YTOY00","state":"running","progress":60}'

# ... and when the work is over.
curl -X PATCH http://meerkat:9090/api/schedules/sch_9f2c/run \\
  -H "Authorization: Bearer $MEERKAT_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"run":"GvbDqbNG__YTOY00","state":"done","detail":"12000 rows"}'

# 204 both times. A report naming a run that is not the one in flight is
# refused with 409: a late word from last night cannot close tonight.
#
# The 202 already told us the work is YOURS: from then on the call is never
# sent again, whatever happens to the gateway. Report at least every five
# minutes, or the lease lapses and the run is closed as lost.`;
}
