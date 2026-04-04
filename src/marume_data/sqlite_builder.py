from __future__ import annotations

import json
import sqlite3
from pathlib import Path

from marume_data.models import ICDMasterRow, ProcedureMasterRow, Rule, RuleCondition, RuleSet, Snapshot


SCHEMA_STATEMENTS = (
    """
    CREATE TABLE IF NOT EXISTS rule_sets (
        rule_set_id TEXT PRIMARY KEY,
        fiscal_year INTEGER NOT NULL,
        rule_version TEXT NOT NULL,
        source_url TEXT,
        source_published_at TEXT,
        build_id TEXT,
        built_at TEXT
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS rules (
        rule_id TEXT PRIMARY KEY,
        rule_set_id TEXT NOT NULL,
        priority INTEGER NOT NULL,
        dpc_code TEXT NOT NULL,
        mdc_code TEXT,
        label TEXT,
        FOREIGN KEY(rule_set_id) REFERENCES rule_sets(rule_set_id)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS rule_conditions (
        condition_id TEXT PRIMARY KEY,
        rule_id TEXT NOT NULL,
        condition_type TEXT NOT NULL,
        operator TEXT NOT NULL,
        value_text TEXT,
        value_num REAL,
        value_json TEXT,
        negated INTEGER NOT NULL DEFAULT 0,
        FOREIGN KEY(rule_id) REFERENCES rules(rule_id)
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS icd_master (
        icd_code TEXT PRIMARY KEY,
        name_ja TEXT,
        classification_code TEXT,
        source_system TEXT,
        valid_from TEXT,
        valid_to TEXT
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS procedure_master (
        procedure_code TEXT PRIMARY KEY,
        name_ja TEXT,
        code_system TEXT,
        valid_from TEXT,
        valid_to TEXT
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS metadata (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL
    )
    """,
)


def create_snapshot_database(output_path: Path, snapshot: Snapshot) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(output_path) as connection:
        connection.execute("PRAGMA foreign_keys = ON")
        for statement in SCHEMA_STATEMENTS:
            connection.execute(statement)

        _replace_rule_set(connection, snapshot.rule_set)
        _replace_rules(connection, snapshot.rule_set)
        _replace_icd_master(connection, snapshot.icd_master)
        _replace_procedure_master(connection, snapshot.procedure_master)
        _replace_metadata(connection, snapshot.metadata)
        connection.commit()


def load_snapshot_json(path: Path) -> Snapshot:
    raw = json.loads(path.read_text(encoding="utf-8"))
    rule_set_raw = raw["rule_set"]
    rules: list[Rule] = []
    for rule_raw in rule_set_raw.get("rules", []):
        conditions = [
            RuleCondition(
                condition_id=condition_raw["condition_id"],
                condition_type=condition_raw["condition_type"],
                operator=condition_raw["operator"],
                value_text=condition_raw.get("value_text"),
                value_num=condition_raw.get("value_num"),
                value_json=_normalize_json_value(condition_raw.get("value_json")),
                negated=bool(condition_raw.get("negated", False)),
            )
            for condition_raw in rule_raw.get("conditions", [])
        ]
        rules.append(
            Rule(
                rule_id=rule_raw["rule_id"],
                priority=rule_raw["priority"],
                dpc_code=rule_raw["dpc_code"],
                mdc_code=rule_raw.get("mdc_code"),
                label=rule_raw.get("label"),
                conditions=conditions,
            )
        )

    rule_set = RuleSet(
        rule_set_id=rule_set_raw["rule_set_id"],
        fiscal_year=rule_set_raw["fiscal_year"],
        rule_version=rule_set_raw["rule_version"],
        source_url=rule_set_raw.get("source_url"),
        source_published_at=rule_set_raw.get("source_published_at"),
        build_id=rule_set_raw.get("build_id"),
        built_at=rule_set_raw.get("built_at"),
        rules=rules,
    )

    return Snapshot(
        rule_set=rule_set,
        icd_master=[ICDMasterRow(**row) for row in raw.get("icd_master", [])],
        procedure_master=[ProcedureMasterRow(**row) for row in raw.get("procedure_master", [])],
        metadata={str(key): str(value) for key, value in raw.get("metadata", {}).items()},
    )


def _replace_rule_set(connection: sqlite3.Connection, rule_set: RuleSet) -> None:
    connection.execute("DELETE FROM rule_conditions")
    connection.execute("DELETE FROM rules")
    connection.execute("DELETE FROM rule_sets")
    connection.execute(
        """
        INSERT INTO rule_sets (
            rule_set_id, fiscal_year, rule_version, source_url, source_published_at, build_id, built_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
        (
            rule_set.rule_set_id,
            rule_set.fiscal_year,
            rule_set.rule_version,
            rule_set.source_url,
            rule_set.source_published_at,
            rule_set.build_id,
            rule_set.built_at,
        ),
    )


def _replace_rules(connection: sqlite3.Connection, rule_set: RuleSet) -> None:
    for rule in rule_set.rules:
        connection.execute(
            """
            INSERT INTO rules (rule_id, rule_set_id, priority, dpc_code, mdc_code, label)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (rule.rule_id, rule_set.rule_set_id, rule.priority, rule.dpc_code, rule.mdc_code, rule.label),
        )
        for condition in rule.conditions:
            connection.execute(
                """
                INSERT INTO rule_conditions (
                    condition_id, rule_id, condition_type, operator, value_text, value_num, value_json, negated
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    condition.condition_id,
                    rule.rule_id,
                    condition.condition_type,
                    condition.operator,
                    condition.value_text,
                    condition.value_num,
                    condition.value_json,
                    int(condition.negated),
                ),
            )


def _replace_icd_master(connection: sqlite3.Connection, rows: list[ICDMasterRow]) -> None:
    connection.execute("DELETE FROM icd_master")
    connection.executemany(
        """
        INSERT INTO icd_master (icd_code, name_ja, classification_code, source_system, valid_from, valid_to)
        VALUES (?, ?, ?, ?, ?, ?)
        """,
        [
            (
                row.icd_code,
                row.name_ja,
                row.classification_code,
                row.source_system,
                row.valid_from,
                row.valid_to,
            )
            for row in rows
        ],
    )


def _replace_procedure_master(connection: sqlite3.Connection, rows: list[ProcedureMasterRow]) -> None:
    connection.execute("DELETE FROM procedure_master")
    connection.executemany(
        """
        INSERT INTO procedure_master (procedure_code, name_ja, code_system, valid_from, valid_to)
        VALUES (?, ?, ?, ?, ?)
        """,
        [(row.procedure_code, row.name_ja, row.code_system, row.valid_from, row.valid_to) for row in rows],
    )


def _replace_metadata(connection: sqlite3.Connection, metadata: dict[str, str]) -> None:
    connection.execute("DELETE FROM metadata")
    connection.executemany(
        "INSERT INTO metadata (key, value) VALUES (?, ?)",
        [(key, value) for key, value in metadata.items()],
    )


def _normalize_json_value(value: object) -> str | None:
    if value is None:
        return None
    if isinstance(value, str):
        return value
    return json.dumps(value, ensure_ascii=False, sort_keys=True)
