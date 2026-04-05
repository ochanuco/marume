from __future__ import annotations

import argparse
import json
from pathlib import Path

from marume_data.http import url_reader_with_timeout
from marume_data.workflow import load_workflow_config, run_workflow


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the marume Python data workflow from a JSON config.")
    parser.add_argument("--workflow", type=Path, required=True, help="Workflow JSON path.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        config = load_workflow_config(args.workflow)
        result = run_workflow(config, url_reader=url_reader_with_timeout)
    except FileNotFoundError as exc:
        print(f"workflow 実行に必要なファイルが見つかりません: {exc}")
        return 1
    except (KeyError, ValueError, json.JSONDecodeError) as exc:
        print(f"workflow 設定または入力が不正です: {exc}")
        return 1
    except Exception as exc:
        print(f"workflow 実行に失敗しました: {exc}")
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0
if __name__ == "__main__":
    raise SystemExit(main())
