from __future__ import annotations

from collections import Counter
from collections.abc import Iterable
import json
import re
from dataclasses import asdict, dataclass
from pathlib import Path


# Source PDFs use full-width parentheses around ICD codes, so these patterns intentionally do the same.
ICD_PATTERN = re.compile("\uFF08([A-Z][0-9]{2}[0-9A-Z\\$]{0,2})\uFF09")
PROCEDURE_PATTERN = re.compile("(?<![\uFF08(])([K][0-9]{3,4})(?![\uFF09)])")
PROCEDURE_CONTEXT_PATTERN = re.compile("(?:手術|術|処置|施行)")
NARRATIVE_START_TOKENS = ("について", "場合", "入院", "施行", "判明", "発症", "併発", "疑い", "ため", "対し")
RESOURCE_DIAGNOSIS_PATTERN = re.compile(
    "(?:医療資源病名|医療資源を最も投入した傷病名)[^。]*?\uFF08([A-Z][0-9]{2}[0-9A-Z\\$]{0,2})\uFF09"
)
CURRENT_CLASSIFICATION_PATTERN = re.compile(
    "本分類[^。]*?\uFF08([A-Z][0-9]{2}[0-9A-Z\\$]{0,2})\uFF09[^。]*?が該当"
)
GUIDANCE_SELECTION_PATTERN = re.compile("([A-Z][0-9]{2}[0-9A-Z\\$]{0,2})\uFF09[^。]*?(?:を選択する|を選択|が該当)")
REQUIRED_STR_ERROR = "{key} must be a string"


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


@dataclass(slots=True)
class CaseInputCandidate:
    case_id: str
    fiscal_year: int
    main_diagnosis: str
    diagnoses: list[str]
    procedures: list[str]
    comorbidities: list[str]
    age: int | None = None
    sex: str = ""


def build_sample_case_candidates(
    extracted_cases: list[dict[str, object]],
    *,
    fiscal_year: int,
) -> list[SampleCaseCandidate]:
    """Convert extracted coding text cases into marume-friendly sample candidates."""

    if fiscal_year <= 0:
        raise ValueError(f"fiscal_year must be positive: {fiscal_year!r}")

    candidates: list[SampleCaseCandidate] = []
    for idx, row in enumerate(extracted_cases, start=1):
        if not isinstance(row, dict):
            raise TypeError(f"Row {idx} must be a dict/mapping, got {type(row).__name__}")
        dpc_code = _require_str(row, "dpc_code")
        raw_name = _require_str(row, "dpc_name")
        example_text = _require_str(row, "example_text")
        guidance_text = _require_str(row, "guidance_text")
        source_page = _parse_source_page(row, idx)

        dpc_name, leaked_example = split_dpc_name_and_example(raw_name)
        combined_example = _join_texts(leaked_example, example_text)
        combined_text = _join_texts(combined_example, guidance_text)
        raw_icd_matches = ICD_PATTERN.findall(combined_text)
        procedures = _extract_procedures(combined_example)

        main_diagnosis = _select_main_diagnosis(combined_text, guidance_text, raw_icd_matches)
        icd_codes = _dedupe(raw_icd_matches)
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


def build_case_input_candidates(candidates: Iterable[SampleCaseCandidate]) -> list[CaseInputCandidate]:
    """Build marume case-input candidates that can pass the CLI's minimum validation."""

    case_inputs: list[CaseInputCandidate] = []
    for candidate in candidates:
        if _case_input_skip_reason(candidate) is not None:
            continue
        case_inputs.append(
            CaseInputCandidate(
                case_id=candidate.case_id,
                fiscal_year=candidate.fiscal_year,
                main_diagnosis=candidate.main_diagnosis,
                diagnoses=list(candidate.diagnoses),
                procedures=list(candidate.procedures),
                comorbidities=list(candidate.comorbidities),
                age=candidate.age,
                sex=candidate.sex,
            )
        )
    return case_inputs


def build_case_input_candidate_report(
    candidates: Iterable[SampleCaseCandidate],
) -> dict[str, object]:
    """Summarize generated case inputs and review-oriented skip counts."""

    total_count = 0
    note_counts: Counter[str] = Counter()
    skipped_reasons: Counter[str] = Counter()
    skipped_count = 0
    for candidate in candidates:
        total_count += 1
        note_counts.update(candidate.notes)
        if reason := _case_input_skip_reason(candidate):
            skipped_count += 1
            skipped_reasons[reason] += 1

    case_input_candidates = total_count - skipped_count

    return {
        "total_candidates": total_count,
        "case_input_candidates": case_input_candidates,
        "skipped_candidates": skipped_count,
        "skipped_reasons": dict(skipped_reasons),
        "review_note_counts": dict(note_counts),
    }


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


def write_case_input_candidates_jsonl(output_path: Path, candidates: list[CaseInputCandidate]) -> None:
    """Write marume case-input candidates as UTF-8 JSONL."""

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as f:
        for candidate in candidates:
            f.write(json.dumps(_case_input_payload(candidate), ensure_ascii=False) + "\n")


def write_case_input_candidate_report_json(output_path: Path, report: dict[str, object]) -> None:
    """Write a compact generation report as UTF-8 JSON."""

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _case_input_payload(candidate: CaseInputCandidate) -> dict[str, object]:
    payload: dict[str, object] = {
        "case_id": candidate.case_id,
        "fiscal_year": candidate.fiscal_year,
        "sex": candidate.sex,
        "main_diagnosis": candidate.main_diagnosis,
        "diagnoses": candidate.diagnoses,
        "procedures": candidate.procedures,
        "comorbidities": candidate.comorbidities,
    }
    if candidate.age is not None:
        payload["age"] = candidate.age
    return payload


def _case_input_skip_reason(candidate: SampleCaseCandidate) -> str | None:
    if not candidate.main_diagnosis.strip():
        return "main_diagnosis 未抽出"
    return None


def _select_main_diagnosis(combined_text: str, guidance_text: str, icd_codes: list[str]) -> str:
    # Prefer the last matched guidance because later sentences in the PDF tend to refine earlier ones.
    resource_codes = RESOURCE_DIAGNOSIS_PATTERN.findall(guidance_text)
    if resource_codes:
        return resource_codes[-1]

    current_classification_codes = CURRENT_CLASSIFICATION_PATTERN.findall(combined_text)
    if current_classification_codes:
        return current_classification_codes[-1]

    preferred = GUIDANCE_SELECTION_PATTERN.findall(combined_text)
    if preferred:
        return preferred[-1]

    guidance_codes = ICD_PATTERN.findall(guidance_text)
    if guidance_codes:
        return guidance_codes[-1]

    return icd_codes[-1] if icd_codes else ""


def _extract_procedures(*texts: str) -> list[str]:
    text = " ".join(texts)
    procedures: list[str] = []
    for match in PROCEDURE_PATTERN.finditer(text):
        context = text[max(0, match.start() - 20) : match.end() + 20]
        if PROCEDURE_CONTEXT_PATTERN.search(context):
            procedures.append(match.group(1))
    return _dedupe(procedures)


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
    """Find the best split index for leaked narrative text.

    The caller passes only positive markers, but this helper returns 0 defensively
    when marker is zero or negative.
    """
    if marker <= 0:
        return 0
    punctuation = max(text.rfind("。", 0, marker), text.rfind("、", 0, marker))
    if punctuation >= 0:
        return punctuation + 1
    prev_space = text.rfind(" ", 0, marker)
    return prev_space + 1 if prev_space >= 0 else marker


def _join_texts(*parts: str) -> str:
    return " ".join(part.strip() for part in parts if part and part.strip())


def _normalize_space(text: str) -> str:
    return re.sub(r"[ \t\u3000]+", " ", text).strip()


def _dedupe(values: Iterable[str]) -> list[str]:
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
        raise TypeError(REQUIRED_STR_ERROR.format(key=key))
    return value


def _parse_source_page(row: dict[str, object], row_index: int) -> int:
    value = row.get("source_page", 0)
    if value in (None, ""):
        return 0
    if isinstance(value, bool):
        raise TypeError(f"source_page must be an integer at row {row_index}")
    if isinstance(value, int):
        if value < 0:
            raise TypeError(f"source_page must be non-negative at row {row_index}: {value!r}")
        return value
    if isinstance(value, str):
        stripped = value.strip()
        if not stripped:
            return 0
        try:
            result = int(stripped)
            if result < 0:
                raise TypeError(f"source_page must be non-negative at row {row_index}: {value!r}")
            return result
        except ValueError as err:
            raise TypeError(f"source_page must be an integer at row {row_index}: {value!r}") from err
    raise TypeError(f"source_page must be an integer at row {row_index}: {value!r}")
