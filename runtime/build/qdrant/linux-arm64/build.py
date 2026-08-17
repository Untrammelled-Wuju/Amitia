#!/usr/bin/env python3
"""Canonical Qdrant Builder for Linux ARM64.

Adapts existing Qdrant build artifacts to the unified FrozenArtifactRecord
interface. This builder does NOT run a second download/producer; it only
adapts an already-built qdrant artifact.
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
    parser = argparse.ArgumentParser(description="Qdrant Linux ARM64 Builder")
    parser.add_argument("--offline", action="store_true", help="Run in offline mode")
    parser.add_argument("--input", required=True, help="Input directory with qdrant artifact")
    parser.add_argument("--output", required=True, help="Output path for frozen record")
    parser.add_argument("--source-revision", default=None, help="Source revision")
    args = parser.parse_args()

    try:
        record = export_frozen_record(args.input, args.output, source_revision=args.source_revision, offline=args.offline)
        print(f"Qdrant builder: exported {record.componentId} v{record.version}")
        return 0
    except BuildError as e:
        print(f"Qdrant builder: ERROR - {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
