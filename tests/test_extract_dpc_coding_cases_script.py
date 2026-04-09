from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "scripts" / "extract_dpc_coding_cases.py"
SPEC = importlib.util.spec_from_file_location("extract_dpc_coding_cases", SCRIPT_PATH)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def test_download_pdfはhttp_https以外を拒否する() -> None:
    with pytest.raises(ValueError, match="url must use http or https"):
        MODULE._download_pdf("file:///tmp/test.pdf")
