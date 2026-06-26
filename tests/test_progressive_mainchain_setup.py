import json
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "setup_progressive_mainchain.js"


class ProgressiveMainchainSetupTest(unittest.TestCase):
    def test_dry_run_lists_child_registration_miner_join_and_fmc_tasks(self):
        with tempfile.TemporaryDirectory() as tmp:
            profile = Path(tmp) / "profile.json"
            profile.write_text(
                json.dumps(
                    {
                        "main": {"scene": "PlatformOnly"},
                        "child1": {
                            "chainId": 0,
                            "scene": "Crowdsource",
                            "settlement": "FmcTaskBill",
                            "validators": ["f1", "f2", "f3"],
                            "taskId": 0,
                            "budgetPerEpochUnit": "1500",
                            "description": "Progressive TPS baseline child 1",
                        },
                        "child2": {
                            "chainId": 1,
                            "scene": "Crowdsource",
                            "settlement": "FmcTaskBill",
                            "validators": ["f4", "f5", "f6"],
                            "taskId": 1,
                            "budgetPerEpochUnit": "1500",
                            "description": "Progressive TPS baseline child 2",
                        },
                    }
                ),
                encoding="utf-8",
            )

            result = subprocess.run(
                [
                    "node",
                    str(SCRIPT),
                    "--profile-file",
                    str(profile),
                    "--chains",
                    "child1,child2",
                    "--dry-run",
                ],
                cwd=ROOT,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("registerChildChain chain_id=0", result.stdout)
        self.assertIn("joinChildChain chain_id=0 validator=f1", result.stdout)
        self.assertIn("createTask task_id=0 chain_id=0 budget_unit=1500", result.stdout)
        self.assertIn("activateTask task_id=0", result.stdout)
        self.assertIn("registerChildChain chain_id=1", result.stdout)


if __name__ == "__main__":
    unittest.main()
