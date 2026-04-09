from __future__ import annotations

import importlib.util
from pathlib import Path
from types import SimpleNamespace

import pytest


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "scripts" / "extract_dpc_coding_cases.py"
SPEC = importlib.util.spec_from_file_location("extract_dpc_coding_cases", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def test_download_pdfはhttp_https以外を拒否する() -> None:
    with pytest.raises(ValueError, match="url must use http or https"):
        MODULE._download_pdf("file:///tmp/test.pdf")


def test_mainはダウンロードした一時pdfを削除する(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    downloaded_pdf = tmp_path / "downloaded.pdf"
    downloaded_pdf.write_bytes(b"%PDF-1.4")
    output_path = tmp_path / "out.json"

    monkeypatch.setattr(
        MODULE,
        "parse_args",
        lambda: SimpleNamespace(
            input_pdf=None,
            url="https://example.com/test.pdf",
            output=output_path,
            start_page=35,
            end_page=None,
        ),
    )
    monkeypatch.setattr(MODULE, "_download_pdf", lambda _: downloaded_pdf)
    monkeypatch.setattr(MODULE, "extract_coding_cases_from_pdf", lambda *args, **kwargs: [])
    monkeypatch.setattr(MODULE, "write_coding_cases_json", lambda *args, **kwargs: None)

    assert MODULE.main() == 0
    assert not downloaded_pdf.exists()
