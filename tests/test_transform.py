from __future__ import annotations

import json
from pathlib import Path

from marume_data.transform import (
    build_snapshot_payload,
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


def test_snapshotでは正式版を優先して最新版リンクとして扱う() -> None:
    html = """
    <html>
      <head><title>dummy</title></head>
      <body>
        <a href="/content/provisional.pdf">診断群分類（DPC）電子点数表（暫定版）（令和8年3月20日更新）</a>
        <a href="/content/official.pdf">診断群分類（DPC）電子点数表（正式版）（令和8年3月18日更新）</a>
      </body>
    </html>
    """

    metadata = parse_mhlw_dpc_page(
        html=html,
        base_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )
    output = json.loads(
        json.dumps(
            build_snapshot_payload(
                fiscal_year=2026,
                source_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
                page_metadata=metadata,
            ),
            ensure_ascii=False,
        )
    )

    assert output["rule_set"]["rule_version"] == "2026.20260318"
    assert output["metadata"]["latest_dpc_url"] == "https://www.mhlw.go.jp/content/official.pdf"


def test_DPCルールCSVからrulesとconditionsを組み立てられる() -> None:
    rows = parse_dpc_rules_csv(_csv_fixture_path())

    assert len(rows) == 2
    assert rows[0].rule_id == "R-2026-0001"
    assert rows[0].procedures == ["K549", "K546"]
    assert rows[1].main_diagnosis == "K703"


def test_DPCルールCSVの必須列不足を検出できる(tmp_path: Path) -> None:
    csv_path = tmp_path / "invalid.csv"
    csv_path.write_text("rule_id,priority\nR-1,1\n", encoding="utf-8")

    try:
        parse_dpc_rules_csv(csv_path)
    except ValueError as exc:
        assert "parse_dpc_rules_csv: missing required columns: dpc_code" == str(exc)
    else:
        raise AssertionError("ValueError was not raised")


def test_DPCルールCSVの不正行を行番号付きで報告できる(tmp_path: Path) -> None:
    csv_path = tmp_path / "invalid-row.csv"
    csv_path.write_text(
        "rule_id,priority,dpc_code\nR-1,not-a-number,010010xx99x0xx\n",
        encoding="utf-8",
    )

    try:
        parse_dpc_rules_csv(csv_path)
    except ValueError as exc:
        assert "parse_dpc_rules_csv: row 2 is invalid" in str(exc)
        assert "not-a-number" in str(exc)
    else:
        raise AssertionError("ValueError was not raised")


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
