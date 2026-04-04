from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from marume_data.transform import DPCSourceLink, parse_mhlw_dpc_page


class URLReader(Protocol):
    def __call__(self, url: str): ...


@dataclass(slots=True)
class DownloadedAsset:
    kind: str
    label: str
    source_url: str
    path: str
    updated_at: str | None = None


def fetch_mhlw_dpc_assets(output_dir: Path, page_url: str, url_reader: URLReader) -> dict[str, object]:
    output_dir.mkdir(parents=True, exist_ok=True)
    html_bytes = _read_bytes(url_reader, page_url)
    html_path = output_dir / "mhlw_dpc_page.html"
    html_path.write_bytes(html_bytes)

    metadata = parse_mhlw_dpc_page(html=html_bytes.decode("utf-8"), base_url=page_url)
    downloaded_assets: list[DownloadedAsset] = []
    for link in metadata.dpc_links:
        asset = _download_pdf_asset(output_dir=output_dir, link=link, url_reader=url_reader)
        downloaded_assets.append(asset)

    manifest = {
        "page_url": page_url,
        "page_path": html_path.name,
        "source_title": metadata.title or "",
        "assets": [
            {
                "kind": asset.kind,
                "label": asset.label,
                "source_url": asset.source_url,
                "path": asset.path,
                "updated_at": asset.updated_at,
            }
            for asset in downloaded_assets
        ],
    }
    (output_dir / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return manifest


def _download_pdf_asset(output_dir: Path, link: DPCSourceLink, url_reader: URLReader) -> DownloadedAsset:
    kind = _classify_link_kind(link.label)
    updated_suffix = (link.updated_at or "unknown").replace("-", "")
    filename = f"dpc_{kind}_{updated_suffix}.pdf"
    file_path = output_dir / filename
    file_path.write_bytes(_read_bytes(url_reader, link.url))
    return DownloadedAsset(
        kind=kind,
        label=link.label,
        source_url=link.url,
        path=file_path.name,
        updated_at=link.updated_at,
    )


def _classify_link_kind(label: str) -> str:
    if "正式版" in label:
        return "official"
    if "暫定版" in label:
        return "provisional"
    return "other"


def _read_bytes(url_reader: URLReader, url: str) -> bytes:
    with url_reader(url) as response:
        return response.read()
