from __future__ import annotations

import argparse
import sys
import tempfile
import urllib.parse
import urllib.request
from pathlib import Path

from marume_data.coding_text import extract_coding_cases_from_pdf, write_coding_cases_json


DEFAULT_PDF_URL = "https://www.mhlw.go.jp/content/12404000/001394024.pdf"
DEFAULT_DOWNLOAD_TIMEOUT = 60
DEFAULT_USER_AGENT = "marume-data/0.1.0"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Extract DPC coding case examples from the MHLW PDF.")
    parser.add_argument("--input-pdf", type=Path, default=None, help="Local coding text PDF path.")
    parser.add_argument("--url", default=DEFAULT_PDF_URL, help="PDF URL to download when --input-pdf is omitted.")
    parser.add_argument("--output", type=Path, required=True, help="Output JSON path.")
    parser.add_argument("--start-page", type=int, default=35, help="1-based page to start parsing from.")
    parser.add_argument("--end-page", type=int, default=None, help="1-based page to stop parsing at.")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    downloaded_temp_pdf = args.input_pdf is None
    pdf_path: Path | None = None
    cases: list[object] = []

    try:
        pdf_path = args.input_pdf or _download_pdf(args.url)
        cases = extract_coding_cases_from_pdf(
            pdf_path,
            start_page=args.start_page,
            end_page=args.end_page,
        )
        write_coding_cases_json(args.output, cases)
    except Exception as exc:  # noqa: BLE001 - top-level CLI converts unexpected errors into exit status 1
        print(f"coding case extraction failed: {exc}", file=sys.stderr)
        return 1
    finally:
        if downloaded_temp_pdf and pdf_path is not None and pdf_path.exists():
            pdf_path.unlink()

    print(f"{args.output} ({len(cases)} cases)")
    return 0


def _download_pdf(url: str) -> Path:
    parsed = urllib.parse.urlparse(url)
    if parsed.scheme not in {"http", "https"}:
        raise ValueError("url must use http or https")

    request = urllib.request.Request(url, headers={"User-Agent": DEFAULT_USER_AGENT})
    with urllib.request.urlopen(request, timeout=DEFAULT_DOWNLOAD_TIMEOUT) as response:
        suffix = Path(parsed.path).suffix or ".pdf"
        with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as handle:
            while chunk := response.read(65536):
                handle.write(chunk)
            return Path(handle.name)


if __name__ == "__main__":
    raise SystemExit(main())
