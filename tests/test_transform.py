from __future__ import annotations

import json

from marume_data.transform import parse_mhlw_dpc_page, write_snapshot_from_mhlw_html


def test_厚労省ページからDPCリンクと更新日を抽出できる() -> None:
    html = _fixture_path().read_text(encoding="utf-8")

    metadata = parse_mhlw_dpc_page(
        html=html,
        base_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )

    assert metadata.title == "令和８年度診療報酬改定について｜厚生労働省"
    assert len(metadata.dpc_links) == 2
    assert metadata.dpc_links[0].updated_at == "2026-03-18"
    assert metadata.dpc_links[0].url == "https://www.mhlw.go.jp/content/12404000/001234568.pdf"


def test_厚労省ページからsnapshot_JSONを書き出せる(tmp_path) -> None:
    output_path = tmp_path / "dpc-2026.json"

    write_snapshot_from_mhlw_html(
        input_path=_fixture_path(),
        output_path=output_path,
        fiscal_year=2026,
        source_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
    )

    payload = json.loads(output_path.read_text(encoding="utf-8"))
    assert payload["rule_set"]["rule_version"] == "2026.20260318"
    assert payload["rule_set"]["source_published_at"] == "2026-03-18"
    assert payload["metadata"]["dpc_link_count"] == "2"
    assert payload["source_links"][0]["updated_at"] == "2026-03-18"


def _fixture_path():
    return __import__("pathlib").Path(__file__).with_name("fixtures").joinpath("mhlw_dpc_page.html")
