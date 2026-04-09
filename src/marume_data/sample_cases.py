from __future__ import annotations

import json
import re
from dataclasses import asdict, dataclass
from pathlib import Path


ICD_PATTERN = re.compile(r"（([A-Z][0-9]{2}[0-9A-Z\$]{0,2})）")
PROCEDURE_PATTERN = re.compile(r"\b([A-Z][0-9]{3,4})\b")
NARRATIVE_START_TOKENS = ("について", "場合", "入院", "施行", "判明", "発症", "併発", "疑い", "ため", "対し")
RESOURCE_DIAGNOSIS_PATTERN = re.compile(r"医療資源病名[^。]*?（([A-Z][0-9]{2}[0-9A-Z\$]{0,2})）")
CURRENT_CLASSIFICATION_PATTERN = re.compile(r"本分類[^。]*?（([A-Z][0-9]{2}[0-9A-Z\$]{0,2})）[^。]*?が該当")
GUIDANCE_SELECTION_PATTERN = re.compile(r"([A-Z][0-9]{2}[0-9A-Z\$]{0,2})）[^。]*?(?:を選択する|を選択|が該当)")


@dataclass(slots=True)
class SampleCaseCandidate:
    case_id: str
    fiscal_year: int
    dpc_code_6: str
    dpc_name: str
    main_diagnosis: str
    diagnoses: list[str]
    procedures: list[str]
    comorbidities: list[str]
    age: int | None
    sex: str
    source_page: int
    example_text: str
    guidance_text: str
    notes: list[str]


def build_sample_case_candidates(
    extracted_cases: list[dict[str, object]],
    *,
    fiscal_year: int,
) -> list[SampleCaseCandidate]:
    """Convert extracted coding text cases into marume-friendly sample candidates."""

    candidates: list[SampleCaseCandidate] = []
    for idx, row in enumerate(extracted_cases, start=1):
        dpc_code = _require_str(row, "dpc_code")
        raw_name = _require_str(row, "dpc_name")
        example_text = _require_str(row, "example_text")
        guidance_text = _require_str(row, "guidance_text")
        source_page = int(row.get("source_page", 0) or 0)

        dpc_name, leaked_example = split_dpc_name_and_example(raw_name)
        combined_example = _join_texts(leaked_example, example_text)
        combined_text = _join_texts(combined_example, guidance_text)
        icd_codes = _dedupe(ICD_PATTERN.findall(combined_text))
        procedures = _extract_procedures(combined_example)

        main_diagnosis = _select_main_diagnosis(combined_text, guidance_text, icd_codes)
        diagnoses = [main_diagnosis] if main_diagnosis else []
        comorbidities = [code for code in icd_codes if code != main_diagnosis]
        notes = _build_notes(raw_name, leaked_example, icd_codes, procedures)

        candidates.append(
            SampleCaseCandidate(
                case_id=f"dpc-{dpc_code}-{idx:04d}",
                fiscal_year=fiscal_year,
                dpc_code_6=dpc_code,
                dpc_name=dpc_name,
                main_diagnosis=main_diagnosis,
                diagnoses=diagnoses,
                procedures=procedures,
                comorbidities=comorbidities,
                age=None,
                sex="",
                source_page=source_page,
                example_text=combined_example,
                guidance_text=guidance_text,
                notes=notes,
            )
        )
    return candidates


def split_dpc_name_and_example(raw_name: str) -> tuple[str, str]:
    """Split leaked example text from a DPC name field."""

    normalized = _normalize_space(raw_name)
    best_split_at: int | None = None
    for token in NARRATIVE_START_TOKENS:
        marker = normalized.find(token)
        if marker <= 0:
            continue
        split_at = _find_split_start(normalized, marker)
        if split_at <= 0:
            continue
        if best_split_at is None or split_at < best_split_at:
            best_split_at = split_at
    if best_split_at is not None:
        return normalized[:best_split_at].strip(), normalized[best_split_at:].strip()
    return normalized, ""


def write_sample_case_candidates_json(output_path: Path, candidates: list[SampleCaseCandidate]) -> None:
    """Write sample case candidates as UTF-8 JSON."""

    output_path.parent.mkdir(parents=True, exist_ok=True)
    payload = [asdict(candidate) for candidate in candidates]
    output_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _select_main_diagnosis(combined_text: str, guidance_text: str, icd_codes: list[str]) -> str:
    resource_codes = RESOURCE_DIAGNOSIS_PATTERN.findall(guidance_text)
    if resource_codes:
        return resource_codes[-1]

    current_classification_codes = CURRENT_CLASSIFICATION_PATTERN.findall(combined_text)
    if current_classification_codes:
        return current_classification_codes[0]

    preferred = GUIDANCE_SELECTION_PATTERN.findall(combined_text)
    if preferred:
        return preferred[-1]

    guidance_codes = ICD_PATTERN.findall(guidance_text)
    if guidance_codes:
        return guidance_codes[-1]

    return icd_codes[-1] if icd_codes else ""


def _extract_procedures(*texts: str) -> list[str]:
    return _dedupe(PROCEDURE_PATTERN.findall(" ".join(texts)))


def _build_notes(raw_name: str, leaked_example: str, icd_codes: list[str], procedures: list[str]) -> list[str]:
    notes: list[str] = []
    if leaked_example:
        notes.append("dpc_name から事例文の食い込みを補正")
    if not icd_codes:
        notes.append("ICD コード未抽出")
    if not procedures:
        notes.append("処置コード未抽出")
    if raw_name != _normalize_space(raw_name):
        notes.append("空白正規化あり")
    return notes


def _find_split_start(text: str, marker: int) -> int:
    punctuation = max(text.rfind("。", 0, marker), text.rfind("、", 0, marker))
    if punctuation >= 0:
        return punctuation + 1
    prev_space = text.rfind(" ", 0, marker)
    return prev_space + 1 if prev_space >= 0 else marker


def _join_texts(*parts: str) -> str:
    return " ".join(part.strip() for part in parts if part and part.strip())


def _normalize_space(text: str) -> str:
    return re.sub(r"[ \t\u3000]+", " ", text).strip()


def _dedupe(values: list[str] | tuple[str, ...] | object) -> list[str]:
    seen: set[str] = set()
    ordered: list[str] = []
    for value in values:
        if value in seen:
            continue
        seen.add(value)
        ordered.append(value)
    return ordered


def _require_str(row: dict[str, object], key: str) -> str:
    value = row.get(key)
    if not isinstance(value, str):
        raise ValueError(f"{key} must be a string")
    return value
