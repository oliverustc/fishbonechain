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
from cmd import logs as logs_cmd  # noqa: E402


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

    async def stream_lines(self, cmd: str):
        self.stream_commands.append(cmd)
        yield "first line\n"


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
                (str(bin_dir / "fishbone-node"), "/remote/bin/fishbone-node"),
                (str(bin_dir / "fishbone-node-2s"), "/remote/bin/fishbone-node-2s"),
            ]

        self.assertIn("mkdir -p /remote/bin", remote.commands)
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

    async def test_stream_log_uses_remote_stream_lines_without_asyncssh_connection(self):
        remote = FakeRemote()
        stop_event = asyncio.Event()

        await logs_cmd.stream_log(remote, "f1", "/tmp/main.log", stop_event, lines=7)

        self.assertEqual(remote.stream_commands, ["tail -n 7 -f /tmp/main.log"])
