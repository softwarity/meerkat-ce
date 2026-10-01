import { Pipe, PipeTransform } from "@angular/core";

// A moment, written in a named zone, in a shape nobody has to decode.
//
// ONE FORMAT, and it is not the reader's locale: `2026-09-21 14:27`. An
// operator's screen shows the same string to everybody, sorts the way it
// reads, and never asks whether "9/12" is September or December - which is
// exactly the question a scheduler's reader must not have to ask about the
// hour a job runs.
//
// UTC says so, with the one letter that means it everywhere: `2026-09-21
// 12:27 Z`. A named zone does not need it - the switch on the screen says
// which one is showing - but an hour written alone is an hour somebody reads
// as their own.
//
// Angular's DatePipe cannot do the zone part: its timezone argument takes an
// offset or an abbreviation, never an IANA name, so "Europe/Paris" silently
// falls back to the browser's - and a gateway's operator is often not where
// their browser is. Intl does know the names, and it is the same list the
// profile's zone picker is built from.
//
// Seconds in, because that is what the gateway stores: every time on the wire
// here is a unix second, and multiplying by a thousand at forty call sites is
// where one of them ends up in 1970.
@Pipe({ name: "zoned" })
export class ZonedDatePipe implements PipeTransform {
  transform(
    seconds: number | null | undefined,
    zone: string,
    style: "short" | "long" = "short",
  ): string {
    if (!seconds) return "";
    return format(seconds * 1000, zone || "UTC", style === "long");
  }
}

function format(ms: number, zone: string, seconds: boolean): string {
  const utc = zone.toUpperCase() === "UTC" || zone === "Etc/UTC";
  try {
    return written(ms, zone, seconds) + (utc ? " Z" : "");
  } catch {
    // A zone this browser does not know THROWS, and a screen that throws while
    // drawing a date shows nothing at all. An account carrying a zone that was
    // renamed - or a browser too old for it - falls back to UTC, and the Z
    // then says so rather than letting somebody read it as their own hours.
    return written(ms, "UTC", seconds) + " Z";
  }
}

function written(ms: number, zone: string, seconds: boolean): string {
  // Assembled from the parts rather than taken from a locale's pattern: the
  // point is that the string does NOT vary.
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone: zone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    ...(seconds ? { second: "2-digit" as const } : {}),
  }).formatToParts(new Date(ms));
  const at = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((p) => p.type === type)?.value ?? "";
  const clock =
    `${at("hour")}:${at("minute")}` + (seconds ? `:${at("second")}` : "");
  return `${at("year")}-${at("month")}-${at("day")} ${clock}`;
}
