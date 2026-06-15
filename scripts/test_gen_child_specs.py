from __future__ import annotations

import unittest
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).parent))
import gen_child_specs


class ChainProfilePatchTests(unittest.TestCase):
    def test_inject_chain_profile_uses_runtime_profile_field_names(self):
        spec = {"genesis": {"runtimeGenesis": {"patch": {}}}}
        profile = {
            "chainId": 5,
            "scene": "DataTrade",
            "settlement": "MainEscrow",
            "paramsHash": "0x" + "00" * 32,
        }

        gen_child_specs.inject_chain_profile(spec, profile)

        self.assertEqual(
            spec["genesis"]["runtimeGenesis"]["patch"]["chainProfile"],
            {
                "profile": {
                    "chain_id": 5,
                    "scene": "DataTrade",
                    "settlement": "MainEscrow",
                    "params_hash": "0x" + "00" * 32,
                },
            },
        )
        self.assertNotIn("chain_profile", spec["genesis"]["runtimeGenesis"]["patch"])


if __name__ == "__main__":
    unittest.main()
