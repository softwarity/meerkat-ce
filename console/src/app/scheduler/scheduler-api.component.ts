import { Component, computed } from "@angular/core";
import { httpResource } from "@angular/common/http";
import { MatButtonModule } from "@angular/material/button";
import { MatExpansionModule } from "@angular/material/expansion";
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
  // The internal plane, by the name a service reaches it under from INSIDE the
  // cluster - not the address this browser is at, which is an operator's way
  // in and often not routable from a pod. The runtime's own name for it when
  // it gave one (a Service's FQDN, a Swarm or compose service), else the one
  // our deployment files give the gateway.
  protected readonly origin = computed(() => {
    const e = this.editionRes.value();
    return e?.internalOrigin || `http://meerkat:${e?.internalPort || "7070"}`;
  });


  protected readonly writeCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Create: the response carries the id.
curl -X POST ${this.origin()}/api/schedules -H "$AUTH" \\
  -H "Content-Type: application/json" --data @schedule.json   # without the comments
# {"id":"sch_9f2c...","name":"station 42, hourly poll","nextAt":1789999188, ...}

# Replace it by its id. Delete it: 204.
curl -X PUT    ${this.origin()}/api/schedules/sch_9f2c -H "$AUTH" \\
  -H "Content-Type: application/json" --data @schedule.json
curl -X DELETE ${this.origin()}/api/schedules/sch_9f2c -H "$AUTH"

# Lost the id? Find the schedule by its metadata.
curl "${this.origin()}/api/schedules?meta.station=42" -H "$AUTH"`,
  );

  // Annotated, and named .jsonc for it: the values a field takes belong beside
  // the field, not three paragraphs below.
  protected readonly payload = `{
  "name": "station 42, hourly poll",
  "description": "Fetches the hourly readings of station 42",
                                  // optional, shown under the name in the console;
                                  // a PUT without it keeps the current one
  "roles": ["station_fetch"],     // roles the call is made with; "role" for one.
                                  // The call is made as user "meerkat" with these
                                  // roles, and the route's rules check them
  "routeId": "stations-api",      // a route of this gateway, never a URL
  "method": "POST",               // method of the call, POST by default
  "path": "/stations/42/poll",

  "headers": {                    // extra headers for your service
    "X-Action": "fetch",
    "X-Api-Key": "\${stations-key}"
  },                              // a "\${name}" value is a vault reference,
                                  // resolved at call time, never stored
  "body": {                       // an object or an array is sent as
    "kind": "poll",               // application/json; a JSON string is sent
    "region": "west",             // as is (form, XML)
    "station": "42"
  },

  "every": "PT1H",                // a cadence: PT30M, PT6H, P1D. One minute at least

  "overlap": "skip",              // a run due while the last one is still running
                                  // waits for it to end ("skip" only)
  "catchUp": 3600,                // seconds: how late a missed run may still be
                                  // made. After a night down, only the last
                                  // hourly run is made, not twelve. 0 = no limit
  "timeout": "PT30S",             // how long to wait for the response, not the work.
                                  // Two minutes at most: longer work answers 202

  "metadata": {                   // your own keys, to find the schedule.
    "svc": "stations",            // Not sent with the call
    "station": "42",
    "env": "prod"
  }
}`;

  protected readonly once = `{
  "name": "panier 4471, relance",     // a delayed action, triggered by an
  "role": "orders_notify",            // event rather than a calendar
  "routeId": "orders-api",
  "path": "/jobs/remind",

  "at": "2026-10-03T04:00:00Z",       // a single date, RFC 3339 with its offset.
                                      // The call is made once, then the
                                      // schedule is finished. No timezone:
                                      // the offset gives it

  "metadata": { "cart": "4471" }      // your own keys, to find it
}`;

  protected readonly calendar = `{
  "name": "monday report",
  "role": "reporting",            // a single role
  "routeId": "stations-api",
  "path": "/jobs/report",

  "cron": "0 3 * * MON",          // instead of "every": minute hour day month weekday
  "timezone": "Europe/Paris",     // time zone of the cron; UTC by default

  "metadata": { "kind": "report" }
}`;

  protected readonly readCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# All schedules, filtered by the query parameters.
curl ${this.origin()}/api/schedules -H "$AUTH"

# By metadata.
curl "${this.origin()}/api/schedules?meta.station=42" -H "$AUTH"
curl "${this.origin()}/api/schedules?meta.kind=poll&meta.region=west" -H "$AUTH"

# By regular expression: every station numbered under a hundred.
curl "${this.origin()}/api/schedules?meta.station=~^[0-9]{1,2}$" -H "$AUTH"

# Pause, resume, run now, cancel the current run.
curl -X POST   ${this.origin()}/api/schedules/sch_9f2c/pause  -H "$AUTH"
curl -X POST   ${this.origin()}/api/schedules/sch_9f2c/resume -H "$AUTH"
curl -X POST   ${this.origin()}/api/schedules/sch_9f2c/run    -H "$AUTH"
curl -X DELETE ${this.origin()}/api/schedules/sch_9f2c/run    -H "$AUTH"

# Every schedule a filter matches, in one call (same filter as the list).
# The response lists those schedules as updated.
curl -X POST "${this.origin()}/api/schedules/pause?meta.svc=stations&meta.job=import" -H "$AUTH"
curl -X POST "${this.origin()}/api/schedules/resume?meta.svc=stations&meta.job=import" -H "$AUTH"`,
  );

  protected readonly runsCmd = computed(
    () => `AUTH="Authorization: Bearer $MEERKAT_TOKEN"

# Run history, newest first.
curl "${this.origin()}/api/schedules/sch_9f2c/runs" -H "$AUTH"
# [{"id":"run-17906f3c9d2a4e10-9f2c11","runId":"GvbDqbNG__YTOY00","attempt":1,
#   "node":"meerkat-7d9c","endedAt":1790770191,"state":"done","status":200, ...}]

# Replay one: a new run with the current payload; the history shows
# which run it replays. Without a body, this call is "run now".
curl -X POST ${this.origin()}/api/schedules/sch_9f2c/run -H "$AUTH" \\
  -H "Content-Type: application/json" \\
  -d '{"replayOf":"run-17906f3c9d2a4e10-9f2c11"}'`,
  );

  protected readonly incoming = computed(
    () => `POST /stations/42/poll HTTP/1.1
Host: ${this.dataOrigin().replace(/^https?:\/\//, "")}
X-Remote-User: meerkat                # the caller, passed the way your route
X-Roles: station_fetch                # passes identity: headers, REMOTE_USER, a JWT
Meerkat-Job: sch_9f2c                 # the schedule
Meerkat-Job-Run: GvbDqbNG__YTOY00     # the run, the same on every attempt:
                                      # ignore one you already handled
Meerkat-Job-Attempt: 1                # 2 or more: sent again (a gateway stopped
                                      # before your response, an attempt failed,
                                      # or a lost run was resent by retryLost)
User-Agent: meerkat-scheduler`,
  );

  protected readonly notYet = `HTTP/1.1 424 Failed Dependency
Retry-After: 600                      # seconds, or an HTTP date

the daily extract is not published yet   # kept on this run, visible
                                         # in the history`;

  protected readonly reportCmd = computed(
    () => `# You answered 202: the run stays open until you close it. The run id
# authenticates the report: no token needed, whatever the Access mode.
curl -X PATCH ${this.origin()}/api/schedules/sch_9f2c/run \\
  -H "Content-Type: application/json" \\
  -d '{"run":"GvbDqbNG__YTOY00","progress":73,"detail":"7 233 / 9 882 records"}'
# 204: continue. Report at least every five minutes, or the run is lost.

# 410 Gone: stop. The run is already closed; send no more reports.
# {"stop":"paused","error":"the schedule is paused: stop the work, ..."}
# stop is paused, cancelled, deleted, lost or closed.

# ... and when the work is over.
curl -X PATCH ${this.origin()}/api/schedules/sch_9f2c/run \\
  -H "Content-Type: application/json" \\
  -d '{"run":"GvbDqbNG__YTOY00","state":"done","detail":"9 882 records"}'

# An unknown run id gets a 409.`,
  );
}
