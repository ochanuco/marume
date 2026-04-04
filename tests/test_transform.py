from __future__ import annotations

import json
from pathlib import Path

from marume_data.transform import (
    parse_dpc_rules_csv,
    parse_mhlw_dpc_page,
    write_snapshot_from_mhlw_html,
    write_snapshot_from_sources,
)


def test_厚労省ページからDPCリンクと更新日を抽出できる() -> None:
    html = _html_fixture_path().read_text(encoding="utf-8")

    metadata = parse_mhlw_dpc_page(
        html=html,
        base_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )

    assert metadata.title == "令和８年度診療報酬改定について｜厚生労働省"
    assert len(metadata.dpc_links) == 2
    assert metadata.dpc_links[0].updated_at == "2026-03-18"
    assert metadata.dpc_links[0].url == "https://www.mhlw.go.jp/content/12404000/001234568.pdf"


def test_厚労省ページからsnapshot_JSONを書き出せる(tmp_path: Path) -> None:
    output_path = tmp_path / "dpc-2026.json"

    write_snapshot_from_mhlw_html(
        input_path=_html_fixture_path(),
        output_path=output_path,
        fiscal_year=2026,
        source_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )

    payload = json.loads(output_path.read_text(encoding="utf-8"))
    assert payload["rule_set"]["rule_version"] == "2026.20260318"
    assert payload["rule_set"]["source_published_at"] == "2026-03-18"
    assert payload["metadata"]["dpc_link_count"] == "2"
    assert payload["source_links"][0]["updated_at"] == "2026-03-18"


def test_DPCルールCSVからrulesとconditionsを組み立てられる() -> None:
    rows = parse_dpc_rules_csv(_csv_fixture_path())

    assert len(rows) == 2
    assert rows[0].rule_id == "R-2026-0001"
    assert rows[0].procedures == ["K549", "K546"]
    assert rows[1].main_diagnosis == "K703"


def test_厚労省ページとCSVからrules入りsnapshot_JSONを書き出せる(tmp_path: Path) -> None:
    output_path = tmp_path / "dpc-2026-rules.json"
    html = _html_fixture_path().read_text(encoding="utf-8")
    metadata = parse_mhlw_dpc_page(
        html=html,
        base_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )

    write_snapshot_from_sources(
        output_path=output_path,
        fiscal_year=2026,
        source_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
        page_metadata=metadata,
        rules_csv_path=_csv_fixture_path(),
    )

    payload = json.loads(output_path.read_text(encoding="utf-8"))
    assert payload["metadata"]["rule_count"] == "2"
    assert payload["rule_set"]["rules"][0]["rule_id"] == "R-2026-0001"
    assert payload["rule_set"]["rules"][0]["conditions"][0]["condition_type"] == "main_diagnosis"
    assert payload["rule_set"]["rules"][0]["conditions"][1]["value_json"] == ["K549", "K546"]

def _html_fixture_path() -> Path:
    return Path(__file__).with_name("fixtures").joinpath("mhlw_dpc_page.html")


def _csv_fixture_path() -> Path:
    return Path(__file__).with_name("fixtures").joinpath("dpc_rules.csv")
