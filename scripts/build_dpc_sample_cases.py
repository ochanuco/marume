from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from marume_data.sample_cases import build_sample_case_candidates, write_sample_case_candidates_json


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build marume-friendly sample case candidates from extracted coding cases.")
    parser.add_argument("--input", type=Path, required=True, help="Extracted coding cases JSON path.")
    parser.add_argument("--output", type=Path, required=True, help="Output JSON path.")
    parser.add_argument("--fiscal-year", type=int, required=True, help="Fiscal year to stamp into case candidates.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        extracted = json.loads(args.input.read_text(encoding="utf-8"))
        if not isinstance(extracted, list):
            raise TypeError("input JSON must be an array")
        cases = build_sample_case_candidates(extracted, fiscal_year=args.fiscal_year)
        write_sample_case_candidates_json(args.output, cases)
    except Exception as exc:
        print(f"sample case generation failed: {exc}", file=sys.stderr)
        return 1
    print(f"{args.output} ({len(cases)} cases)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
