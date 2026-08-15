import argparse


def create_base_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Amitia Runtime Build System")
    parser.add_argument("--offline", action="store_true", help="Run in offline mode")
    parser.add_argument("--input-cache", type=str, default=None, help="Path to input artifact cache directory")
    parser.add_argument("--output-root", type=str, default=None, help="Root directory for build output")
    parser.add_argument("--release", action="store_true", help="Release build mode")
    return parser
