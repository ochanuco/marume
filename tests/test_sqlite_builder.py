from __future__ import annotations

import sqlite3

from marume_data.sqlite_builder import create_snapshot_database
from marume_data.models import ICDMasterRow, ProcedureMasterRow, Rule, RuleCondition, RuleSet, Snapshot


def test_create_snapshot_database_writes_minimum_tables(tmp_path) -> None:
    output_path = tmp_path / "rules-2026.sqlite"
    snapshot = Snapshot(
        rule_set=RuleSet(
            rule_set_id="dpc-2026",
            fiscal_year=2026,
            rule_version="2026.0.0-poc",
            source_url="https://example.com/dpc",
            build_id="test-build",
            built_at="2026-04-05T00:00:00Z",
            rules=[
                Rule(
                    rule_id="R-2026-0001",
                    priority=10,
                    dpc_code="040080xx99x0xx",
                    mdc_code="04",
                    label="心筋梗塞POC",
                    conditions=[
                        RuleCondition(
                            condition_id="C-1",
                            condition_type="main_diagnosis",
                            operator="eq",
                            value_text="I219",
                        ),
                        RuleCondition(
                            condition_id="C-2",
                            condition_type="procedure",
                            operator="in",
                            value_json='["K549"]',
                        ),
                    ],
                )
            ],
        ),
        icd_master=[
            ICDMasterRow(
                icd_code="I219",
                name_ja="急性心筋梗塞",
                classification_code="I21.9",
                source_system="icd10",
            )
        ],
        procedure_master=[
            ProcedureMasterRow(
                procedure_code="K549",
                name_ja="経皮的冠動脈形成術",
                code_system="k-code",
            )
        ],
        metadata={"source": "pytest"},
    )

    create_snapshot_database(output_path, snapshot)

    with sqlite3.connect(output_path) as connection:
        assert _count_rows(connection, "rule_sets") == 1
        assert _count_rows(connection, "rules") == 1
        assert _count_rows(connection, "rule_conditions") == 2
        assert _count_rows(connection, "icd_master") == 1
        assert _count_rows(connection, "procedure_master") == 1
        assert _metadata_value(connection, "source") == "pytest"


def _count_rows(connection: sqlite3.Connection, table_name: str) -> int:
    return connection.execute(f"SELECT COUNT(*) FROM {table_name}").fetchone()[0]


def _metadata_value(connection: sqlite3.Connection, key: str) -> str:
    return connection.execute("SELECT value FROM metadata WHERE key = ?", (key,)).fetchone()[0]
