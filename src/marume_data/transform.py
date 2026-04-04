from __future__ import annotations

import json
from pathlib import Path


def write_placeholder_snapshot(output_path: Path, fiscal_year: int, source_url: str) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "rule_set": {
            "rule_set_id": f"dpc-{fiscal_year}",
            "fiscal_year": fiscal_year,
            "rule_version": f"{fiscal_year}.0.0-poc",
            "source_url": source_url,
            "source_published_at": None,
            "build_id": "manual",
            "built_at": None,
            "rules": [],
        },
        "icd_master": [],
        "procedure_master": [],
        "metadata": {
            "note": "placeholder snapshot; implement parser for MHLW source files next",
        },
    }
    output_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
