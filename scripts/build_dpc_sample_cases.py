from __future__ import annotations

import argparse
import json
import sys
import traceback
from pathlib import Path

from marume_data.sample_cases import (
    build_case_input_candidate_report,
    build_case_input_candidates,
    build_sample_case_candidates,
    write_case_input_candidate_report_json,
    write_case_input_candidates_jsonl,
    write_sample_case_candidates_json,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build marume-friendly sample case candidates from extracted coding cases.")
    parser.add_argument("--input", type=Path, required=True, help="Extracted coding cases JSON path.")
    parser.add_argument("--output", type=Path, required=True, help="Output JSON path.")
    parser.add_argument(
        "--case-input-jsonl",
        type=Path,
        default=None,
        help="Optional output JSONL path for marume case-input candidates.",
    )
    parser.add_argument(
        "--report",
        type=Path,
        default=None,
        help="Optional output JSON path for generation counts and skip reasons.",
    )
    parser.add_argument("--fiscal-year", type=int, required=True, help="Fiscal year to stamp into case candidates.")
    parser.add_argument(
        "--debug",
        action=argparse.BooleanOptionalAction,
        default=False,
        help="Print a traceback when generation fails.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        extracted = json.loads(args.input.read_text(encoding="utf-8"))
        if not isinstance(extracted, list):
            raise TypeError("input JSON must be an array")
        cases = build_sample_case_candidates(extracted, fiscal_year=args.fiscal_year)
        write_sample_case_candidates_json(args.output, cases)
        if args.case_input_jsonl is not None:
            case_inputs = build_case_input_candidates(cases)
            write_case_input_candidates_jsonl(args.case_input_jsonl, case_inputs)
        if args.report is not None:
            report = build_case_input_candidate_report(cases)
            write_case_input_candidate_report_json(args.report, report)
    except Exception as exc:
        if args.debug:
            traceback.print_exc(file=sys.stderr)
        else:
            print(f"sample case generation failed: {exc}", file=sys.stderr)
        return 1
    else:
        print(f"{args.output} ({len(cases)} cases)")
        if args.case_input_jsonl is not None:
            print(f"{args.case_input_jsonl} ({len(case_inputs)} case inputs)")
        if args.report is not None:
            print(f"{args.report}")
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
