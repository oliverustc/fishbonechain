import importlib.util
import unittest
from pathlib import Path

from deploy.fishbone.config import apply_chain_profile_overrides, load


ROOT = Path(__file__).resolve().parents[1]
PROFILE = ROOT / "scripts" / "profiles" / "progressive_tps.json"
CONFIG = ROOT / "deploy" / "config.toml"
GEN_SPECS = ROOT / "scripts" / "gen_child_specs.py"


def load_gen_specs_module():
    spec = importlib.util.spec_from_file_location("gen_child_specs", GEN_SPECS)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class ProgressiveDeployProfileTest(unittest.TestCase):
    def test_deploy_profile_overrides_runtime_binaries_without_mutating_default(self):
        cfg = load(CONFIG)
        progressive = apply_chain_profile_overrides(cfg, PROFILE)

        self.assertTrue(cfg.chains["child6"].binary.endswith("fishbone-node-data-trade"))
        self.assertTrue(progressive.chains["child4"].binary.endswith("fishbone-node-crowdsource-v1"))
        self.assertTrue(progressive.chains["child5"].binary.endswith("fishbone-node-crowdsource-v2"))
        self.assertTrue(progressive.chains["child6"].binary.endswith("fishbone-node-crowdsource-v3"))

    def test_spec_generator_uses_same_progressive_profile(self):
        module = load_gen_specs_module()
        chains = module.apply_profile_overrides(module.chain_configs(), PROFILE)
        by_name = {cfg["name"]: cfg for cfg in chains}

        self.assertEqual(by_name["child6"]["binary"].name, "fishbone-node-crowdsource-v3")
        self.assertEqual(by_name["child6"]["profile"]["scene"], "Crowdsource")
        self.assertEqual(by_name["child6"]["profile"]["settlement"], "FmcTaskBill")


if __name__ == "__main__":
    unittest.main()
