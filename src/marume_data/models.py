from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(slots=True)
class RuleCondition:
    """One normalized rule condition stored in the snapshot."""

    condition_id: str
    condition_type: str
    operator: str
    value_text: str | None = None
    value_num: float | None = None
    value_json: str | None = None
    negated: bool = False


@dataclass(slots=True)
class Rule:
    """One normalized DPC rule with expanded conditions."""

    rule_id: str
    priority: int
    dpc_code: str
    mdc_code: str | None = None
    label: str | None = None
    conditions: list[RuleCondition] = field(default_factory=list)


@dataclass(slots=True)
class RuleSet:
    """A versioned collection of normalized DPC rules."""

    rule_set_id: str
    fiscal_year: int
    rule_version: str
    source_url: str | None = None
    source_published_at: str | None = None
    build_id: str | None = None
    built_at: str | None = None
    rules: list[Rule] = field(default_factory=list)


@dataclass(slots=True)
class ICDMasterRow:
    """One ICD master row included in the snapshot."""

    icd_code: str
    name_ja: str | None = None
    classification_code: str | None = None
    source_system: str | None = None
    valid_from: str | None = None
    valid_to: str | None = None


@dataclass(slots=True)
class ProcedureMasterRow:
    """One procedure master row included in the snapshot."""

    procedure_code: str
    name_ja: str | None = None
    code_system: str | None = None
    valid_from: str | None = None
    valid_to: str | None = None


@dataclass(slots=True)
class Snapshot:
    """Top-level snapshot payload used to build SQLite output."""

    rule_set: RuleSet
    icd_master: list[ICDMasterRow] = field(default_factory=list)
    procedure_master: list[ProcedureMasterRow] = field(default_factory=list)
    metadata: dict[str, str] = field(default_factory=dict)
