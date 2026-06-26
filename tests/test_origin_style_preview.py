import importlib.util
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "plot_origin_style_preview.py"
CAPITAL_SCRIPT = ROOT / "scripts" / "plot_capital_capacity.py"


def load_module():
    spec = importlib.util.spec_from_file_location("plot_origin_style_preview", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def load_capital_module():
    spec = importlib.util.spec_from_file_location("plot_capital_capacity", CAPITAL_SCRIPT)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class OriginStylePreviewTest(unittest.TestCase):
    def test_isolation_series_uses_chinese_names_and_gain_labels(self):
        module = load_module()
        rows = [
            {
                "scenario": "a",
                "scenario_name": "快递配送",
                "single_chain_success_rate": "12.46",
                "dedicated_chain_success_rate": "100.00",
                "improvement_x": "8.03",
            }
        ]

        series = module.build_isolation_series(rows)

        self.assertEqual(series["labels"], ["快递配送"])
        self.assertEqual(series["gain_labels"], ["8.03x"])

    def test_builds_expected_origin_style_figures_only(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            data_dir = root / "data"
            out_dir = root / "out"
            data_dir.mkdir()

            (data_dir / "exp_capacity_summary.csv").write_text(
                "\n".join(
                    [
                        "n,sum_individual_chain_tps,aggregate_chain_accepted_tps",
                        "1,88.69,88.69",
                        "3,264.58,258.10",
                        "6,396.12,381.27",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            (data_dir / "exp_isolation_summary.csv").write_text(
                "\n".join(
                    [
                        "scenario,single_chain_success_rate,dedicated_chain_success_rate",
                        "data_trade,63.5,100.0",
                        "crowdsource,59.1,100.0",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            (data_dir / "exp_e_fund_state_v5.csv").write_text(
                "\n".join(
                    [
                        "timestamp,task_locked_unit,baseline_locked_unit",
                        "2026-06-18T00:00:00+00:00,200,1000",
                        "2026-06-18T00:05:00+00:00,150,1000",
                        "2026-06-18T00:10:00+00:00,100,0",
                        "2026-06-18T00:15:00+00:00,0,0",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )

            outputs = module.build_origin_style_previews(data_dir, out_dir, formats=("png",))
            names = sorted(path.name for path in outputs)

        self.assertEqual(
            names,
            [
                "origin_style_7a_liquidity_ratio.png",
                "origin_style_capacity_scaling.png",
                "origin_style_isolation_comparison.png",
            ],
        )
        self.assertNotIn("origin_style_scale_mainchain_load.png", names)

    def test_capital_capacity_groups_are_derived_from_horizon_ratios(self):
        module = load_capital_module()
        rows = [
            {"horizon_epochs": "3", "locked_reduction_x": "3.00"},
            {"horizon_epochs": "10", "locked_reduction_x": "10.00"},
            {"horizon_epochs": "20", "locked_reduction_x": "20.00"},
        ]

        series = module.build_capacity_group_series(rows)

        self.assertEqual(series["horizons"], [3, 10, 20])
        self.assertEqual(series["traditional_groups"], [1, 1, 1])
        self.assertEqual(series["fishbone_groups"], [3, 10, 20])


if __name__ == "__main__":
    unittest.main()
