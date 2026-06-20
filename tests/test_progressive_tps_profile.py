import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
PROFILE = ROOT / "scripts" / "profiles" / "progressive_tps.json"
BRIDGE_PROFILE = ROOT / "scripts" / "profiles" / "progressive_tps_18vm_bridge.json"


class ProgressiveTpsProfileTest(unittest.TestCase):
    def test_profile_keeps_all_six_experiment_chains_crowdsource(self):
        data = json.loads(PROFILE.read_text(encoding="utf-8"))
        self.assertEqual(sorted(data), [f"child{i}" for i in range(1, 7)])
        self.assertTrue(all(item["scene"] == "Crowdsource" for item in data.values()))
        self.assertEqual(data["child4"]["runtimeBinary"], "fishbone-node-crowdsource-v1")
        self.assertEqual(data["child5"]["runtimeBinary"], "fishbone-node-crowdsource-v2")
        self.assertEqual(data["child6"]["runtimeBinary"], "fishbone-node-crowdsource-v3")

    def test_bridge_profile_uses_short_epoch_runtime_binaries(self):
        data = json.loads(BRIDGE_PROFILE.read_text(encoding="utf-8"))
        profiles = data["chains"] if "chains" in data else data

        self.assertEqual(profiles["child1"]["runtimeBinary"], "fishbone-node-crowdsource-2s-bridge")
        self.assertEqual(profiles["child2"]["runtimeBinary"], "fishbone-node-crowdsource-2s-bridge")
        self.assertEqual(profiles["child3"]["runtimeBinary"], "fishbone-node-crowdsource-2s-bridge")
        self.assertEqual(profiles["child4"]["runtimeBinary"], "fishbone-node-crowdsource-v1-bridge")
        self.assertEqual(profiles["child5"]["runtimeBinary"], "fishbone-node-crowdsource-v2-bridge")
        self.assertEqual(profiles["child6"]["runtimeBinary"], "fishbone-node-crowdsource-v3-bridge")


if __name__ == "__main__":
    unittest.main()
