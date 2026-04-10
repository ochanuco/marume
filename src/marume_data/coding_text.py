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
PAGE_MARKER_PATTERN = re.compile(r"^<<PAGE:(\d+)>>$")


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
    """Extract DPC coding cases from the appendix section of a coding text PDF.

    When `start_page` is provided, parsing starts from that page immediately and
    appendix-heading detection is skipped.
    """

    reader = PdfReader(str(pdf_path))
    page_count = len(reader.pages)
    page_start = start_page if start_page is not None else 1
    page_end = end_page if end_page is not None else page_count
    if page_start < 1:
        raise ValueError(f"start_page {start_page} must be at least 1")
    if page_end < 1:
        raise ValueError(f"end_page {end_page} must be at least 1")
    if page_start > page_count:
        raise ValueError(f"start_page {page_start} exceeds page count {page_count}")
    if page_end > page_count:
        raise ValueError(f"end_page {page_end} exceeds page count {page_count}")
    if page_end < page_start:
        raise ValueError(f"end_page {page_end} is before start_page {page_start}")

    in_appendix = start_page is not None
    combined_lines: list[str] = []
    for page_no in range(page_start, page_end + 1):
        page_text = reader.pages[page_no - 1].extract_text() or ""
        if not in_appendix and _contains_appendix_heading(page_text):
            in_appendix = True
        if not in_appendix:
            continue
        combined_lines.append(f"<<PAGE:{page_no}>>")
        combined_lines.extend(page_text.splitlines())
    return _parse_coding_cases_from_lines(combined_lines, default_source_page=page_start)


def parse_coding_cases_from_text(text: str, *, source_page: int) -> list[CodingTextCase]:
    """Parse coding case rows from one appendix page worth of extracted text."""

    return _parse_coding_cases_from_lines(text.splitlines(), default_source_page=source_page)


def _parse_coding_cases_from_lines(lines: list[str], *, default_source_page: int) -> list[CodingTextCase]:
    cleaned = _clean_lines(lines)
    blocks: list[list[str]] = []
    current: list[str] = []
    current_source_page = default_source_page
    block_source_page = default_source_page
    block_source_pages: list[int] = []

    for line in cleaned:
        page_match = PAGE_MARKER_PATTERN.match(line)
        if page_match is not None:
            current_source_page = int(page_match.group(1))
            continue
        if _contains_appendix_heading(line) or _is_header_line(line):
            continue
        if CASE_CODE_PATTERN.match(line):
            if current:
                blocks.append(current)
                block_source_pages.append(block_source_page)
            current = [line]
            block_source_page = current_source_page
            continue
        if current:
            current.append(line)

    if current:
        blocks.append(current)
        block_source_pages.append(block_source_page)

    parsed: list[CodingTextCase] = []
    for block, source_page in zip(blocks, block_source_pages, strict=True):
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
    if not block:
        return None
    match = CASE_CODE_PATTERN.match(block[0])
    if match is None:
        return None

    dpc_code = match.group("code")
    remaining_lines = []
    if match.group("name"):
        remaining_lines.append(match.group("name"))
    remaining_lines.extend(block[1:])
    remaining_lines = [line for line in remaining_lines if line]
    if not remaining_lines:
        return None

    name_lines: list[str] = []
    idx = 0
    while idx < len(remaining_lines):
        if _looks_like_narrative(remaining_lines[idx]):
            break
        name_lines.append(remaining_lines[idx])
        idx += 1

    narrative = remaining_lines[idx:]
    if not name_lines and narrative:
        name_lines.append(narrative[0])
        narrative = narrative[1:]
        while narrative and not _looks_like_narrative(narrative[0]):
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
        raw_text=_join_lines(remaining_lines),
        source_page=source_page,
    )


def _find_guidance_start(lines: list[str]) -> int | None:
    for idx, line in enumerate(lines):
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
    return compact.startswith(("別添", "付録", "Ⅴ.付録", "DPC上6桁", "DPC上６桁", "DPC名称")) or compact in ("事例", "対応")


def _looks_like_narrative(line: str) -> bool:
    stripped = line.strip()
    if any(char in line for char in ("。", "．", "!", "?", "！", "？")):
        return True
    if any(stripped.startswith(prefix) for prefix in ("説明", "備考", "（", "(")):
        return True
    # Check for hiragana/katakana (actual narrative indicators), not just kanji
    if re.search(r'[\u3040-\u309F\u30A0-\u30FF]', stripped):
        return True
    return False


def _join_lines(lines: list[str]) -> str:
    return " ".join(line.strip() for line in lines if line.strip())


def _contains_appendix_heading(text: str) -> bool:
    return APPENDIX_HEADING_PATTERN.search(_normalize_text(text)) is not None
