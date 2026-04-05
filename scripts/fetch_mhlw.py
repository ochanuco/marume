from __future__ import annotations

import argparse
import sys
from pathlib import Path

from marume_data.fetch import fetch_mhlw_dpc_assets
from marume_data.http import url_reader_with_timeout


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fetch MHLW source pages for marume data prep.")
    parser.add_argument("--url", required=True, help="Source page URL to fetch.")
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path(".local/raw/mhlw"),
        help="Directory where fetched files are stored.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        manifest = fetch_mhlw_dpc_assets(
            output_dir=args.output_dir,
            page_url=args.url,
            url_reader=url_reader_with_timeout,
        )
    except Exception as exc:
        print(f"厚労省データの取得に失敗しました: {args.url}: {exc}", file=sys.stderr)
        return 1
    print(args.output_dir / manifest["page_path"])
    for asset in manifest["assets"]:
        print(args.output_dir / asset["path"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
