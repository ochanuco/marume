from __future__ import annotations

import importlib.util
from pathlib import Path
from types import SimpleNamespace

import pytest


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "scripts" / "extract_dpc_coding_cases.py"


@pytest.fixture(scope="module")
def script_module():
    if not SCRIPT_PATH.exists():
        pytest.skip(f"Script not found: {SCRIPT_PATH}")
    spec = importlib.util.spec_from_file_location("extract_dpc_coding_cases", SCRIPT_PATH)
    if spec is None or spec.loader is None:
        pytest.skip("Failed to load script spec")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_download_pdfはhttp_https以外を拒否する(script_module) -> None:
    with pytest.raises(ValueError, match="url must use http or https"):
        script_module._download_pdf("file:///tmp/test.pdf")


def test_mainはダウンロードした一時pdfを削除する(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path, script_module
) -> None:
    downloaded_pdf = tmp_path / "downloaded.pdf"
    downloaded_pdf.write_bytes(b"%PDF-1.4")
    output_path = tmp_path / "out.json"

    monkeypatch.setattr(
        script_module,
        "parse_args",
        lambda: SimpleNamespace(
            input_pdf=None,
            url="https://example.com/test.pdf",
            output=output_path,
            start_page=35,
            end_page=None,
        ),
    )
    monkeypatch.setattr(script_module, "_download_pdf", lambda _: downloaded_pdf)
    monkeypatch.setattr(script_module, "extract_coding_cases_from_pdf", lambda *args, **kwargs: [])
    monkeypatch.setattr(script_module, "write_coding_cases_json", lambda *args, **kwargs: None)

    assert script_module.main() == 0
    assert not downloaded_pdf.exists()
