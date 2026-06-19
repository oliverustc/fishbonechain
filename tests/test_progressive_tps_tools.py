import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SUMMARY_SCRIPT = ROOT / "scripts" / "summarize_progressive_tps.py"
PLOT_SCRIPT = ROOT / "scripts" / "plot_progressive_tps.py"
RUN_SCRIPT = ROOT / "scripts" / "run_exp_progressive_tps.sh"


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


class ProgressiveTpsToolsTest(unittest.TestCase):
    def test_runner_has_child_service_preflight_controls(self):
        script = RUN_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("STOP_ALL_CHILDREN_BEFORE_RUN", script)
        self.assertIn("START_ACTIVE_CHILDREN_EACH_STAGE", script)
        self.assertIn("stop-all-children", script)
        self.assertIn("control_children start", script)
        self.assertIn("--chains \"$chains_csv\"", script)

    def test_summarizer_combines_child_tps_and_mainchain_pressure(self):
        module = load_module(SUMMARY_SCRIPT, "summarize_progressive_tps")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_dir = root / "raw"
            log_dir = root / "logs"
            out = root / "progressive_tps_summary.csv"
            raw_dir.mkdir()
            log_dir.mkdir()

            (raw_dir / "progressive_tps_n1_child_precise_summary.json").write_text(
                json.dumps(
                    {
                        "hit_summary": {
                            "ws://10.2.2.11:9945": {
                                "accepted_delta": 1000,
                                "elapsed_s": 5,
                                "cap_subs": 1000,
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n1_main_blocks.csv").write_text(
                "\n".join(
                    [
                        "timestamp,block_number,block_hash,extrinsics_total,bridge_extrinsics,ccmc_digest_calls,fmc_bill_calls,ccmc_events,fmc_events",
                        "2026-06-18T00:00:00+00:00,1,0x1,10,1,1,0,3,0",
                        "2026-06-18T00:00:10+00:00,2,0x2,10,1,1,0,3,0",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            (log_dir / "n1_burst_child1.log").write_text(
                "[burst 2026-06-18T00:00:10.000Z] elapsed=10.0s sent=1010 ok=1000 reject=5 fail=5 inflight=0 okTPS=100.0 successRate=99.0% final=true\n",
                encoding="utf-8",
            )

            rows = [module.summarize_stage(raw_dir, log_dir, 1, module.DEFAULT_ORDER)]
            module.write_summary(rows, out)

            content = out.read_text(encoding="utf-8")

        self.assertIn("aggregate_child_tps", content)
        self.assertIn("200.0000", content)
        self.assertIn("0.2000", content)
        self.assertIn("0.1000", content)

    def test_plotter_builds_single_combined_figure(self):
        module = load_module(PLOT_SCRIPT, "plot_progressive_tps")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            summary = root / "summary.csv"
            out_dir = root / "figures"
            summary.write_text(
                "\n".join(
                    [
                        "n,stage_key,stage_label,profile_label,active_chains,measurement_source,accepted_submissions,child_window_seconds,aggregate_child_tps,conservative_child_tps,worker_sent,worker_ok,worker_reject,worker_fail,worker_elapsed_seconds,main_window_seconds,main_bridge_events,main_bridge_tps,main_total_extrinsics,main_total_tps,main_bridge_pressure_pct,main_bridge_share_pct,submissions_per_extrinsic",
                        "1,baseline-tuned,部署/出块/RPC 调优,基线调优-1链,child1,precise,1000,5,200,200,1000,1000,0,0,5,10,2,0.2,20,2,0.1,10,1",
                        "2,baseline-tuned,部署/出块/RPC 调优,基线调优-2链,child1+child2,precise,2400,6,400,400,2400,2400,0,0,6,10,2,0.2,20,2,0.05,10,1",
                        "3,baseline-tuned,部署/出块/RPC 调优,基线调优-3链,child1+child2+child3,precise,3600,6,600,600,3600,3600,0,0,6,10,2,0.2,20,2,0.0333,10,1",
                        "4,runtime-v1,递进式子链优化,部分运行时优化,child1+child2+child3+child4,precise,5600,7,800,800,5600,5600,0,0,7,10,2,0.2,20,2,0.025,10,1",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            outputs = module.build_progressive_tps_figure(summary, out_dir, formats=("png",))
            names = [path.name for path in outputs]

        self.assertEqual(names, ["progressive_tps_mainchain_load.png"])


if __name__ == "__main__":
    unittest.main()
