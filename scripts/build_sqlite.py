from __future__ import annotations

import argparse
import json
import sqlite3
import sys
from pathlib import Path

from marume_data.sqlite_builder import create_snapshot_database, load_snapshot_json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build a SQLite snapshot from normalized JSON.")
    parser.add_argument("--input", type=Path, required=True, help="Normalized snapshot JSON path.")
    parser.add_argument("--output", type=Path, required=True, help="SQLite output path.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        snapshot = load_snapshot_json(args.input)
        create_snapshot_database(args.output, snapshot)
    except FileNotFoundError as exc:
        print(f"入力ファイルが見つかりません: {exc}", file=sys.stderr)
        return 1
    except json.JSONDecodeError as exc:
        print(f"入力JSONの読み込みに失敗しました: {args.input}: {exc}", file=sys.stderr)
        return 1
    except sqlite3.Error as exc:
        print(f"SQLite 生成に失敗しました: {args.output}: {exc}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"予期しないエラーが発生しました: {args.output}: {exc}", file=sys.stderr)
        return 1
    print(args.output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
