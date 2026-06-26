#!/usr/bin/env python3
"""Summarize progressive child-chain TPS and mainchain bridge pressure runs."""

from __future__ import annotations

import argparse
import csv
import json
import re
from dataclasses import dataclass
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

DEFAULT_ORDER = ["child1", "child2", "child3", "child4", "child5", "child6"]

STAGE_ROWS = {
    1: ("baseline-tuned", "部署/出块/RPC 调优", "基线调优-1链"),
    2: ("baseline-tuned", "部署/出块/RPC 调优", "基线调优-2链"),
    3: ("baseline-tuned", "部署/出块/RPC 调优", "基线调优-3链"),
    4: ("runtime-v1", "递进式子链优化", "部分运行时优化"),
    5: ("runtime-v2", "递进式子链优化", "索引/聚合存储优化"),
    6: ("runtime-v3", "递进式子链优化", "批量/完整热路径优化"),
}


@dataclass(frozen=True)
class WorkerStats:
    sent: int = 0
    ok: int = 0
    reject: int = 0
    fail: int = 0
    elapsed_s: float = 0.0


def parse_ts(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def as_float(value: object, default: float = 0.0) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def as_int(value: object, default: int = 0) -> int:
    return int(round(as_float(value, float(default))))


def chain_name(value: str) -> str:
    return CHAIN_NAMES.get(value, value)


def active_chains_for(n: int, order: list[str]) -> list[str]:
    if n < 1 or n > len(order):
        raise ValueError(f"N must be in 1..{len(order)}, got {n}")
    return order[:n]


def load_capacity_summary(raw_dir: Path, n: int) -> tuple[dict[str, dict[str, float]], str]:
    candidates = [
        raw_dir / f"progressive_tps_n{n}_child_precise_summary.json",
        raw_dir / f"exp_capacity_n{n}_precise_summary.json",
    ]
    for path in candidates:
        if path.exists():
            data = json.loads(path.read_text())
            return parse_precise_summary(data), path.name
    return {}, "missing"


def merge_precise_csv_partials(raw_dir: Path, n: int, chain_stats: dict[str, dict[str, float]]) -> None:
    candidates = [
        raw_dir / f"progressive_tps_n{n}_child_precise.csv",
        raw_dir / f"exp_capacity_n{n}_precise.csv",
    ]
    path = next((item for item in candidates if item.exists()), None)
    if path is None:
        return

    last_by_chain: dict[str, dict[str, str]] = {}
    with path.open(newline="") as f:
        for row in csv.DictReader(f):
            last_by_chain[chain_name(row["chain_url"])] = row

    for chain, row in last_by_chain.items():
        current = chain_stats.get(chain)
        if current and current.get("accepted_delta", 0.0) > 0:
            continue
        delta = as_float(row.get("delta_from_initial"))
        elapsed_s = max(as_float(row.get("elapsed_s")), 0.001)
        chain_stats[chain] = {
            "accepted_delta": delta,
            "elapsed_s": elapsed_s,
            "accepted_tps": delta / elapsed_s,
            "hit_cap": as_float(row.get("hit_cap")),
        }


def parse_precise_summary(data: dict) -> dict[str, dict[str, float]]:
    out: dict[str, dict[str, float]] = {}
    for url, item in data.get("hit_summary", {}).items():
        name = chain_name(url)
        if not item:
            out[name] = {
                "accepted_delta": 0.0,
                "elapsed_s": 0.0,
                "accepted_tps": 0.0,
                "hit_cap": 0.0,
            }
            continue
        accepted_delta = as_float(item.get("accepted_delta"))
        elapsed_s = max(as_float(item.get("elapsed_s")), 0.001)
        out[name] = {
            "accepted_delta": accepted_delta,
            "elapsed_s": elapsed_s,
            "accepted_tps": accepted_delta / elapsed_s,
            "hit_cap": 1.0 if as_float(item.get("cap_subs")) > 0 else 0.0,
        }
    return out


def summarize_worker_logs(log_dir: Path, n: int) -> WorkerStats:
    final_re = re.compile(
        r"elapsed=([0-9.]+)s.*sent=([0-9]+).*ok=([0-9]+).*reject=([0-9]+).*fail=([0-9]+).*final=true"
    )
    sent = ok = reject = fail = 0
    elapsed_s = 0.0

    for path in sorted(log_dir.glob(f"n{n}_burst_*.log")):
        last = None
        with path.open(errors="replace") as f:
            for line in f:
                match = final_re.search(line)
                if match:
                    last = match
        if not last:
            continue
        elapsed_s = max(elapsed_s, float(last.group(1)))
        sent += int(last.group(2))
        ok += int(last.group(3))
        reject += int(last.group(4))
        fail += int(last.group(5))

    return WorkerStats(sent=sent, ok=ok, reject=reject, fail=fail, elapsed_s=elapsed_s)


def summarize_main_blocks(raw_dir: Path, n: int) -> dict[str, float]:
    candidates = [
        raw_dir / f"progressive_tps_n{n}_main_blocks.csv",
        raw_dir / f"exp_scale_main_n{n}_main_blocks.csv",
    ]
    path = next((item for item in candidates if item.exists()), None)
    if path is None:
        return {
            "main_window_seconds": 0.0,
            "main_bridge_events": 0.0,
            "main_bridge_runtime_events": 0.0,
            "main_bridge_tps": 0.0,
            "main_total_extrinsics": 0.0,
            "main_total_tps": 0.0,
            "main_bridge_share_pct": 0.0,
        }

    timestamps = []
    bridge_total = 0
    bridge_runtime_events = 0
    extrinsics_total = 0
    with path.open(newline="") as f:
        for row in csv.DictReader(f):
            try:
                timestamps.append(parse_ts(row["timestamp"]))
            except (KeyError, ValueError):
                pass
            bridge_total += as_int(row.get("bridge_extrinsics"))
            extrinsics_total += as_int(row.get("extrinsics_total"))
            if "ccmc_events" in row or "fmc_events" in row:
                bridge_runtime_events += as_int(row.get("ccmc_events")) + as_int(row.get("fmc_events"))

    if len(timestamps) >= 2:
        window_s = max((max(timestamps) - min(timestamps)).total_seconds(), 0.001)
    else:
        window_s = 0.0

    main_bridge_tps = bridge_total / window_s if window_s > 0 else 0.0
    main_total_tps = extrinsics_total / window_s if window_s > 0 else 0.0
    bridge_share = (bridge_total / extrinsics_total * 100) if extrinsics_total else 0.0
    return {
        "main_window_seconds": window_s,
        "main_bridge_events": float(bridge_total),
        "main_bridge_runtime_events": float(bridge_runtime_events),
        "main_bridge_tps": main_bridge_tps,
        "main_total_extrinsics": float(extrinsics_total),
        "main_total_tps": main_total_tps,
        "main_bridge_share_pct": bridge_share,
    }


def load_stage_values(raw_dir: Path, n: int) -> dict[str, str]:
    path = raw_dir / f"progressive_tps_n{n}_stage.txt"
    values: dict[str, str] = {}
    if not path.exists():
        return values

    with path.open(errors="replace") as f:
        for line in f:
            key, sep, value = line.strip().partition("=")
            if sep:
                values[key] = value
    return values


def load_stage_batch_sizes(raw_dir: Path, n: int, active: list[str]) -> dict[str, float]:
    batch_sizes = {chain: 1.0 for chain in active}
    values = load_stage_values(raw_dir, n)
    for key, value in values.items():
        if not key.startswith("batch_size_") or not value:
            continue
        chain = key.removeprefix("batch_size_")
        if chain in batch_sizes:
            batch_sizes[chain] = max(as_float(value, 1.0), 1.0)
    return batch_sizes


def bridge_measurement_status(stage_values: dict[str, str], main: dict[str, float]) -> str:
    required = stage_values.get("require_bridge_events") == "1"
    if required and main["main_bridge_events"] <= 0:
        return "missing_required_bridge_events"
    if main["main_bridge_events"] > 0:
        return "observed"
    return "not_required_or_not_observed"


def estimate_business_submissions_per_extrinsic(
    chain_stats: dict[str, dict[str, float]],
    active: list[str],
    batch_sizes: dict[str, float],
) -> float:
    accepted = 0.0
    estimated_extrinsics = 0.0
    for chain in active:
        accepted_delta = chain_stats.get(chain, {}).get("accepted_delta", 0.0)
        batch_size = max(batch_sizes.get(chain, 1.0), 1.0)
        accepted += accepted_delta
        estimated_extrinsics += accepted_delta / batch_size
    return accepted / estimated_extrinsics if estimated_extrinsics > 0 else 1.0


def mainchain_capacity_occupancy(
    n: int,
    mainchain_max_tps: float,
    bridge_epoch_seconds: float,
    bridge_extrinsics_per_epoch: float,
) -> tuple[float, float]:
    if bridge_epoch_seconds <= 0:
        return 0.0, 0.0
    theoretical_bridge_tps = n * bridge_extrinsics_per_epoch / bridge_epoch_seconds
    occupancy_pct = (theoretical_bridge_tps / mainchain_max_tps * 100) if mainchain_max_tps > 0 else 0.0
    return theoretical_bridge_tps, occupancy_pct


def summarize_stage(
    raw_dir: Path,
    log_dir: Path,
    n: int,
    order: list[str],
    *,
    mainchain_max_tps: float = 0.0,
    bridge_epoch_seconds: float = 120.0,
    bridge_extrinsics_per_epoch: float = 2.0,
) -> dict[str, str]:
    active = active_chains_for(n, order)
    chain_stats, source = load_capacity_summary(raw_dir, n)
    merge_precise_csv_partials(raw_dir, n, chain_stats)
    worker = summarize_worker_logs(log_dir, n)
    main = summarize_main_blocks(raw_dir, n)
    stage_values = load_stage_values(raw_dir, n)
    batch_sizes = load_stage_batch_sizes(raw_dir, n, active)

    accepted = sum(chain_stats.get(chain, {}).get("accepted_delta", 0.0) for chain in active)
    max_window_s = max((chain_stats.get(chain, {}).get("elapsed_s", 0.0) for chain in active), default=0.0)
    sum_chain_tps = sum(chain_stats.get(chain, {}).get("accepted_tps", 0.0) for chain in active)
    conservative_tps = accepted / max_window_s if max_window_s > 0 else 0.0

    if not chain_stats and worker.ok and worker.elapsed_s > 0:
        accepted = float(worker.ok)
        max_window_s = worker.elapsed_s
        sum_chain_tps = accepted / worker.elapsed_s
        conservative_tps = sum_chain_tps
        source = "worker_logs"

    main_bridge_tps = main["main_bridge_tps"]
    pressure_pct = (main_bridge_tps / sum_chain_tps * 100) if sum_chain_tps > 0 else 0.0
    theoretical_bridge_tps, occupancy_pct = mainchain_capacity_occupancy(
        n,
        mainchain_max_tps,
        bridge_epoch_seconds,
        bridge_extrinsics_per_epoch,
    )
    stage_key, stage_label, profile_label = STAGE_ROWS[n]
    submissions_per_extrinsic = estimate_business_submissions_per_extrinsic(chain_stats, active, batch_sizes)

    return {
        "n": str(n),
        "stage_key": stage_key,
        "stage_label": stage_label,
        "profile_label": profile_label,
        "active_chains": "+".join(active),
        "measurement_source": source,
        "accepted_submissions": f"{accepted:.0f}",
        "child_window_seconds": f"{max_window_s:.3f}",
        "aggregate_child_tps": f"{sum_chain_tps:.4f}",
        "conservative_child_tps": f"{conservative_tps:.4f}",
        "worker_sent": str(worker.sent),
        "worker_ok": str(worker.ok),
        "worker_reject": str(worker.reject),
        "worker_fail": str(worker.fail),
        "worker_elapsed_seconds": f"{worker.elapsed_s:.3f}",
        "main_window_seconds": f"{main['main_window_seconds']:.3f}",
        "main_bridge_events": f"{main['main_bridge_events']:.0f}",
        "main_bridge_runtime_events": f"{main['main_bridge_runtime_events']:.0f}",
        "main_bridge_tps": f"{main_bridge_tps:.4f}",
        "main_total_extrinsics": f"{main['main_total_extrinsics']:.0f}",
        "main_total_tps": f"{main['main_total_tps']:.4f}",
        "main_bridge_pressure_pct": f"{pressure_pct:.4f}",
        "main_bridge_share_pct": f"{main['main_bridge_share_pct']:.4f}",
        "bridge_measurement_status": bridge_measurement_status(stage_values, main),
        "submissions_per_extrinsic": f"{submissions_per_extrinsic:.4f}",
        "mainchain_max_tps": f"{mainchain_max_tps:.4f}",
        "bridge_epoch_seconds": f"{bridge_epoch_seconds:.3f}",
        "bridge_extrinsics_per_epoch": f"{bridge_extrinsics_per_epoch:.3f}",
        "theoretical_bridge_tps": f"{theoretical_bridge_tps:.4f}",
        "mainchain_capacity_occupancy_pct": f"{occupancy_pct:.4f}",
    }


def write_summary(rows: list[dict[str, str]], out: Path) -> None:
    fieldnames = [
        "n",
        "stage_key",
        "stage_label",
        "profile_label",
        "active_chains",
        "measurement_source",
        "accepted_submissions",
        "child_window_seconds",
        "aggregate_child_tps",
        "conservative_child_tps",
        "worker_sent",
        "worker_ok",
        "worker_reject",
        "worker_fail",
        "worker_elapsed_seconds",
        "main_window_seconds",
        "main_bridge_events",
        "main_bridge_runtime_events",
        "main_bridge_tps",
        "main_total_extrinsics",
        "main_total_tps",
        "main_bridge_pressure_pct",
        "main_bridge_share_pct",
        "bridge_measurement_status",
        "submissions_per_extrinsic",
        "mainchain_max_tps",
        "bridge_epoch_seconds",
        "bridge_extrinsics_per_epoch",
        "theoretical_bridge_tps",
        "mainchain_capacity_occupancy_pct",
    ]
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--runs", "--raw-dir", dest="raw_dir", required=True, help="Directory with progressive raw files")
    parser.add_argument("--log-dir", help="Directory with n*_burst_*.log files; defaults to --runs")
    parser.add_argument(
        "--out",
        default="docs/experiments/progressive_tps/progressive_tps_summary.csv",
        help="Output summary CSV path",
    )
    parser.add_argument("--order", default=",".join(DEFAULT_ORDER), help="Comma-separated child-chain order")
    parser.add_argument("--n-start", type=int, default=1)
    parser.add_argument("--n-end", type=int, default=6)
    parser.add_argument("--mainchain-max-tps", type=float, default=0.0)
    parser.add_argument("--bridge-epoch-seconds", type=float, default=120.0)
    parser.add_argument("--bridge-extrinsics-per-epoch", type=float, default=2.0)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    raw_dir = Path(args.raw_dir)
    log_dir = Path(args.log_dir) if args.log_dir else raw_dir
    out = Path(args.out)
    order = [item.strip() for item in args.order.split(",") if item.strip()]
    rows = [
        summarize_stage(
            raw_dir,
            log_dir,
            n,
            order,
            mainchain_max_tps=args.mainchain_max_tps,
            bridge_epoch_seconds=args.bridge_epoch_seconds,
            bridge_extrinsics_per_epoch=args.bridge_extrinsics_per_epoch,
        )
        for n in range(args.n_start, args.n_end + 1)
    ]
    write_summary(rows, out)
    print(f"[saved] {out} ({len(rows)} rows)")


if __name__ == "__main__":
    main()
