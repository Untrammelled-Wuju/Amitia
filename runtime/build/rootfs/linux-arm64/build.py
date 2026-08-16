#!/usr/bin/env python3
"""Canonical Rootfs Builder for Linux ARM64."""
import argparse
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from artifact_record import FrozenArtifactRecord, validate


def main():
    parser = argparse.ArgumentParser(description="Rootfs Linux ARM64 Builder")
    parser.add_argument("--offline", action="store_true", help="Run in offline mode")
    parser.add_argument("--input", default=None, help="Input directory")
    parser.add_argument("--output", default=None, help="Output directory")
    args = parser.parse_args()

    if args.offline:
        print("Rootfs builder: offline mode - checks skipped")
        return 0

    print("Rootfs builder: canonical builder entry point")
    return 0


if __name__ == "__main__":
    sys.exit(main())
