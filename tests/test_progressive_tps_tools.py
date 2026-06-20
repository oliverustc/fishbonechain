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
WORKER_BURST = ROOT / "scripts" / "worker_burst.js"
CAPACITY_MONITOR = ROOT / "scripts" / "capacity_monitor.js"
BRIDGE_CROWDSOURCE = ROOT / "scripts" / "bridges" / "crowdsource.js"
EPOCH_FINALIZER = ROOT / "scripts" / "finalize_progressive_epochs.js"


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

    def test_runner_resets_stages_with_deploy_control_stop_clean(self):
        script = RUN_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("control_children stop-clean", script)
        self.assertIn("deploy_children", script)
        self.assertIn("n${n}_stop_clean.log", script)
        self.assertIn("n${n}_deploy.log", script)
        self.assertNotIn("reset_child_chains.sh", script)

    def test_runner_and_worker_support_batch_business_submissions(self):
        runner = RUN_SCRIPT.read_text(encoding="utf-8")
        worker = WORKER_BURST.read_text(encoding="utf-8")

        self.assertIn("DEFAULT_BATCH_SIZE", runner)
        self.assertIn("batch_size_for_child", runner)
        self.assertIn("local child_batch_size", runner)
        self.assertIn("--batch-size", runner)
        self.assertIn("batchSize", worker)
        self.assertIn("submitDataBatch", worker)

    def test_runner_initializes_mainchain_and_runs_bridges_for_pressure_measurement(self):
        script = RUN_SCRIPT.read_text(encoding="utf-8")

        self.assertIn("SETUP_MAINCHAIN_FOR_BRIDGE", script)
        self.assertIn("setup_progressive_mainchain.js", script)
        self.assertIn("start_bridges_for_stage", script)
        self.assertIn("BRIDGE_MINER_SURI", script)
        self.assertIn('MINER_SURI="${BRIDGE_MINER_SURI[$child]}"', script)
        self.assertIn("scripts/bridges/crowdsource.js", script)
        self.assertIn("finalize_progressive_epochs.js", script)
        self.assertIn("n${n}_bridge_${child}.log", script)

    def test_capacity_monitor_prefers_business_submission_counter(self):
        monitor = CAPACITY_MONITOR.read_text(encoding="utf-8")

        self.assertIn("acceptedSubmissionCount", monitor)
        self.assertIn("epochSubmissions", monitor)

    def test_capacity_monitor_accumulates_across_epoch_counter_resets(self):
        monitor = CAPACITY_MONITOR.read_text(encoding="utf-8")

        self.assertIn("cumulativeAccepted", monitor)
        self.assertIn("item.subs < tracker.lastSubs", monitor)
        self.assertIn("tracker.cumulativeAccepted >= cfg.cap", monitor)

    def test_crowdsource_bridge_supports_bounded_exit_after_events(self):
        bridge = BRIDGE_CROWDSOURCE.read_text(encoding="utf-8")

        self.assertIn("EXIT_AFTER_EVENTS", bridge)
        self.assertIn("--exit-after-events", bridge)
        self.assertIn("processedCount >= EXIT_AFTER_EVENTS", bridge)

    def test_epoch_finalizer_waits_for_syncing_and_calls_finalize_epoch(self):
        self.assertTrue(EPOCH_FINALIZER.exists())
        content = EPOCH_FINALIZER.read_text(encoding="utf-8")

        self.assertIn("currentEpoch", content)
        self.assertIn("Syncing", content)
        self.assertIn("finalizeEpoch", content)
        self.assertIn("EpochFinalized", content)
        self.assertIn("alreadyFinalized", content)
        self.assertIn("epochIdValue(after) > epochIdValue(before)", content)

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

    def test_summarizer_uses_stage_batch_sizes_for_business_per_extrinsic(self):
        module = load_module(SUMMARY_SCRIPT, "summarize_progressive_tps_batch")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_dir = root / "raw"
            log_dir = root / "logs"
            raw_dir.mkdir()
            log_dir.mkdir()

            (raw_dir / "progressive_tps_n6_child_precise_summary.json").write_text(
                json.dumps(
                    {
                        "hit_summary": {
                            "ws://10.2.2.11:9945": {"accepted_delta": 100, "elapsed_s": 10, "cap_subs": 100},
                            "ws://10.2.2.14:9946": {"accepted_delta": 100, "elapsed_s": 10, "cap_subs": 100},
                            "ws://10.2.2.17:9947": {"accepted_delta": 100, "elapsed_s": 10, "cap_subs": 100},
                            "ws://10.2.2.20:9948": {"accepted_delta": 100, "elapsed_s": 10, "cap_subs": 100},
                            "ws://10.2.3.11:9949": {"accepted_delta": 100, "elapsed_s": 10, "cap_subs": 100},
                            "ws://10.2.3.14:9950": {"accepted_delta": 400, "elapsed_s": 10, "cap_subs": 400},
                        }
                    }
                ),
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n6_stage.txt").write_text(
                "\n".join(
                    [
                        "batch_size_child1=1",
                        "batch_size_child2=1",
                        "batch_size_child3=1",
                        "batch_size_child4=1",
                        "batch_size_child5=1",
                        "batch_size_child6=4",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            row = module.summarize_stage(raw_dir, log_dir, 6, module.DEFAULT_ORDER)

        self.assertEqual(row["submissions_per_extrinsic"], "1.5000")

    def test_summarizer_marks_missing_required_bridge_traffic(self):
        module = load_module(SUMMARY_SCRIPT, "summarize_progressive_tps_required_bridge")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_dir = root / "raw"
            log_dir = root / "logs"
            raw_dir.mkdir()
            log_dir.mkdir()
            (raw_dir / "progressive_tps_n1_child_precise_summary.json").write_text(
                json.dumps({"hit_summary": {"ws://child": {"accepted_delta": 1000, "elapsed_s": 10, "cap_subs": 1000}}}),
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n1_main_blocks.csv").write_text(
                "timestamp,block_number,block_hash,extrinsics_total,bridge_extrinsics,ccmc_digest_calls,fmc_bill_calls,ccmc_events,fmc_events\n"
                "2026-06-20T00:00:00Z,1,0x1,1,0,0,0,0,0\n",
                encoding="utf-8",
            )
            (raw_dir / "progressive_tps_n1_stage.txt").write_text(
                "require_bridge_events=1\nfailed=0\n",
                encoding="utf-8",
            )

            row = module.summarize_stage(raw_dir, log_dir, 1, module.DEFAULT_ORDER)

        self.assertEqual(row["bridge_measurement_status"], "missing_required_bridge_events")

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

    def test_plotter_keeps_secondary_axis_clean_when_bridge_pressure_is_zero(self):
        module = load_module(PLOT_SCRIPT, "plot_progressive_tps_zero_pressure")
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            summary = root / "summary.csv"
            out_dir = root / "figures"
            summary.write_text(
                "\n".join(
                    [
                        "n,stage_key,stage_label,profile_label,active_chains,measurement_source,accepted_submissions,child_window_seconds,aggregate_child_tps,conservative_child_tps,worker_sent,worker_ok,worker_reject,worker_fail,worker_elapsed_seconds,main_window_seconds,main_bridge_events,main_bridge_tps,main_total_extrinsics,main_total_tps,main_bridge_pressure_pct,main_bridge_share_pct,submissions_per_extrinsic",
                        "1,baseline-tuned,部署/出块/RPC 调优,基线调优-1链,child1,precise,1000,5,150,150,1000,1000,0,0,5,10,0,0,20,2,0,0,1",
                        "2,baseline-tuned,部署/出块/RPC 调优,基线调优-2链,child1+child2,precise,2000,6,310,310,2000,2000,0,0,6,10,0,0,20,2,0,0,1",
                        "3,baseline-tuned,部署/出块/RPC 调优,基线调优-3链,child1+child2+child3,precise,3000,7,440,440,3000,3000,0,0,7,10,0,0,20,2,0,0,1",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            captured = {}
            original_save = module.save_figure

            def capture_figure(fig, out_dir, stem, formats):
                fig.canvas.draw()
                captured["fig"] = fig
                return original_save(fig, out_dir, stem, formats)

            module.save_figure = capture_figure
            try:
                module.build_progressive_tps_figure(summary, out_dir, formats=("png",))
            finally:
                module.save_figure = original_save

        ax, ax2 = captured["fig"].axes
        self.assertTrue(all(not tick.tick2line.get_visible() for tick in ax.yaxis.majorTicks))
        self.assertLessEqual(ax2.get_ylim()[1], 0.1)
        self.assertIn("桥接事件=0", [text.get_text() for text in ax.texts])


if __name__ == "__main__":
    unittest.main()
