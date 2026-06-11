"""SSH 连接封装，通过系统 ssh 命令执行远程操作（完整支持 ~/.ssh/config）。"""
from __future__ import annotations
import asyncio
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import AsyncIterator, Optional


@dataclass
class RunResult:
    returncode: int
    stdout: str
    stderr: str


class RemoteNode:
    """代表一个可通过 SSH 操作的远程节点。"""

    def __init__(self, ssh_host: str, sudo_pass: str):
        self.ssh_host  = ssh_host
        self.sudo_pass = sudo_pass

    async def run(self, cmd: str, check: bool = True, input: Optional[str] = None) -> RunResult:
        """通过系统 ssh 命令在远程执行命令（完整读取 ~/.ssh/config）。"""
        proc = await asyncio.create_subprocess_exec(
            "ssh", "-o", "StrictHostKeyChecking=no",
            "-o", "ConnectTimeout=10",
            self.ssh_host, cmd,
            stdin=asyncio.subprocess.PIPE  if input else asyncio.subprocess.DEVNULL,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdin_data = input.encode() if input else None
        stdout, stderr = await proc.communicate(input=stdin_data)
        result = RunResult(
            returncode=proc.returncode,
            stdout=stdout.decode(errors="replace"),
            stderr=stderr.decode(errors="replace"),
        )
        if check and result.returncode != 0:
            raise RuntimeError(
                f"[{self.ssh_host}] rc={result.returncode}: {cmd!r}\n"
                f"stderr: {result.stderr}"
            )
        return result

    async def sudo(self, cmd: str, check: bool = True) -> RunResult:
        """用 sudo 运行命令，通过 stdin 传密码。"""
        full_cmd = f"echo {self.sudo_pass!r} | sudo -S {cmd}"
        return await self.run(full_cmd, check=check)

    async def upload(self, local_path: str, remote_path: str) -> None:
        """通过 scp 上传文件（自动走 ProxyJump）。"""
        proc = await asyncio.create_subprocess_exec(
            "scp", "-o", "StrictHostKeyChecking=no",
            "-o", "ConnectTimeout=10",
            local_path, f"{self.ssh_host}:{remote_path}",
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        _, stderr = await proc.communicate()
        if proc.returncode != 0:
            raise RuntimeError(
                f"scp 失败 {local_path} → {self.ssh_host}:{remote_path}\n"
                f"{stderr.decode(errors='replace')}"
            )

    async def stream_lines(self, cmd: str) -> AsyncIterator[str]:
        """通过系统 ssh 流式读取远程命令输出。"""
        proc = await asyncio.create_subprocess_exec(
            "ssh", "-o", "StrictHostKeyChecking=no",
            "-o", "ConnectTimeout=10",
            self.ssh_host, cmd,
            stdin=asyncio.subprocess.DEVNULL,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
        )
        try:
            assert proc.stdout is not None
            while True:
                line = await proc.stdout.readline()
                if not line:
                    break
                yield line.decode(errors="replace").rstrip("\n")
        finally:
            if proc.returncode is None:
                proc.terminate()
                try:
                    await asyncio.wait_for(proc.wait(), timeout=5)
                except asyncio.TimeoutError:
                    proc.kill()
                    await proc.wait()

    async def write_file(self, path: str, content: str, sudo: bool = False) -> None:
        """把字符串内容写到远程文件（通过 stdin 传输）。"""
        tmp = f"/tmp/_fb_{abs(hash(path))}"
        # 用 ssh 的 stdin 传内容
        await self.run(f"cat > {tmp}", input=content, check=True)
        if sudo:
            await self.sudo(f"cp {tmp} {path}")
            await self.run(f"rm -f {tmp}")
        else:
            await self.run(f"mv {tmp} {path}")

    async def exists(self, path: str) -> bool:
        r = await self.run(f"test -e {path}", check=False)
        return r.returncode == 0

    async def service_status(self, service: str) -> str:
        r = await self.run(f"systemctl is-active {service}", check=False)
        return r.stdout.strip()

    async def journal(self, service: str, lines: int = 30) -> str:
        r = await self.sudo(
            f"journalctl -u {service} --no-pager -n {lines} 2>/dev/null",
            check=False,
        )
        return r.stdout


@asynccontextmanager
async def connect_all(nodes_cfg, sudo_pass: str):
    """创建所有节点的 RemoteNode，并行做一次连通性测试。"""
    remotes = {n.id: RemoteNode(n.ssh, sudo_pass) for n in nodes_cfg}

    async def _test(node_id, remote):
        r = await remote.run("echo ok", check=False)
        if r.returncode != 0 or r.stdout.strip() != "ok":
            raise RuntimeError(f"连通性测试失败: {r.stderr}")
        return node_id, remote

    tasks = [_test(nid, r) for nid, r in remotes.items()]
    results = await asyncio.gather(*tasks, return_exceptions=True)

    connected = {}
    for r in results:
        if isinstance(r, Exception):
            nid = list(remotes.keys())[results.index(r)]
            print(f"[警告] {nid} 连接失败: {r}")
        else:
            nid, remote = r
            connected[nid] = remote

    yield connected
