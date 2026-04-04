from __future__ import annotations

import argparse
import json
import urllib.request
from pathlib import Path

from marume_data.workflow import load_workflow_config, run_workflow


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run the marume Python data workflow from a JSON config.")
    parser.add_argument("--workflow", type=Path, required=True, help="Workflow JSON path.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    config = load_workflow_config(args.workflow)
    result = run_workflow(config, url_reader=_url_reader_with_timeout)  # noqa: S310
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


def _url_reader_with_timeout(url: str):
    return urllib.request.urlopen(url, timeout=10)  # noqa: S310


if __name__ == "__main__":
    raise SystemExit(main())
