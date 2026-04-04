from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

from marume_data.extract import scaffold_rules_csv_from_manifest
from marume_data.fetch import URLReader, fetch_mhlw_dpc_assets
from marume_data.sqlite_builder import create_snapshot_database, load_snapshot_json
from marume_data.transform import parse_mhlw_dpc_page, write_snapshot_from_sources


@dataclass(slots=True)
class WorkflowPaths:
    raw_dir: Path
    rules_csv: Path
    snapshot_json: Path
    sqlite_output: Path


@dataclass(slots=True)
class WorkflowConfig:
    source_url: str
    fiscal_year: int
    paths: WorkflowPaths


def load_workflow_config(path: Path) -> WorkflowConfig:
    raw = json.loads(path.read_text(encoding="utf-8"))
    paths = raw["paths"]
    return WorkflowConfig(
        source_url=str(raw["source_url"]),
        fiscal_year=int(raw["fiscal_year"]),
        paths=WorkflowPaths(
            raw_dir=Path(paths["raw_dir"]),
            rules_csv=Path(paths["rules_csv"]),
            snapshot_json=Path(paths["snapshot_json"]),
            sqlite_output=Path(paths["sqlite_output"]),
        ),
    )


def run_workflow(config: WorkflowConfig, url_reader: URLReader) -> dict[str, str]:
    manifest = fetch_mhlw_dpc_assets(
        output_dir=config.paths.raw_dir,
        page_url=config.source_url,
        url_reader=url_reader,
    )
    manifest_path = config.paths.raw_dir / "manifest.json"
    scaffold_rules_csv_from_manifest(
        manifest_path=manifest_path,
        output_csv_path=config.paths.rules_csv,
    )

    html = (config.paths.raw_dir / str(manifest["page_path"])).read_text(encoding="utf-8")
    metadata = parse_mhlw_dpc_page(html=html, base_url=config.source_url)
    write_snapshot_from_sources(
        output_path=config.paths.snapshot_json,
        fiscal_year=config.fiscal_year,
        source_url=config.source_url,
        page_metadata=metadata,
        rules_csv_path=config.paths.rules_csv,
    )

    snapshot = load_snapshot_json(config.paths.snapshot_json)
    create_snapshot_database(config.paths.sqlite_output, snapshot)
    return {
        "manifest": str(manifest_path),
        "rules_csv": str(config.paths.rules_csv),
        "snapshot_json": str(config.paths.snapshot_json),
        "sqlite_output": str(config.paths.sqlite_output),
    }
