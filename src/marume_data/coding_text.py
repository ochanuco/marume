from __future__ import annotations

import json
import re
from dataclasses import asdict, dataclass
from pathlib import Path

from pypdf import PdfReader


APPENDIX_HEADING_PATTERN = re.compile(r"DPC\s*上[6６]桁別\s*注意すべき\s*DPC\s*コーディングの事例集")
CASE_CODE_PATTERN = re.compile(r"^(?P<code>\d{6})(?:\s+(?P<name>.+))?$")
GUIDANCE_MARKERS = (
    "医療資源病名",
    "医療資源を最も投入した傷病名",
    "入院契機病名",
    "DPCコーディング",
    "を選択する",
    "を選択",
    "が該当",
    "として扱う",
    "に分類",
)


@dataclass(slots=True)
class CodingTextCase:
    dpc_code: str
    dpc_name: str
    example_text: str
    guidance_text: str
    raw_text: str
    source_page: int


def extract_coding_cases_from_pdf(
    pdf_path: Path,
    *,
    start_page: int | None = None,
    end_page: int | None = None,
) -> list[CodingTextCase]:
    """Extract DPC coding cases from the appendix section of a coding text PDF."""

    reader = PdfReader(str(pdf_path))
    page_count = len(reader.pages)
    page_start = max(1, start_page or 1)
    page_end = min(page_count, end_page or page_count)

    in_appendix = False
    extracted: list[CodingTextCase] = []
    for page_no in range(page_start, page_end + 1):
        page_text = reader.pages[page_no - 1].extract_text() or ""
        if not in_appendix and _contains_appendix_heading(page_text):
            in_appendix = True
        if not in_appendix:
            continue
        extracted.extend(parse_coding_cases_from_text(page_text, source_page=page_no))
    return extracted


def parse_coding_cases_from_text(text: str, *, source_page: int) -> list[CodingTextCase]:
    """Parse coding case rows from one appendix page worth of extracted text."""

    lines = _clean_lines(text.splitlines())
    blocks: list[list[str]] = []
    current: list[str] = []

    for line in lines:
        if _contains_appendix_heading(line) or _is_header_line(line):
            continue
        if CASE_CODE_PATTERN.match(line):
            if current:
                blocks.append(current)
            current = [line]
            continue
        if current:
            current.append(line)

    if current:
        blocks.append(current)

    parsed: list[CodingTextCase] = []
    for block in blocks:
        case = _parse_case_block(block, source_page=source_page)
        if case is not None:
            parsed.append(case)
    return parsed


def write_coding_cases_json(output_path: Path, cases: list[CodingTextCase]) -> None:
    """Write parsed coding cases as UTF-8 JSON."""

    output_path.parent.mkdir(parents=True, exist_ok=True)
    payload = [asdict(case) for case in cases]
    output_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def _parse_case_block(block: list[str], *, source_page: int) -> CodingTextCase | None:
    match = CASE_CODE_PATTERN.match(block[0])
    if match is None:
        return None

    dpc_code = match.group("code")
    remaining = []
    if match.group("name"):
        remaining.append(match.group("name"))
    remaining.extend(block[1:])
    remaining = [line for line in remaining if line]
    if not remaining:
        return None

    name_lines: list[str] = []
    idx = 0
    while idx < len(remaining):
        if _looks_like_narrative(remaining[idx]):
            break
        name_lines.append(remaining[idx])
        idx += 1

    narrative = remaining[idx:] or remaining[len(name_lines) :]
    if not name_lines and narrative:
        name_lines.append(narrative[0])
        narrative = narrative[1:]

    split_idx = _find_guidance_start(narrative)
    if split_idx is None:
        example_lines = narrative
        guidance_lines: list[str] = []
    else:
        example_lines = narrative[:split_idx]
        guidance_lines = narrative[split_idx:]

    return CodingTextCase(
        dpc_code=dpc_code,
        dpc_name=_join_lines(name_lines),
        example_text=_join_lines(example_lines),
        guidance_text=_join_lines(guidance_lines),
        raw_text=_join_lines(remaining),
        source_page=source_page,
    )


def _find_guidance_start(lines: list[str]) -> int | None:
    for idx, line in enumerate(lines):
        if idx == 0:
            continue
        if any(marker in line for marker in GUIDANCE_MARKERS):
            return idx
    return None


def _clean_lines(lines: list[str]) -> list[str]:
    cleaned: list[str] = []
    for line in lines:
        normalized = _normalize_text(line)
        if not normalized:
            continue
        if re.fullmatch(r"-\s*\d+\s*-", normalized):
            continue
        cleaned.append(normalized)
    return cleaned


def _normalize_text(text: str) -> str:
    return re.sub(r"[ \t\u3000]+", " ", text).strip()


def _is_header_line(line: str) -> bool:
    compact = re.sub(r"\s+", "", line)
    return (
        compact.startswith("別添")
        or compact.startswith("付録")
        or compact.startswith("Ⅴ.付録")
        or compact.startswith("DPC上6桁")
        or compact.startswith("DPC上６桁")
        or compact.startswith("DPC名称")
        or compact.startswith("事例")
        or compact.startswith("対応")
    )


def _looks_like_narrative(line: str) -> bool:
    return any(token in line for token in ("。", "場合", "入院", "術", "診断", "病名", "について"))


def _join_lines(lines: list[str]) -> str:
    return " ".join(line.strip() for line in lines if line.strip())


def _contains_appendix_heading(text: str) -> bool:
    return APPENDIX_HEADING_PATTERN.search(_normalize_text(text)) is not None
