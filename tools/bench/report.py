#!/usr/bin/env python3
"""Turn a benchmark run (tools/bench/out/raw) into results.json and summary.md.

results.json is the record: the CI appends it to the history the doc site
reads, so its shape is a contract - add fields, never rename one. summary.md is
the same thing for a human, printed at the end of a run and posted as the CI
job summary.

A scenario whose requests did not come back 200 is not a fast gateway, it is a
misconfigured one: below 99% the run is refused, loudly, rather than recorded.
That holds at the fixed rate for every scenario, and at max for the comparable
ones. A DEPLOYED scenario is the exception at max, on purpose: what it measures
is how an assembled setup holds, and Traefik's ForwardAuth answering 500s once
its outgoing calls pile up is that answer - so it is recorded with its error
rate, and shown beside the throughput, instead of being hidden by a refusal.
"""

import json
import re
import statistics
import sys
from pathlib import Path

SCHEMA = 1
GATEWAYS = ["meerkat", "gostd", "kong", "apisix", "traefik"]
SCENARIOS = ["proxy", "auth", "auth-jwt", "auth-delegated", "limit"]
MIN_SUCCESS = 0.99
# The Go benchmarks that go through a socket, and so read against BareProxy.
# The others (selection, deduction, the rate gate) measure a decision with no
# socket at all, and a ratio to a proxy would say nothing about them.
PROXIED = {"BareProxy", "RouteMinimal", "RouteAmong50", "RouteWithFilters", "AuthenticatedToken"}

# What each scenario is, per gateway, in words the doc site shows as is.
KIND = {
    "proxy": "comparable",
    "auth": "comparable",
    "limit": "comparable",
    "auth-delegated": "deployed",
    # Meerkat only: what it usually does in production, which no other gateway
    # does here. Held to the same 99% as the comparable ones.
    "auth-jwt": "extra",
}
AUTH_HOW = {
    ("meerkat", "auth"): "personal API token resolved by the gateway, caller's name and id forwarded as headers, no roles",
    ("meerkat", "auth-jwt"): "personal API token resolved by the gateway, caller forwarded as an ES256-signed JWT",
    ("kong", "auth"): "key-auth plugin, consumer's name and id forwarded as headers, no roles",
    ("apisix", "auth"): "key-auth plugin, consumer's name forwarded as headers, no roles",
    ("traefik", "auth-delegated"): "ForwardAuth: an external service decides on every request",
}


def load_env(path):
    values = {}
    for line in path.read_text().splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            values[key] = value
    return values


def version_of(gw, meta):
    if gw == "meerkat":
        return meta["commit"][:7]
    if gw == "gostd":
        return meta.get("go_version", "")
    return meta.get(f"image_{gw}", "").split(":")[-1]


def ms(seconds):
    return round(seconds * 1000, 3)


def oha(path):
    """Read one oha JSON output into the figures we keep."""
    data = json.loads(path.read_text())
    summary = data["summary"]
    codes = {int(k): v for k, v in (data.get("statusCodeDistribution") or {}).items()}
    # An in-flight request cut by the deadline is how every timed run ends,
    # not a failure of the thing under test.
    errors = sum(v for k, v in (data.get("errorDistribution") or {}).items()
                 if "deadline" not in k)
    answered = sum(codes.values())
    ok = codes.get(200, 0)
    total = answered + errors
    pct = data.get("latencyPercentiles") or {}
    return {
        "rps": round(summary.get("requestsPerSec") or 0, 1),
        "p50": ms(pct.get("p50") or 0),
        "p90": ms(pct.get("p90") or 0),
        "p99": ms(pct.get("p99") or 0),
        "requests": total,
        "success": round(ok / total, 5) if total else 0,
    }


def mebibytes(text):
    """docker stats writes 12.3MiB, 1.02GiB, 512KiB."""
    match = re.match(r"([\d.]+)\s*([KMG]i?B|B)", text.strip())
    if not match:
        return None
    value, unit = float(match.group(1)), match.group(2)
    factor = {"B": 1 / 1048576, "KiB": 1 / 1024, "KB": 1 / 1024, "MiB": 1, "MB": 1,
              "GiB": 1024, "GB": 1024}[unit]
    return round(value * factor, 1)


def gobench(path):
    """Median ns/op and allocs/op per benchmark over the -count runs."""
    runs = {}
    line = re.compile(r"^Benchmark(\S+?)(?:-\d+)?\s+\d+\s+([\d.]+) ns/op(?:\s+([\d.]+) B/op)?(?:\s+([\d.]+) allocs/op)?")
    for text in path.read_text().splitlines():
        match = line.match(text)
        if match:
            entry = runs.setdefault(match.group(1), {"ns": [], "allocs": []})
            entry["ns"].append(float(match.group(2)))
            if match.group(4):
                entry["allocs"].append(float(match.group(4)))
    if not runs:
        return []
    floor = statistics.median(runs["BareProxy"]["ns"]) if "BareProxy" in runs else None
    result = []
    for name, entry in runs.items():
        ns = statistics.median(entry["ns"])
        result.append({
            "name": name,
            "nsPerOp": round(ns),
            "allocsPerOp": round(statistics.median(entry["allocs"])) if entry["allocs"] else None,
            "vsBareProxy": round(ns / floor, 2) if floor and name in PROXIED else None,
        })
    return result


def main():
    out = Path(sys.argv[1])
    raw = out / "raw"
    meta = load_env(raw / "meta.env")

    direct_fixed = oha(raw / "direct-proxy-fixed.json")
    direct_max = oha(raw / "direct-proxy-max.json")

    refused = []
    gateways = []
    for gw in GATEWAYS:
        startup = raw / f"{gw}-startup.txt"
        if not startup.exists():
            continue
        entry = {
            "name": gw,
            "version": version_of(gw, meta),
            "startupMs": int(startup.read_text().strip()),
            "memoryIdleMiB": mebibytes((raw / f"{gw}-mem-idle.txt").read_text()),
            "memoryPeakMiB": None,
            "scenarios": {},
        }
        peaks = []
        for scenario in SCENARIOS:
            fixed_path = raw / f"{gw}-{scenario}-fixed.json"
            if not fixed_path.exists():
                continue
            fixed = oha(fixed_path)
            best = oha(raw / f"{gw}-{scenario}-max.json")
            for label, figures in (("fixed", fixed), ("max", best)):
                if label == "max" and KIND[scenario] == "deployed":
                    continue
                if figures["success"] < MIN_SUCCESS:
                    refused.append(f"{gw} {scenario} {label}: {figures['success']:.1%} answered 200")
            fixed["overheadP50"] = round(fixed["p50"] - direct_fixed["p50"], 3)
            fixed["overheadP99"] = round(fixed["p99"] - direct_fixed["p99"], 3)
            scenario_entry = {"kind": KIND[scenario], "fixed": fixed, "max": best}
            if (gw, scenario) in AUTH_HOW:
                scenario_entry["how"] = AUTH_HOW[(gw, scenario)]
            entry["scenarios"][scenario] = scenario_entry
            mem_file = raw / f"{gw}-{scenario}-mem.txt"
            if mem_file.exists():
                peaks += [m for m in (mebibytes(line) for line in mem_file.read_text().splitlines()) if m]
        entry["memoryPeakMiB"] = max(peaks) if peaks else None
        gateways.append(entry)

    source = re.search(r"meerkat@([0-9a-f]{7,})", meta.get("subject", ""))
    results = {
        "schema": SCHEMA,
        "date": meta["date"],
        "commit": meta["commit"],
        "sourceCommit": source.group(1) if source else None,
        "machine": {
            "runner": meta["runner"],
            "os": meta["host_os"],
            "arch": meta["docker_arch"],
            "cpuModel": meta["cpu_model"],
            "cpus": int(meta["host_cpus"]),
            "threadsPerCore": int(meta["threads_per_core"] or 1),
            "memoryGiB": round(int(meta["host_memory_kb"]) / 1048576, 1),
            "docker": meta["docker_version"],
        },
        "protocol": {
            "gatewayCpus": 1,
            "gatewayMemory": meta["memory"],
            "rate": int(meta["rate"]),
            "fixedDuration": meta["fixed"],
            "maxDuration": meta["max"],
            "connectionsFixed": int(meta["conn_fixed"]),
            "connectionsMax": int(meta["conn_max"]),
            "loadCpus": meta["load_cpus"],
            "loadGenerator": meta["image_oha"],
        },
        "direct": {"fixed": direct_fixed, "max": direct_max},
        "gateways": gateways,
        "goBench": gobench(raw / "gobench.txt") if (raw / "gobench.txt").exists() else [],
    }
    (out / "results.json").write_text(json.dumps(results, indent=2) + "\n")

    summary = render(results)
    (out / "summary.md").write_text(summary)
    print(summary)

    if refused:
        print("Refused - these did not answer 200 and would be measured as something they are not:")
        for line in refused:
            print("  " + line)
        sys.exit(2)


def render(r):
    m = r["machine"]
    lines = [
        "## Gateway benchmark",
        "",
        f"**{m['runner']}** - {m['cpuModel']}, {m['cpus']} CPUs ({m['threadsPerCore']} thread(s) per core), "
        f"{m['memoryGiB']} GiB, {m['arch']}",
        "",
        f"Each gateway pinned to 1 CPU and {r['protocol']['gatewayMemory']}; fixed rate "
        f"{r['protocol']['rate']} req/s for {r['protocol']['fixedDuration']}, max for {r['protocol']['maxDuration']}. "
        f"Direct to the upstream: p50 {r['direct']['fixed']['p50']} ms, p99 {r['direct']['fixed']['p99']} ms, "
        f"max {r['direct']['max']['rps']:.0f} req/s.",
        "",
    ]
    for scenario in SCENARIOS:
        rows = [(g, g["scenarios"][scenario]) for g in r["gateways"] if scenario in g["scenarios"]]
        if scenario == "auth":
            rows += [(g, g["scenarios"]["auth-jwt"]) for g in r["gateways"] if "auth-jwt" in g["scenarios"]]
            rows += [(g, g["scenarios"]["auth-delegated"]) for g in r["gateways"] if "auth-delegated" in g["scenarios"]]
        if scenario in ("auth-delegated", "auth-jwt") or not rows:
            continue
        lines += [f"### {scenario}", "",
                  "| Gateway | p50 ms | p99 ms | added p50 | added p99 | max req/s |",
                  "|---|---:|---:|---:|---:|---:|"]
        for g, s in rows:
            name = g["name"] + {"deployed": " (delegated)", "extra": " (signed JWT)"}.get(s["kind"], "")
            f, x = s["fixed"], s["max"]
            errors = f" ({1 - x['success']:.1%} errors)" if x["success"] < MIN_SUCCESS else ""
            lines.append(f"| {name} | {f['p50']} | {f['p99']} | {f['overheadP50']} | {f['overheadP99']} | {x['rps']:.0f}{errors} |")
        lines.append("")
    lines += ["### Footprint", "", "| Gateway | version | ready in ms | memory idle MiB | memory peak MiB |",
              "|---|---|---:|---:|---:|"]
    for g in r["gateways"]:
        lines.append(f"| {g['name']} | {g['version']} | {g['startupMs']} | {g['memoryIdleMiB']} | {g['memoryPeakMiB']} |")
    lines.append("")
    if r["goBench"]:
        lines += ["### Go micro-benchmarks", "", "| Benchmark | ns/op | allocs/op | x bare proxy |", "|---|---:|---:|---:|"]
        for b in r["goBench"]:
            ratio = b["vsBareProxy"] if b["vsBareProxy"] is not None else "-"
            lines.append(f"| {b['name']} | {b['nsPerOp']} | {b['allocsPerOp']} | {ratio} |")
        lines.append("")
    return "\n".join(lines)


if __name__ == "__main__":
    main()
