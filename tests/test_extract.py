from __future__ import annotations

import json
from pathlib import Path

import pytest

from marume_data.extract import RULES_CSV_HEADERS, scaffold_rules_csv_from_manifest, scaffold_rules_csv_from_workbook
from tests.helpers import build_sample_dpc_workbook_bytes


def test_正式版workbookからルールCSVを抽出できる(tmp_path: Path) -> None:
    workbook_path = tmp_path / "dpc_official_20260318.xlsx"
    workbook_path.write_bytes(build_sample_dpc_workbook_bytes())
    output_csv_path = tmp_path / "dpc_rules.csv"

    scaffold_rules_csv_from_workbook(workbook_path=workbook_path, output_csv_path=output_csv_path)

    lines = output_csv_path.read_text(encoding="utf-8").splitlines()
    assert lines == [
        ",".join(RULES_CSV_HEADERS),
        "R-010010-00001,10,010010xx9900xx,01,脳腫瘍,C700,",
        "R-010010-00002,20,010010xx9901xx,01,脳腫瘍,C700,",
    ]
    metadata = json.loads(output_csv_path.with_suffix(".source.json").read_text(encoding="utf-8"))
    assert metadata["source_workbook"] == str(workbook_path)
    assert metadata["status"] == "extracted"
    assert metadata["row_count"] == 2


def test_manifestから正式版workbookを選んでルールCSVを抽出できる(tmp_path: Path) -> None:
    manifest_path = tmp_path / "manifest.json"
    manifest_path.write_text(
        json.dumps(
            {
                "page_url": "https://example.com/mhlw_dpc_page.html",
                "page_path": "mhlw_dpc_page.html",
                "source_title": "令和８年度診療報酬改定について｜厚生労働省",
                "assets": [
                    {
                        "kind": "official",
                        "label": "診断群分類（DPC）電子点数表（正式版）",
                        "source_url": "https://example.com/official.xlsx",
                        "path": "dpc_official_20260318_001234568.xlsx",
                        "updated_at": "2026-03-18",
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )
    (tmp_path / "dpc_official_20260318_001234568.xlsx").write_bytes(build_sample_dpc_workbook_bytes())

    output_csv_path = scaffold_rules_csv_from_manifest(
        manifest_path=manifest_path,
        output_csv_path=tmp_path / "dpc_rules.csv",
    )

    assert output_csv_path == tmp_path / "dpc_rules.csv"
    lines = output_csv_path.read_text(encoding="utf-8").splitlines()
    assert lines[0] == ",".join(RULES_CSV_HEADERS)
    assert lines[1].startswith("R-010010-00001,10,010010xx9900xx,01,脳腫瘍,C700,")
    metadata = json.loads(output_csv_path.with_suffix(".source.json").read_text(encoding="utf-8"))
    assert metadata["source_workbook"].endswith("dpc_official_20260318_001234568.xlsx")
    assert metadata["status"] == "extracted"


def test_manifestの正式版assetがworkbookでない場合はエラーにする(tmp_path: Path) -> None:
    manifest_path = tmp_path / "manifest.json"
    manifest_path.write_text(
        json.dumps(
            {
                "page_url": "https://example.com/mhlw_dpc_page.html",
                "page_path": "mhlw_dpc_page.html",
                "source_title": "令和８年度診療報酬改定について｜厚生労働省",
                "assets": [
                    {
                        "kind": "official",
                        "label": "診断群分類（DPC）電子点数表（正式版）",
                        "source_url": "https://example.com/official.pdf",
                        "path": "dpc_official_20260318_001234568.pdf",
                        "updated_at": "2026-03-18",
                    }
                ],
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    with pytest.raises(ValueError, match="official DPC workbook is required"):
        scaffold_rules_csv_from_manifest(
            manifest_path=manifest_path,
            output_csv_path=tmp_path / "dpc_rules.csv",
        )


def test_workbookに必要なsheetが無い場合はわかりやすく失敗する(tmp_path: Path) -> None:
    workbook_path = tmp_path / "dpc_official_20260318.xlsx"
    workbook_path.write_bytes(build_sample_dpc_workbook_bytes(include_icd_sheet=False))

    with pytest.raises(ValueError, match="required sheet was not found in workbook: ４）ＩＣＤ"):
        scaffold_rules_csv_from_workbook(
            workbook_path=workbook_path,
            output_csv_path=tmp_path / "dpc_rules.csv",
        )


def test_workbookの行が壊れている場合は列不足を明示して失敗する(tmp_path: Path) -> None:
    workbook_path = tmp_path / "dpc_official_20260318.xlsx"
    workbook_path.write_bytes(build_sample_dpc_workbook_bytes(include_point_label_column=False))

    with pytest.raises(
        ValueError,
        match="malformed row in sheet 11）診断群分類点数表: row 1 does not have column index 3",
    ):
        scaffold_rules_csv_from_workbook(
            workbook_path=workbook_path,
            output_csv_path=tmp_path / "dpc_rules.csv",
        )
