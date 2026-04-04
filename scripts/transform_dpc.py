from __future__ import annotations

import argparse
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Transform fetched MHLW assets into a snapshot JSON.")
    parser.add_argument("--input", type=Path, required=True, help="Fetched source file path.")
    parser.add_argument("--output", type=Path, required=True, help="Output snapshot JSON path.")
    parser.add_argument(
        "--rules-csv",
        type=Path,
        default=None,
        help="Optional CSV file that contains flattened DPC rules.",
    )
    parser.add_argument("--fiscal-year", type=int, default=2026, help="Fiscal year to stamp in output.")
    parser.add_argument(
        "--source-url",
        default="https://www.mhlw.go.jp/stf/newpage_67729.html",
        help="Source URL recorded in the snapshot.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if not args.input.exists():
        raise FileNotFoundError(args.input)
    from marume_data.transform import parse_mhlw_dpc_page, write_snapshot_from_sources

    html = args.input.read_text(encoding="utf-8")
    metadata = parse_mhlw_dpc_page(
        html=html,
        base_url=args.source_url,
    )
    write_snapshot_from_sources(
        output_path=args.output,
        fiscal_year=args.fiscal_year,
        source_url=args.source_url,
        page_metadata=metadata,
        rules_csv_path=args.rules_csv,
    )
    print(args.output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
