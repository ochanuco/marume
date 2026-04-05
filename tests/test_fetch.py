from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path

from marume_data.fetch import fetch_mhlw_dpc_assets, resolve_latest_asset_path


def test_厚労省ページとDPC_PDFを安定した保存名で取得できる(
    tmp_path: Path,
    fake_url_reader_factory: Callable[[dict[str, bytes]], Callable[[str], object]],
) -> None:
    html = _fixture_path("mhlw_dpc_page.html").read_bytes()
    responses = {
        "https://www.mhlw.go.jp/stf/newpage_67729.html": html,
        "https://www.mhlw.go.jp/content/12404000/001234567.pdf": b"%PDF-provisional",
        "https://www.mhlw.go.jp/content/12404000/001234568.pdf": b"%PDF-official",
    }

    manifest = fetch_mhlw_dpc_assets(
        output_dir=tmp_path,
        page_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
        url_reader=fake_url_reader_factory(responses),
    )

    assert (tmp_path / "mhlw_dpc_page.html").read_bytes() == html
    assert (tmp_path / "dpc_official_20260318_001234568.pdf").read_bytes() == b"%PDF-official"
    assert (tmp_path / "dpc_provisional_20260305_001234567.pdf").read_bytes() == b"%PDF-provisional"
    assert manifest["assets"][0]["path"] == "dpc_official_20260318_001234568.pdf"
    assert manifest["assets"][1]["path"] == "dpc_provisional_20260305_001234567.pdf"


def test_manifest_JSONを書き出せる(
    tmp_path: Path,
    fake_url_reader_factory: Callable[[dict[str, bytes]], Callable[[str], object]],
) -> None:
    html = _fixture_path("mhlw_dpc_page.html").read_bytes()
    responses = {
        "https://www.mhlw.go.jp/stf/newpage_67729.html": html,
        "https://www.mhlw.go.jp/content/12404000/001234567.pdf": b"%PDF-provisional",
        "https://www.mhlw.go.jp/content/12404000/001234568.pdf": b"%PDF-official",
    }

    fetch_mhlw_dpc_assets(
        output_dir=tmp_path,
        page_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
        url_reader=fake_url_reader_factory(responses),
    )

    manifest = json.loads((tmp_path / "manifest.json").read_text(encoding="utf-8"))
    assert manifest["page_path"] == "mhlw_dpc_page.html"
    assert manifest["assets"][0]["kind"] == "official"
    assert manifest["assets"][1]["updated_at"] == "2026-03-05"


def test_manifestから正式版PDFを解決できる(
    tmp_path: Path,
    fake_url_reader_factory: Callable[[dict[str, bytes]], Callable[[str], object]],
) -> None:
    html = _fixture_path("mhlw_dpc_page.html").read_bytes()
    responses = {
        "https://www.mhlw.go.jp/stf/newpage_67729.html": html,
        "https://www.mhlw.go.jp/content/12404000/001234567.pdf": b"%PDF-provisional",
        "https://www.mhlw.go.jp/content/12404000/001234568.pdf": b"%PDF-official",
    }

    fetch_mhlw_dpc_assets(
        output_dir=tmp_path,
        page_url="https://www.mhlw.go.jp/stf/newpage_67729.html",
        url_reader=fake_url_reader_factory(responses),
    )

    resolved = resolve_latest_asset_path(tmp_path / "manifest.json")
    assert resolved == tmp_path / "dpc_official_20260318_001234568.pdf"


def _fixture_path(name: str) -> Path:
    return Path(__file__).with_name("fixtures").joinpath(name)
