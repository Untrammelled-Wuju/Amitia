#!/usr/bin/env python3
"""Synchronize/check the standalone public Go Game Plugin SDK.

The backend remains the canonical implementation used by the host. This script
copies public protocol implementation files plus the canonical SDK implementation and
portable public-contract tests, rewrites module import paths, and deliberately excludes
backend-internal packages and repository-local test fixtures.
"""
from __future__ import annotations

import argparse
import hashlib
import shutil
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
BACKEND_PROTOCOL = ROOT / "backend/pkg/gameplugin/protocol"
BACKEND_SDK = ROOT / "backend/pkg/gameplugin/sdk/go"
PUBLIC = ROOT / "sdk/game-plugin-go"

GO_MOD = "module github.com/u-ai/game-plugin-sdk-go\n\ngo 1.26.1\n"
README = """# Amitia Game Plugin SDK for Go

Public, standalone Go SDK for `amitia-game-host/1`.

This module intentionally has no dependency on the Amitia backend module or any
`internal/` package. Game plugins may depend on this module alone and use:

- `github.com/u-ai/game-plugin-sdk-go` for the SDK client/runner/helpers;
- `github.com/u-ai/game-plugin-sdk-go/protocol` for public wire contracts;
- `github.com/u-ai/game-plugin-sdk-go/protocol/contracts` for shared control contracts.

The source is synchronized from the canonical backend protocol/SDK by
`scripts/sync_game_plugin_go_sdk.py`; CI rejects drift.

## Host-mediated game networking

Use a `restricted` plugin network policy plus `service.network.request` for portable
game/companion communication. The plugin process stays network-isolated while
GameHost owns and policy-checks HTTP(S), TCP, UDP, and WebSocket connections.
`allowHostLoopback: true` enables the portable `host-loopback` target, so plugins
never need platform-specific host-network addresses. The root SDK exposes
`NetworkRequest`, `NetworkTCPOpen/Read/Write/Close`,
`NetworkUDPOpen/Receive/Send/Close`, and
`NetworkWebSocketOpen/Receive/Send/Close`. Handles are bound to the runtime
generation/session and are released on service stop/crash/restart or host shutdown.
"""


PUBLIC_SDK_TESTS = {
    "client_generation_test.go",
    "runner_handshake_test.go",
}


def go_files(directory: Path):
    return sorted(p for p in directory.glob("*.go") if not p.name.endswith("_test.go"))


def rewrite(text: str) -> str:
    return text.replace(
        "github.com/u-ai/backend/pkg/gameplugin/protocol/contracts",
        "github.com/u-ai/game-plugin-sdk-go/protocol/contracts",
    ).replace(
        "github.com/u-ai/backend/pkg/gameplugin/protocol",
        "github.com/u-ai/game-plugin-sdk-go/protocol",
    )


def generate(destination: Path) -> None:
    if destination.exists():
        shutil.rmtree(destination)
    (destination / "protocol/contracts").mkdir(parents=True)

    for source in go_files(BACKEND_PROTOCOL):
        (destination / "protocol" / source.name).write_text(rewrite(source.read_text()))
    for source in go_files(BACKEND_PROTOCOL / "contracts"):
        (destination / "protocol/contracts" / source.name).write_text(rewrite(source.read_text()))
    for source in go_files(BACKEND_SDK):
        (destination / source.name).write_text(rewrite(source.read_text()))
    for name in sorted(PUBLIC_SDK_TESTS):
        source = BACKEND_SDK / name
        if not source.is_file():
            raise FileNotFoundError(f"canonical public SDK test missing: {source}")
        (destination / name).write_text(rewrite(source.read_text()))

    (destination / "go.mod").write_text(GO_MOD)
    (destination / "README.md").write_text(README)
    shutil.copy2(ROOT / "LICENSE", destination / "LICENSE")


def tree_digest(root: Path) -> str:
    digest = hashlib.sha256()
    files = sorted(p for p in root.rglob("*") if p.is_file())
    for path in files:
        rel = path.relative_to(root).as_posix().encode("utf-8")
        data = path.read_bytes()
        digest.update(len(rel).to_bytes(8, "big"))
        digest.update(rel)
        digest.update(len(data).to_bytes(8, "big"))
        digest.update(data)
    return digest.hexdigest()


def same_tree(a: Path, b: Path) -> bool:
    if not a.is_dir() or not b.is_dir():
        return False
    return tree_digest(a) == tree_digest(b)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail if the public SDK is out of sync")
    args = parser.parse_args()

    if not args.check:
        generate(PUBLIC)
        return 0

    with tempfile.TemporaryDirectory(prefix="amitia-game-sdk-") as td:
        expected = Path(td) / "game-plugin-go"
        generate(expected)
        if not same_tree(expected, PUBLIC):
            print("standalone Go Game Plugin SDK is out of sync; run scripts/sync_game_plugin_go_sdk.py")
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
