from __future__ import annotations

import json
import re
from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin


@dataclass(slots=True)
class DPCSourceLink:
    label: str
    url: str
    updated_at: str | None = None


@dataclass(slots=True)
class DPCPageMetadata:
    title: str | None
    dpc_links: list[DPCSourceLink]


class _MHLWPageParser(HTMLParser):
    def __init__(self, base_url: str) -> None:
        super().__init__()
        self.base_url = base_url
        self.in_title = False
        self.title_parts: list[str] = []
        self.current_href: str | None = None
        self.current_anchor_parts: list[str] = []
        self.anchor_results: list[tuple[str, str]] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attrs_dict = dict(attrs)
        if tag == "title":
            self.in_title = True
        if tag == "a" and attrs_dict.get("href"):
            self.current_href = urljoin(self.base_url, attrs_dict["href"])
            self.current_anchor_parts = []

    def handle_endtag(self, tag: str) -> None:
        if tag == "title":
            self.in_title = False
        if tag == "a" and self.current_href:
            anchor_text = _normalize_space("".join(self.current_anchor_parts))
            if anchor_text:
                self.anchor_results.append((anchor_text, self.current_href))
            self.current_href = None
            self.current_anchor_parts = []

    def handle_data(self, data: str) -> None:
        if self.in_title:
            self.title_parts.append(data)
        if self.current_href:
            self.current_anchor_parts.append(data)


def write_snapshot_from_mhlw_html(
    input_path: Path,
    output_path: Path,
    fiscal_year: int,
    source_url: str,
) -> None:
    html = input_path.read_text(encoding="utf-8")
    metadata = parse_mhlw_dpc_page(html=html, base_url=source_url)
    payload = build_snapshot_payload(
        fiscal_year=fiscal_year,
        source_url=source_url,
        page_metadata=metadata,
    )
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def build_snapshot_payload(
    fiscal_year: int,
    source_url: str,
    page_metadata: DPCPageMetadata,
) -> dict[str, object]:
    latest_link = page_metadata.dpc_links[0] if page_metadata.dpc_links else None
    return {
        "rule_set": {
            "rule_set_id": f"dpc-{fiscal_year}",
            "fiscal_year": fiscal_year,
            "rule_version": _derive_rule_version(fiscal_year, latest_link),
            "source_url": source_url,
            "source_published_at": latest_link.updated_at if latest_link else None,
            "build_id": "manual",
            "built_at": None,
            "rules": [],
        },
        "icd_master": [],
        "procedure_master": [],
        "metadata": {
            "note": "rules are not parsed yet; page metadata and source links were extracted",
            "source_title": page_metadata.title or "",
            "dpc_link_count": str(len(page_metadata.dpc_links)),
            "latest_dpc_url": latest_link.url if latest_link else "",
        },
        "source_links": [
            {"label": link.label, "url": link.url, "updated_at": link.updated_at}
            for link in page_metadata.dpc_links
        ],
    }


def parse_mhlw_dpc_page(html: str, base_url: str) -> DPCPageMetadata:
    parser = _MHLWPageParser(base_url=base_url)
    parser.feed(html)

    links: list[DPCSourceLink] = []
    for label, href in parser.anchor_results:
        if "診断群分類" not in label or "電子点数表" not in label:
            continue
        links.append(
            DPCSourceLink(
                label=label,
                url=href,
                updated_at=_extract_updated_at(label),
            )
        )

    links.sort(key=lambda link: link.updated_at or "", reverse=True)
    return DPCPageMetadata(
        title=_normalize_space("".join(parser.title_parts)) or None,
        dpc_links=links,
    )


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


def _derive_rule_version(fiscal_year: int, latest_link: DPCSourceLink | None) -> str:
    if latest_link and latest_link.updated_at:
        return f"{fiscal_year}.{latest_link.updated_at.replace('-', '')}"
    return f"{fiscal_year}.0.0-poc"


def _extract_updated_at(text: str) -> str | None:
    match = re.search(r"令和(?P<era_year>\d+)年(?P<month>\d+)月(?P<day>\d+)日更新", text)
    if not match:
        return None
    western_year = 2018 + int(match.group("era_year"))
    month = int(match.group("month"))
    day = int(match.group("day"))
    return f"{western_year:04d}-{month:02d}-{day:02d}"


def _normalize_space(text: str) -> str:
    return " ".join(text.replace("\u3000", " ").split())
