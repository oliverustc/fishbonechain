#!/usr/bin/env python3
"""Summarize N=1..6 scaling + main-chain load raw CSV files."""

from __future__ import annotations

import argparse
import csv
import re
from collections import defaultdict
from datetime import datetime
from pathlib import Path
from statistics import mean


CHAIN_NAMES = {
    "ws://10.2.2.11:9945": "child1",
    "ws://10.2.2.14:9946": "child2",
    "ws://10.2.2.17:9947": "child3",
    "ws://10.2.2.11:9948": "child4",
    "ws://10.2.2.20:9949": "child5",
    "ws://10.2.2.11:9950": "child6",
}

ORDER = ["child4", "child1", "child6", "child3", "child2", "child5"]


def parse_ts(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def stable_values(values: list[int]) -> list[int]:
    values = [v for v in values if v > 0]
    if len(values) <= 2:
        return values
    return values[1:-1]


def summarize_child_state(path: Path) -> tuple[float, float]:
    by_chain_epoch: dict[str, dict[int, int]] = defaultdict(lambda: defaultdict(int))
    with path.open(newline="") as f:
        for row in csv.DictReader(f):
            chain = CHAIN_NAMES.get(row["chain_url"], row["chain_url"])
            try:
                epoch = int(row["epoch_id"])
                subs = int(row["submissions_count"])
            except ValueError:
                continue
            by_chain_epoch[chain][epoch] = max(by_chain_epoch[chain][epoch], subs)

    chain_means = []
    stable_epoch_counts = []
    for epochs in by_chain_epoch.values():
        vals = stable_values([subs for _, subs in sorted(epochs.items())])
        if not vals:
            continue
        chain_means.append(mean(vals))
        stable_epoch_counts.append(len(vals))

    if not chain_means:
        return 0.0, 1.0
    return sum(chain_means), max(mean(stable_epoch_counts), 1.0)


def summarize_main_blocks(path: Path, epoch_count_hint: float) -> tuple[float, float, float, float]:
    timestamps = []
    extrinsics_total = 0
    bridge_total = 0
    with path.open(newline="") as f:
        for row in csv.DictReader(f):
            timestamps.append(parse_ts(row["timestamp"]))
            extrinsics_total += int(row["extrinsics_total"])
            bridge_total += int(row["bridge_extrinsics"])

    if len(timestamps) >= 2:
        duration_min = max((max(timestamps) - min(timestamps)).total_seconds() / 60, 1 / 60)
    else:
        duration_min = 1 / 60

    bridge_per_min = bridge_total / duration_min
    main_per_min = extrinsics_total / duration_min
    bridge_share_pct = (bridge_total / extrinsics_total * 100) if extrinsics_total else 0.0

    return (
        bridge_total / max(epoch_count_hint, 1.0),
        bridge_per_min,
        main_per_min,
        bridge_share_pct,
    )


def summarize_workers(log_dir: Path, n: int) -> tuple[float, int, float]:
    total_ok = 0
    max_elapsed_s = 0.0
    stat_re = re.compile(r"elapsed=([0-9.]+)s\s+ok=([0-9]+)")

    for path in sorted(log_dir.glob(f"n{n}_worker_*.log")):
        last = None
        with path.open(errors="replace") as f:
            for line in f:
                match = stat_re.search(line)
                if match:
                    last = match
        if not last:
            print(f"[warn] N={n}: no worker stats in {path.name}")
            continue
        elapsed_s = float(last.group(1))
        ok = int(last.group(2))
        max_elapsed_s = max(max_elapsed_s, elapsed_s)
        total_ok += ok

    duration_min = max(max_elapsed_s / 60, 1 / 60)
    return total_ok / duration_min, total_ok, duration_min


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--raw-dir", required=True, help="Directory containing exp_scale_main_n*_*.csv")
    parser.add_argument("--log-dir", help="Directory containing n*_worker_*.log")
    parser.add_argument(
        "--out",
        default="docs/figures/data/exp_scale_mainchain_summary.csv",
        help="Output summary CSV path",
    )
    args = parser.parse_args()

    raw_dir = Path(args.raw_dir)
    log_dir = Path(args.log_dir) if args.log_dir else raw_dir
    out = Path(args.out)
    rows = []

    for n in range(1, 7):
        state_path = raw_dir / f"exp_scale_main_n{n}_state.csv"
        main_path = raw_dir / f"exp_scale_main_n{n}_main_blocks.csv"
        if not state_path.exists() or not main_path.exists():
            print(f"[warn] skip N={n}: missing {state_path.name} or {main_path.name}")
            continue

        child_subs_per_epoch, stable_epoch_count = summarize_child_state(state_path)
        bridge_per_epoch, bridge_per_min, main_tx_per_min, bridge_share_pct = summarize_main_blocks(
            main_path, stable_epoch_count
        )
        child_subs_per_min, worker_ok_total, worker_duration_min = summarize_workers(log_dir, n)
        if child_subs_per_min == 0:
            print(f"[warn] N={n}: worker logs unavailable, falling back to epoch-level child summary")
            child_subs_per_min = child_subs_per_epoch
        child_tps = child_subs_per_min / 60
        bridge_tps = bridge_per_min / 60
        main_tps = main_tx_per_min / 60
        bridge_to_child_pct = (bridge_tps / child_tps * 100) if child_tps else 0.0
        rows.append({
            "n": n,
            "active_chains": "+".join(ORDER[:n]),
            "child_subs_tps": f"{child_tps:.4f}",
            "main_bridge_tps": f"{bridge_tps:.4f}",
            "main_total_tps": f"{main_tps:.4f}",
            "main_bridge_to_child_tps_pct": f"{bridge_to_child_pct:.2f}",
            "main_bridge_share_of_observed_main_tx_pct": f"{bridge_share_pct:.2f}",
            "child_subs_per_min": f"{child_subs_per_min:.2f}",
            "child_ok_total": str(worker_ok_total),
            "worker_duration_min": f"{worker_duration_min:.2f}",
            "child_subs_per_epoch": f"{child_subs_per_epoch:.2f}",
            "main_bridge_tx_per_epoch": f"{bridge_per_epoch:.2f}",
            "main_bridge_tx_per_min": f"{bridge_per_min:.2f}",
            "main_tx_per_min": f"{main_tx_per_min:.2f}",
        })

    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", newline="") as f:
        writer = csv.DictWriter(
            f,
            fieldnames=[
                "n",
                "active_chains",
                "child_subs_tps",
                "main_bridge_tps",
                "main_total_tps",
                "main_bridge_to_child_tps_pct",
                "main_bridge_share_of_observed_main_tx_pct",
                "child_subs_per_min",
                "child_ok_total",
                "worker_duration_min",
                "child_subs_per_epoch",
                "main_bridge_tx_per_epoch",
                "main_bridge_tx_per_min",
                "main_tx_per_min",
            ],
        )
        writer.writeheader()
        writer.writerows(rows)

    print(f"[saved] {out} ({len(rows)} rows)")


if __name__ == "__main__":
    main()
