from __future__ import annotations

import json
import subprocess
from pathlib import Path


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
        "page_url": "https://www.mhlw.go.jp/stf/newpage_67729.html",
        "page_path": "mhlw_dpc_page.html",
        "source_title": "令和８年度診療報酬改定について｜厚生労働省",
        "assets": [],
    }
    (tmp_path / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False), encoding="utf-8")

    snapshot_path = tmp_path / "snapshot.json"
    sqlite_path = tmp_path / "rules.sqlite"
    repo_root = Path(__file__).resolve().parents[1]

    subprocess.run(
        [
            "uv",
            "run",
            "python",
            "scripts/transform_dpc.py",
            "--manifest",
            str(tmp_path / "manifest.json"),
            "--output",
            str(snapshot_path),
        ],
        check=True,
        cwd=repo_root,
    )
    subprocess.run(
        [
            "uv",
            "run",
            "python",
            "scripts/build_sqlite.py",
            "--input",
            str(snapshot_path),
            "--output",
            str(sqlite_path),
        ],
        check=True,
        cwd=repo_root,
    )

    payload = json.loads(snapshot_path.read_text(encoding="utf-8"))
    assert payload["metadata"]["rule_count"] == "2"
    assert sqlite_path.exists()
