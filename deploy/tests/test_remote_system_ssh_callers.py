from __future__ import annotations

import asyncio
import sys
from tempfile import TemporaryDirectory
from pathlib import Path
from types import SimpleNamespace
from unittest import IsolatedAsyncioTestCase
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from cmd import deploy as deploy_cmd  # noqa: E402
from cmd import control as control_cmd  # noqa: E402
from cmd import logs as logs_cmd  # noqa: E402
from cmd import status as status_cmd  # noqa: E402
from fishbone.config import filter_config_to_chains  # noqa: E402
from fishbone import remote as remote_mod  # noqa: E402


class FakeRemote:
    def __init__(self) -> None:
        self.commands: list[str] = []
        self.uploads: list[tuple[str, str]] = []
        self.stream_commands: list[str] = []

    async def run(self, cmd: str, check: bool = True):
        self.commands.append(cmd)
        return SimpleNamespace(returncode=0, stdout="", stderr="")

    async def upload(self, local_path: str, remote_path: str) -> None:
        self.uploads.append((local_path, remote_path))

    async def sudo(self, cmd: str, check: bool = True):
        self.commands.append(f"sudo {cmd}")
        return SimpleNamespace(returncode=0, stdout="", stderr="")

    async def stream_lines(self, cmd: str):
        self.stream_commands.append(cmd)
        yield "first line\n"

    async def service_status(self, service: str):
        self.commands.append(f"service_status {service}")
        return "active"


class RemoteSystemSshCallerTests(IsolatedAsyncioTestCase):
    async def test_push_binaries_uses_remote_upload_without_asyncssh_connection(self):
        remote = FakeRemote()
        cfg = SimpleNamespace(
            binary="/remote/bin/fishbone-node",
            chains={
                "main": SimpleNamespace(binary=None),
                "child2": SimpleNamespace(binary="/remote/bin/fishbone-node-2s"),
            },
        )

        with TemporaryDirectory() as tmp:
            bin_dir = Path(tmp)
            (bin_dir / "fishbone-node").write_text("")
            (bin_dir / "fishbone-node-2s").write_text("")

            with patch.object(deploy_cmd, "BIN_DIR", bin_dir):
                await deploy_cmd.push_binaries(remote, cfg, "f1")

            expected = [
                (str(bin_dir / "fishbone-node"), "/remote/bin/fishbone-node.new"),
                (str(bin_dir / "fishbone-node-2s"), "/remote/bin/fishbone-node-2s.new"),
            ]

        self.assertIn("mkdir -p /remote/bin", remote.commands)
        self.assertIn(
            "chmod +x /remote/bin/fishbone-node.new && mv -f /remote/bin/fishbone-node.new /remote/bin/fishbone-node",
            remote.commands,
        )
        self.assertIn(
            "chmod +x /remote/bin/fishbone-node-2s.new && mv -f /remote/bin/fishbone-node-2s.new /remote/bin/fishbone-node-2s",
            remote.commands,
        )
        self.assertEqual(remote.uploads, expected)

    async def test_push_specs_uses_remote_upload_without_asyncssh_connection(self):
        remote = FakeRemote()
        cfg = SimpleNamespace(
            base_dir="/remote/fishbone",
            chains={"main": object(), "child1": object()},
        )

        with TemporaryDirectory() as tmp:
            specs_dir = Path(tmp)
            (specs_dir / "main-custom-raw.json").write_text("{}")
            (specs_dir / "child1-custom-raw.json").write_text("{}")

            with patch.object(deploy_cmd, "SPECS_DIR", specs_dir):
                await deploy_cmd.push_specs(remote, cfg, "f1")

            expected = [
                (str(specs_dir / "child1-custom-raw.json"), "/remote/fishbone/specs/child1-custom-raw.json"),
                (str(specs_dir / "main-custom-raw.json"), "/remote/fishbone/specs/main-custom-raw.json"),
            ]

        self.assertIn("mkdir -p /remote/fishbone/specs", remote.commands)
        self.assertEqual(remote.uploads, expected)

    def test_filter_config_to_chains_limits_chain_map_and_node_roles(self):
        cfg = SimpleNamespace(
            chains={
                "main": object(),
                "child1": object(),
                "child2": object(),
                "child6": object(),
            },
            nodes=[
                SimpleNamespace(id="f1", roles=["main", "child1", "child2"]),
                SimpleNamespace(id="f4", roles=["main", "child6"]),
                SimpleNamespace(id="f6", roles=["main"]),
            ],
        )

        filtered = deploy_cmd.filter_config_to_chains(cfg, {"main", "child6"})

        self.assertEqual(list(filtered.chains), ["main", "child6"])
        self.assertEqual(filtered.nodes[0].roles, ["main"])
        self.assertEqual(filtered.nodes[1].roles, ["main", "child6"])
        self.assertEqual(filtered.nodes[2].roles, ["main"])
        self.assertEqual(cfg.nodes[0].roles, ["main", "child1", "child2"])

    def test_shared_filter_config_to_chains_preserves_original_roles(self):
        cfg = SimpleNamespace(
            chains={"main": object(), "child1": object(), "child6": object()},
            nodes=[
                SimpleNamespace(id="f1", roles=["main", "child1", "child6"]),
                SimpleNamespace(id="f8", roles=["main"]),
            ],
        )

        filtered = filter_config_to_chains(cfg, {"main", "child6"})

        self.assertEqual(list(filtered.chains), ["main", "child6"])
        self.assertEqual(filtered.nodes[0].roles, ["main", "child6"])
        self.assertEqual(filtered.nodes[1].roles, ["main"])
        self.assertEqual(cfg.nodes[0].roles, ["main", "child1", "child6"])

    async def test_clean_chain_data_preserves_node_key_and_removes_chain_state(self):
        remote = FakeRemote()
        cfg = SimpleNamespace(base_dir="/remote/fishbone", log_dir="/remote/fishbone/logs")

        await control_cmd.clean_chain_data(remote, cfg, "child6", clean_logs=True)

        joined = "\n".join(remote.commands)
        self.assertIn("/remote/fishbone/child6", joined)
        self.assertIn("! -name node-key", joined)
        self.assertIn("rm -rf", joined)
        self.assertIn("/remote/fishbone/logs/child6.log", joined)

    async def test_stop_all_child_services_scans_units_instead_of_config_roles(self):
        remote = FakeRemote()

        await control_cmd.stop_all_child_services(remote)

        joined = "\n".join(remote.commands)
        self.assertIn("fishbone-child*.service", joined)
        self.assertIn("systemctl stop", joined)
        self.assertNotIn("fishbone-main", joined)

    def test_remote_ssh_args_use_user_config_to_avoid_system_config_breakage(self):
        args = remote_mod.ssh_base_args()

        self.assertIn("-F", args)
        self.assertIn(str(Path.home() / ".ssh" / "config"), args)

    async def test_stream_log_uses_remote_stream_lines_without_asyncssh_connection(self):
        remote = FakeRemote()
        stop_event = asyncio.Event()

        await logs_cmd.stream_log(remote, "f1", "/tmp/main.log", stop_event, lines=7)

        self.assertEqual(remote.stream_commands, ["tail -n 7 -f /tmp/main.log"])

    async def test_rpc_query_defaults_to_local_curl_without_gateway(self):
        calls = []

        async def fake_local_curl(ip: str, port: int, payload: str):
            calls.append((ip, port, payload))
            return SimpleNamespace(returncode=0, stdout='{"result":{"number":"0xa"}}', stderr="")

        with patch.object(status_cmd, "run_local_rpc_curl", fake_local_curl, create=True):
            result = await status_cmd.rpc_query("10.2.2.11", 9944, "chain_getHeader")

        self.assertEqual(result["result"]["number"], "0xa")
        self.assertEqual(calls, [("10.2.2.11", 9944, '{"id": 1, "jsonrpc": "2.0", "method": "chain_getHeader", "params": []}')])

    async def test_rpc_query_uses_gateway_only_when_requested(self):
        gateway = FakeRemote()
        local_calls = []

        async def fake_local_curl(ip: str, port: int, payload: str):
            local_calls.append((ip, port, payload))
            return SimpleNamespace(returncode=0, stdout='{"result":"local"}', stderr="")

        async def fake_gateway_run(cmd: str, check: bool = True):
            gateway.commands.append(cmd)
            return SimpleNamespace(returncode=0, stdout='{"result":"gateway"}', stderr="")

        gateway.run = fake_gateway_run

        with patch.object(status_cmd, "run_local_rpc_curl", fake_local_curl, create=True):
            result = await status_cmd.rpc_query("10.2.2.11", 9944, "chain_getHeader", gateway=gateway)

        self.assertEqual(result["result"], "gateway")
        self.assertEqual(local_calls, [])
        self.assertIn("http://10.2.2.11:9944", gateway.commands[0])

    async def test_get_node_info_reports_all_requested_configured_chains(self):
        cfg = SimpleNamespace(
            chains={
                "main": SimpleNamespace(rpc_port=9944),
                "child6": SimpleNamespace(rpc_port=9950),
            },
        )
        node = SimpleNamespace(id="f1", ip="10.2.2.11", roles=["main", "child6"])
        remote = FakeRemote()
        rpc_calls = []

        async def fake_rpc_query(ip: str, port: int, method: str, gateway=None):
            rpc_calls.append((ip, port, method, gateway))
            if method == "chain_getHeader":
                return {"result": {"number": "0xa"}}
            if method == "system_health":
                return {"result": {"peers": 3}}
            if method == "chain_getFinalizedHead":
                return {"result": "0x1234567890"}
            return {}

        with patch.object(status_cmd, "rpc_query", fake_rpc_query, create=True):
            info = await status_cmd.get_node_info(remote, node, cfg, ["main", "child6"])

        self.assertEqual(info["main"]["block"], "10")
        self.assertEqual(info["child6"]["block"], "10")
        self.assertEqual(remote.commands, ["service_status fishbone-main", "service_status fishbone-child6"])
        self.assertEqual({call[1] for call in rpc_calls}, {9944, 9950})
