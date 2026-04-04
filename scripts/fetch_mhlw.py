from __future__ import annotations

import argparse
import urllib.request
from pathlib import Path


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
    args.output_dir.mkdir(parents=True, exist_ok=True)
    output_path = args.output_dir / "dpc-2026.html"
    with urllib.request.urlopen(args.url) as response:  # noqa: S310
        output_path.write_bytes(response.read())
    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
