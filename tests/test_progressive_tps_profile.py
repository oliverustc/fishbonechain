import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
PROFILE = ROOT / "scripts" / "profiles" / "progressive_tps.json"


class ProgressiveTpsProfileTest(unittest.TestCase):
    def test_profile_keeps_all_six_experiment_chains_crowdsource(self):
        data = json.loads(PROFILE.read_text(encoding="utf-8"))
        self.assertEqual(sorted(data), [f"child{i}" for i in range(1, 7)])
        self.assertTrue(all(item["scene"] == "Crowdsource" for item in data.values()))
        self.assertEqual(data["child4"]["runtimeBinary"], "fishbone-node-crowdsource-v1")
        self.assertEqual(data["child5"]["runtimeBinary"], "fishbone-node-crowdsource-v2")
        self.assertEqual(data["child6"]["runtimeBinary"], "fishbone-node-crowdsource-v3")


if __name__ == "__main__":
    unittest.main()
