#!/usr/bin/env python3
"""Canonical Backend Builder for Linux ARM64.

Adapts existing Backend build artifacts to the unified FrozenArtifactRecord
interface. This builder does NOT run a second Go build; it only adapts
an already-built backend artifact.
"""
import argparse
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from frozen_adapter import export_frozen_record
from errors import BuildError


def main():
    parser = argparse.ArgumentParser(description="Backend Linux ARM64 Builder")
    parser.add_argument("--offline", action="store_true", help="Run in offline mode")
    parser.add_argument("--input", required=True, help="Input directory with backend artifact")
    parser.add_argument("--output", required=True, help="Output path for frozen record")
    parser.add_argument("--source-revision", default=None, help="Source revision")
    args = parser.parse_args()

    if args.offline:
        print("Backend builder: offline mode - checks skipped")
        return 0

    try:
        record = export_frozen_record(args.input, args.output, source_revision=args.source_revision)
        print(f"Backend builder: exported {record.component_id} v{record.version}")
        return 0
    except BuildError as e:
        print(f"Backend builder: ERROR - {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
