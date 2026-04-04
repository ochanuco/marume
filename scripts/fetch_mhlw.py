from __future__ import annotations

import argparse
import urllib.request
from pathlib import Path

from marume_data.fetch import fetch_mhlw_dpc_assets


DEFAULT_DPC_URL = "https://www.mhlw.go.jp/stf/newpage_67729.html"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fetch MHLW source pages for marume data prep.")
    parser.add_argument("--url", default=DEFAULT_DPC_URL, help="Source page URL to fetch.")
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path(".local/raw/mhlw"),
        help="Directory where fetched files are stored.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    manifest = fetch_mhlw_dpc_assets(
        output_dir=args.output_dir,
        page_url=args.url,
        url_reader=_url_reader_with_timeout,  # noqa: S310
    )
    print(args.output_dir / manifest["page_path"])
    for asset in manifest["assets"]:
        print(args.output_dir / asset["path"])
    return 0


def _url_reader_with_timeout(url: str):
    return urllib.request.urlopen(url, timeout=10)  # noqa: S310


if __name__ == "__main__":
    raise SystemExit(main())
