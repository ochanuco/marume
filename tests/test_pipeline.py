from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from shutil import which


def _run_python_script(repo_root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    """Run a project Python script via uv and capture stdout/stderr for assertions."""

    uv = which("uv")
    assert uv is not None
    return subprocess.run(  # noqa: S603
        [
            uv,
            "run",
            sys.executable,
            *args,
        ],
        check=False,
        cwd=repo_root,
        capture_output=True,
        text=True,
    )


def test_manifestからtransformしてsqliteまで作れる(tmp_path) -> None:
    fixture_dir = Path(__file__).with_name("fixtures")
    (tmp_path / "mhlw_dpc_page.html").write_text(
        (fixture_dir / "mhlw_dpc_page.html").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    (tmp_path / "dpc_rules.csv").write_text(
        (fixture_dir / "dpc_rules.csv").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    manifest = {
        "page_url": "https://example.com/mhlw_dpc_page.html",
        "page_path": "mhlw_dpc_page.html",
        "source_title": "令和８年度診療報酬改定について｜厚生労働省",
        "assets": [],
    }
    (tmp_path / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False), encoding="utf-8")

    snapshot_path = tmp_path / "snapshot.json"
    sqlite_path = tmp_path / "rules.sqlite"
    repo_root = Path(__file__).resolve().parents[1]

    result = _run_python_script(
        repo_root,
        "scripts/transform_dpc.py",
        "--manifest",
        str(tmp_path / "manifest.json"),
        "--fiscal-year",
        "2026",
        "--output",
        str(snapshot_path),
    )
    assert result.returncode == 0, f"transform_dpc.py failed:\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"

    result = _run_python_script(
        repo_root,
        "scripts/build_sqlite.py",
        "--input",
        str(snapshot_path),
        "--output",
        str(sqlite_path),
    )
    assert result.returncode == 0, f"build_sqlite.py failed:\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"

    payload = json.loads(snapshot_path.read_text(encoding="utf-8"))
    assert payload["metadata"]["rule_count"] == "2"
    assert sqlite_path.exists()


def test_transformはinputとmanifestの同時指定を拒否する(tmp_path) -> None:
    fixture_dir = Path(__file__).with_name("fixtures")
    input_path = tmp_path / "mhlw_dpc_page.html"
    input_path.write_text(
        (fixture_dir / "mhlw_dpc_page.html").read_text(encoding="utf-8"),
        encoding="utf-8",
    )
    manifest = {
        "page_url": "https://example.com/mhlw_dpc_page.html",
        "page_path": "mhlw_dpc_page.html",
        "source_title": "令和８年度診療報酬改定について｜厚生労働省",
        "assets": [],
    }
    manifest_path = tmp_path / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False), encoding="utf-8")
    snapshot_path = tmp_path / "snapshot.json"
    repo_root = Path(__file__).resolve().parents[1]

    result = _run_python_script(
        repo_root,
        "scripts/transform_dpc.py",
        "--input",
        str(input_path),
        "--manifest",
        str(manifest_path),
        "--fiscal-year",
        "2026",
        "--source-url",
        "https://example.com/mhlw_dpc_page.html",
        "--output",
        str(snapshot_path),
    )

    assert result.returncode == 1
    assert "--input と --manifest は同時に指定できません" in result.stderr
