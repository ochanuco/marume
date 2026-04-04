from __future__ import annotations

import argparse
from pathlib import Path

from marume_data.sqlite_builder import create_snapshot_database, load_snapshot_json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build a SQLite snapshot from normalized JSON.")
    parser.add_argument("--input", type=Path, required=True, help="Normalized snapshot JSON path.")
    parser.add_argument("--output", type=Path, required=True, help="SQLite output path.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    snapshot = load_snapshot_json(args.input)
    create_snapshot_database(args.output, snapshot)
    print(args.output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
