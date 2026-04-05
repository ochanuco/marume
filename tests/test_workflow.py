from __future__ import annotations

import json
from collections.abc import Callable
from pathlib import Path

from marume_data.workflow import load_workflow_config, run_workflow


def test_workflow_JSON初回実行ではCSV雛形を作って止まる(
    tmp_path: Path,
    fake_url_reader_factory: Callable[[dict[str, bytes]], Callable[[str], object]],
) -> None:
    fixture_dir = Path(__file__).with_name("fixtures")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(
        json.dumps(
            {
                "source_url": "https://example.com/mhlw_dpc_page.html",
                "fiscal_year": 2026,
                "paths": {
                    "raw_dir": str(tmp_path / "raw"),
                    "rules_csv": str(tmp_path / "raw" / "dpc_rules.csv"),
                    "snapshot_json": str(tmp_path / "snapshot.json"),
                    "sqlite_output": str(tmp_path / "rules.sqlite"),
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    html = (fixture_dir / "mhlw_dpc_page.html").read_bytes()
    responses = {
        "https://example.com/mhlw_dpc_page.html": html,
        "https://example.com/content/12404000/001234567.pdf": b"%PDF-provisional",
        "https://example.com/content/12404000/001234568.pdf": b"%PDF-official",
    }
    config = load_workflow_config(workflow_path)

    result = run_workflow(config, url_reader=fake_url_reader_factory(responses))

    assert result["status"] == "needs_rules_csv"
    assert Path(result["manifest"]).exists()
    assert Path(result["rules_csv"]).exists()
    assert not (tmp_path / "snapshot.json").exists()
    assert not (tmp_path / "rules.sqlite").exists()


def test_workflow_JSONに実ルールCSVがあれば最後まで実行できる(
    tmp_path: Path,
    fake_url_reader_factory: Callable[[dict[str, bytes]], Callable[[str], object]],
) -> None:
    fixture_dir = Path(__file__).with_name("fixtures")
    workflow_path = tmp_path / "workflow.json"
    workflow_path.write_text(
        json.dumps(
            {
                "source_url": "https://example.com/mhlw_dpc_page.html",
                "fiscal_year": 2026,
                "paths": {
                    "raw_dir": str(tmp_path / "raw"),
                    "rules_csv": str(tmp_path / "raw" / "dpc_rules.csv"),
                    "snapshot_json": str(tmp_path / "snapshot.json"),
                    "sqlite_output": str(tmp_path / "rules.sqlite"),
                },
            },
            ensure_ascii=False,
        ),
        encoding="utf-8",
    )

    raw_dir = tmp_path / "raw"
    raw_dir.mkdir(parents=True, exist_ok=True)
    (raw_dir / "dpc_rules.csv").write_text(
        (fixture_dir / "dpc_rules.csv").read_text(encoding="utf-8"),
        encoding="utf-8",
    )

    html = (fixture_dir / "mhlw_dpc_page.html").read_bytes()
    responses = {
        "https://example.com/mhlw_dpc_page.html": html,
        "https://example.com/content/12404000/001234567.pdf": b"%PDF-provisional",
        "https://example.com/content/12404000/001234568.pdf": b"%PDF-official",
    }
    config = load_workflow_config(workflow_path)

    result = run_workflow(config, url_reader=fake_url_reader_factory(responses))

    assert result["status"] == "completed"
    assert Path(result["snapshot_json"]).exists()
    assert Path(result["sqlite_output"]).exists()
