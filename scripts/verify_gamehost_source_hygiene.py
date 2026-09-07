#!/usr/bin/env python3
"""Reject stale/duplicate GameHost source trees that can mask canonical code.

These paths have previously contained copied implementations that were not part
of the build but were still shipped in source archives. Keeping this gate in CI
prevents reviewers and plugin authors from seeing multiple divergent copies of
security- and lifecycle-sensitive GameHost code.
"""
from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FORBIDDEN = (
    ROOT / "backend/internal/gamehost/integration/_pending",
    ROOT / "U-Ai",
    ROOT / "temp_extract",
)


def main() -> None:
    found = [path.relative_to(ROOT).as_posix() for path in FORBIDDEN if path.exists()]
    if found:
        print("ERROR: stale/duplicate GameHost source paths must not be shipped:", file=sys.stderr)
        for path in found:
            print(f"  - {path}", file=sys.stderr)
        raise SystemExit(1)
    print("GameHost source hygiene verified")


if __name__ == "__main__":
    main()
