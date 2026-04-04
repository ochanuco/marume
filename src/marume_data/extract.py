from __future__ import annotations

import csv
import json
from pathlib import Path

from marume_data.fetch import resolve_latest_pdf_path


RULES_CSV_HEADERS = (
    "rule_id",
    "priority",
    "dpc_code",
    "mdc_code",
    "label",
    "main_diagnosis",
    "procedures",
)


def scaffold_rules_csv_from_manifest(manifest_path: Path, output_csv_path: Path) -> Path:
    pdf_path = resolve_latest_pdf_path(manifest_path, kind="official")
    if pdf_path is None:
        raise FileNotFoundError("official DPC PDF was not found in manifest")
    return scaffold_rules_csv_from_pdf(pdf_path=pdf_path, output_csv_path=output_csv_path)


def scaffold_rules_csv_from_pdf(pdf_path: Path, output_csv_path: Path) -> Path:
    if not pdf_path.exists():
        raise FileNotFoundError(pdf_path)

    output_csv_path.parent.mkdir(parents=True, exist_ok=True)
    with output_csv_path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(RULES_CSV_HEADERS)

    metadata_path = output_csv_path.with_suffix(".source.json")
    metadata = {
        "source_pdf": str(pdf_path),
        "output_csv": str(output_csv_path),
        "status": "scaffolded",
        "note": "Replace this scaffold with extracted DPC rules from the official PDF.",
    }
    metadata_path.write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return output_csv_path
