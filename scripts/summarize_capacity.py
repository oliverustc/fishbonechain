#!/usr/bin/env python3
"""Summarize high-pressure capacity benchmark results."""

from __future__ import annotations

import argparse
import csv
import json
import re
from collections import defaultdict
from datetime import datetime
from pathlib import Path


CHAIN_NAMES = {
    "ws://10.2.2.11:9945": "child1",
    "ws://10.2.2.12:9945": "child1",
    "ws://10.2.2.13:9945": "child1",
    "ws://10.2.2.14:9946": "child2",
    "ws://10.2.2.15:9946": "child2",
    "ws://10.2.2.16:9946": "child2",
    "ws://10.2.2.17:9947": "child3",
    "ws://10.2.2.18:9947": "child3",
    "ws://10.2.2.19:9947": "child3",
    "ws://10.2.3.11:9947": "child3",
    "ws://10.2.3.12:9947": "child3",
    "ws://10.2.3.13:9947": "child3",
    "ws://10.2.2.11:9948": "child4",
    "ws://10.2.2.20:9948": "child4",
    "ws://10.2.2.21:9948": "child4",
    "ws://10.2.2.22:9948": "child4",
    "ws://10.2.2.20:9949": "child5",
    "ws://10.2.3.11:9949": "child5",
    "ws://10.2.3.12:9949": "child5",
    "ws://10.2.3.13:9949": "child5",
    "ws://10.2.2.11:9950": "child6",
    "ws://10.2.3.14:9950": "child6",
    "ws://10.2.3.15:9950": "child6",
    "ws://10.2.3.16:9950": "child6",
}

ORDER = ["child4", "child1", "child6", "child3", "child2", "child5"]
CAP = 10000


def parse_ts(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def summarize_state(path: Path) -> dict[str, dict[str, float]]:
    rows_by_chain: dict[str, list[tuple[datetime, int]]] = defaultdict(list)
    with path.open(newline="") as f:
        for row in csv.DictReader(f):
            chain = CHAIN_NAMES.get(row["chain_url"], row["chain_url"])
            try:
                rows_by_chain[chain].append((parse_ts(row["timestamp"]), int(row["submissions_count"])))
            except (ValueError, KeyError):
                continue

    out = {}
    for chain, rows in rows_by_chain.items():
        rows.sort(key=lambda item: item[0])
        if not rows:
            continue
        t0, initial = rows[0]
        max_ts, max_subs = max(rows, key=lambda item: item[1])
        reached = [item for item in rows if item[1] >= CAP]
        if reached:
            end_ts, end_subs = reached[0]
        else:
            end_ts, end_subs = max_ts, max_subs
        delta = max(0, end_subs - initial)
        elapsed_s = max((end_ts - t0).total_seconds(), 0.001)
        out[chain] = {
            "initial_subs": initial,
            "max_subs": max_subs,
            "accepted_delta": delta,
            "time_to_cap_s": elapsed_s if end_subs >= CAP else 0.0,
            "accepted_tps": delta / elapsed_s,
            "hit_cap": 1 if end_subs >= CAP else 0,
        }
    return out


def summarize_precise(path: Path) -> dict[str, dict[str, float]]:
    data = json.loads(path.read_text())
    hit_summary = data.get("hit_summary", {})
    out = {}
    for url, item in hit_summary.items():
        chain = CHAIN_NAMES.get(url, url)
        if not item:
            out[chain] = {
                "initial_subs": 0,
                "max_subs": 0,
                "accepted_delta": 0,
                "time_to_cap_s": 0.0,
                "accepted_tps": 0.0,
                "hit_cap": 0,
            }
            continue

        delta = float(item.get("accepted_delta", 0))
        elapsed_s = max(float(item.get("elapsed_s", 0)), 0.001)
        out[chain] = {
            "initial_subs": float(item.get("initial_subs", 0)),
            "max_subs": float(item.get("cap_subs", 0)),
            "accepted_delta": delta,
            "time_to_cap_s": elapsed_s,
            "accepted_tps": delta / elapsed_s,
            "hit_cap": 1 if float(item.get("cap_subs", 0)) >= CAP else 0,
        }
    return out


def summarize_burst_logs(log_dir: Path, n: int) -> tuple[int, int, int, int]:
    final_re = re.compile(r"sent=([0-9]+).*ok=([0-9]+).*reject=([0-9]+).*fail=([0-9]+).*final=true")
    sent = ok = reject = fail = 0
    for path in sorted(log_dir.glob(f"n{n}_burst_*.log")):
        last = None
        with path.open(errors="replace") as f:
            for line in f:
                m = final_re.search(line)
                if m:
                    last = m
        if last:
            sent += int(last.group(1))
            ok += int(last.group(2))
            reject += int(last.group(3))
            fail += int(last.group(4))
    return sent, ok, reject, fail


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--raw-dir", required=True)
    parser.add_argument("--log-dir", required=True)
    parser.add_argument("--out", default="docs/experiments/figures/data/exp_capacity_summary.csv")
    args = parser.parse_args()

    raw_dir = Path(args.raw_dir)
    log_dir = Path(args.log_dir)
    rows = []

    for n in range(1, 7):
        state_path = raw_dir / f"exp_capacity_n{n}_state.csv"
        precise_path = raw_dir / f"exp_capacity_n{n}_precise_summary.json"
        if precise_path.exists():
            chain_stats = summarize_precise(precise_path)
            source = "precise"
        elif state_path.exists():
            chain_stats = summarize_state(state_path)
            source = "state"
        else:
            print(f"[warn] skip N={n}: missing {precise_path} and {state_path}")
            continue
        active = ORDER[:n]
        total_delta = sum(chain_stats.get(c, {}).get("accepted_delta", 0) for c in active)
        hit_caps = sum(chain_stats.get(c, {}).get("hit_cap", 0) for c in active)
        time_to_all_caps = max(
            (chain_stats.get(c, {}).get("time_to_cap_s", 0.0) for c in active),
            default=0.0,
        )
        total_tps = total_delta / time_to_all_caps if time_to_all_caps > 0 else 0.0
        sum_chain_tps = sum(chain_stats.get(c, {}).get("accepted_tps", 0) for c in active)
        sent, ok, reject, fail = summarize_burst_logs(log_dir, n)
        rows.append({
            "n": n,
            "active_chains": "+".join(active),
            "measurement_source": source,
            "aggregate_chain_accepted_delta": f"{total_delta:.0f}",
            "aggregate_chain_accepted_tps": f"{total_tps:.2f}",
            "sum_individual_chain_tps": f"{sum_chain_tps:.2f}",
            "chains_hit_cap": hit_caps,
            "time_to_all_caps_s": f"{time_to_all_caps:.1f}",
            "worker_sent": sent,
            "worker_ok": ok,
            "worker_reject": reject,
            "worker_fail": fail,
        })

    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=[
            "n",
            "active_chains",
            "measurement_source",
            "aggregate_chain_accepted_delta",
            "aggregate_chain_accepted_tps",
            "sum_individual_chain_tps",
            "chains_hit_cap",
            "time_to_all_caps_s",
            "worker_sent",
            "worker_ok",
            "worker_reject",
            "worker_fail",
        ])
        writer.writeheader()
        writer.writerows(rows)
    print(f"[saved] {out} ({len(rows)} rows)")


if __name__ == "__main__":
    main()
