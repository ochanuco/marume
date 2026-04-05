from __future__ import annotations

import json
from pathlib import Path

from marume_data.extract import RULES_CSV_HEADERS, scaffold_rules_csv_from_manifest, scaffold_rules_csv_from_pdf


def test_正式版PDFからルールCSVの雛形を作れる(tmp_path) -> None:
    pdf_path = tmp_path / "dpc_official_20260318.pdf"
    pdf_path.write_bytes(b"%PDF-official")
    output_csv_path = tmp_path / "dpc_rules.csv"

    scaffold_rules_csv_from_pdf(pdf_path=pdf_path, output_csv_path=output_csv_path)

    lines = output_csv_path.read_text(encoding="utf-8").splitlines()
    assert lines == [",".join(RULES_CSV_HEADERS)]
    metadata = json.loads(output_csv_path.with_suffix(".source.json").read_text(encoding="utf-8"))
    assert metadata["source_pdf"] == str(pdf_path)
    assert metadata["status"] == "scaffolded"


def test_manifestから正式版PDFを選んでルールCSVの雛形を作れる(tmp_path: Path) -> None:
    fixture_dir = Path(__file__).with_name("fixtures")
    manifest_path = tmp_path / "manifest.json"
    manifest_path.write_text((fixture_dir / "manifest_with_pdfs.json").read_text(encoding="utf-8"), encoding="utf-8")
    (tmp_path / "dpc_official_20260318_001234568.pdf").write_bytes(b"%PDF-official")
    (tmp_path / "dpc_provisional_20260305_001234567.pdf").write_bytes(b"%PDF-provisional")

    output_csv_path = scaffold_rules_csv_from_manifest(
        manifest_path=manifest_path,
        output_csv_path=tmp_path / "dpc_rules.csv",
    )

    assert output_csv_path == tmp_path / "dpc_rules.csv"
    lines = output_csv_path.read_text(encoding="utf-8").splitlines()
    assert lines == [",".join(RULES_CSV_HEADERS)]
    metadata = json.loads(output_csv_path.with_suffix(".source.json").read_text(encoding="utf-8"))
    assert metadata["source_pdf"].endswith("dpc_official_20260318_001234568.pdf")
    assert metadata["status"] == "scaffolded"
