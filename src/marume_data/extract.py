from __future__ import annotations

import csv
import json
import os
from io import BytesIO
from pathlib import Path

from openpyxl import load_workbook

from marume_data.fetch import resolve_latest_asset_path


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
    """Extract rules CSV rows from the official DPC workbook referenced by a manifest."""

    source_path = resolve_latest_asset_path(manifest_path, kind="official")
    if source_path is None:
        raise FileNotFoundError("official DPC workbook was not found in manifest")
    if source_path.suffix.lower() not in {".xlsx", ".xlsm"}:
        raise ValueError(f"official DPC workbook is required, but got: {source_path.name}")
    return scaffold_rules_csv_from_workbook(workbook_path=source_path, output_csv_path=output_csv_path)


def scaffold_rules_csv_from_workbook(workbook_path: Path, output_csv_path: Path) -> Path:
    """Extract flattened rule rows from an official DPC workbook."""

    if not workbook_path.exists():
        raise FileNotFoundError(workbook_path)

    rows = _extract_flat_rule_rows(workbook_path)
    output_csv_path.parent.mkdir(parents=True, exist_ok=True)
    tmp_output_csv_path = output_csv_path.with_name(f".{output_csv_path.name}.tmp")
    with tmp_output_csv_path.open("w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(RULES_CSV_HEADERS)
        writer.writerows(rows)
        handle.flush()
        os.fsync(handle.fileno())
    tmp_output_csv_path.replace(output_csv_path)

    metadata_path = output_csv_path.with_suffix(".source.json")
    metadata = {
        "source_workbook": str(workbook_path),
        "output_csv": str(output_csv_path),
        "status": "extracted",
        "row_count": len(rows),
        "note": "Rows were extracted from the official DPC workbook. Procedure mapping is still simplified.",
    }
    tmp_metadata_path = metadata_path.with_name(f".{metadata_path.name}.tmp")
    with tmp_metadata_path.open("w", encoding="utf-8") as handle:
        handle.write(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n")
        handle.flush()
        os.fsync(handle.fileno())
    tmp_metadata_path.replace(metadata_path)
    return output_csv_path


def _extract_flat_rule_rows(source_path: Path) -> list[tuple[str, int, str, str, str, str, str]]:
    """Extract a minimal flattened rule CSV from the official DPC workbook."""

    workbook = load_workbook(BytesIO(source_path.read_bytes()), read_only=True, data_only=True)
    try:
        icd_by_classification = _load_first_icd_by_classification(workbook["４）ＩＣＤ"])
        score_sheet = workbook["11）診断群分類点数表"]
        rows: list[tuple[str, int, str, str, str, str, str]] = []
        priority = 10
        for row_index, row in enumerate(score_sheet.iter_rows(min_row=5, values_only=True), start=1):
            dpc_code = _normalize_cell(row[2])
            if not dpc_code:
                continue
            label = _normalize_cell(row[3]) or dpc_code
            mdc_code = dpc_code[:2]
            classification_code = dpc_code[2:6]
            main_diagnosis = icd_by_classification.get((mdc_code, classification_code), "")
            rows.append(
                (
                    f"R-{mdc_code}{classification_code}-{row_index:05d}",
                    priority,
                    dpc_code,
                    mdc_code,
                    label,
                    main_diagnosis,
                    "",
                )
            )
            priority += 10
        return rows
    finally:
        workbook.close()


def _load_first_icd_by_classification(sheet: object) -> dict[tuple[str, str], str]:
    """Load the first ICD code for each classification from the ICD sheet."""

    mapping: dict[tuple[str, str], str] = {}
    for row in sheet.iter_rows(min_row=3, values_only=True):
        mdc_code = _normalize_cell(row[0])
        classification_code = _normalize_cell(row[1])
        icd_code = _normalize_cell(row[3])
        if not mdc_code or not classification_code or not icd_code:
            continue
        mapping.setdefault((mdc_code, classification_code), icd_code)
    return mapping


def _normalize_cell(value: object) -> str:
    """Normalize workbook cell values into stripped strings."""

    if value is None:
        return ""
    return str(value).strip()
