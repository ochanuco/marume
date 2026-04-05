from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol
from urllib.parse import urlparse

from marume_data.transform import DPCSourceLink, parse_mhlw_dpc_page


class URLReaderResponse(Protocol):
    """Minimal response interface required by fetch helpers."""

    def __enter__(self) -> URLReaderResponse: ...
    def __exit__(self, exc_type: object, exc: object, tb: object) -> bool | None: ...
    def read(self) -> bytes: ...


class URLReader(Protocol):
    """Callable that returns a context-managed byte response for a URL."""

    def __call__(self, url: str) -> URLReaderResponse: ...


@dataclass(slots=True)
class DownloadedAsset:
    """One downloaded source asset recorded in the manifest."""

    kind: str
    label: str
    source_url: str
    path: str
    updated_at: str | None = None


def fetch_mhlw_dpc_assets(output_dir: Path, page_url: str, url_reader: URLReader) -> dict[str, object]:
    """Fetch the MHLW page and linked DPC source assets into a raw asset directory."""

    output_dir.mkdir(parents=True, exist_ok=True)
    html_bytes = _read_bytes(url_reader, page_url)
    html_path = output_dir / "mhlw_dpc_page.html"
    html_path.write_bytes(html_bytes)

    metadata = parse_mhlw_dpc_page(html=html_bytes.decode("utf-8"), base_url=page_url)
    downloaded_assets: list[DownloadedAsset] = []
    for link in metadata.dpc_links:
        asset = _download_asset(output_dir=output_dir, link=link, url_reader=url_reader)
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


def load_manifest(manifest_path: Path) -> dict[str, object]:
    """Load a manifest JSON file from disk."""

    return json.loads(manifest_path.read_text(encoding="utf-8"))


def resolve_page_path(manifest_path: Path) -> Path:
    """Resolve the fetched HTML page path recorded in a manifest."""

    manifest = load_manifest(manifest_path)
    page_path = manifest.get("page_path")
    if not page_path:
        raise KeyError(f"page_path is missing in manifest: {manifest_path}")
    return manifest_path.parent / str(page_path)


def resolve_rules_csv_path(manifest_path: Path) -> Path | None:
    """Resolve a rules CSV path from manifest assets or the default raw path."""

    manifest = load_manifest(manifest_path)
    for asset in manifest.get("assets", []):
        path = str(asset.get("path", ""))
        if path.endswith(".csv"):
            return manifest_path.parent / path
    default_path = manifest_path.parent / "dpc_rules.csv"
    if default_path.exists():
        return default_path
    return None


def resolve_latest_asset_path(manifest_path: Path, kind: str = "official") -> Path | None:
    """Resolve the newest DPC source asset path of a given kind from a manifest."""

    manifest = load_manifest(manifest_path)
    matched_assets = [
        asset
        for asset in manifest.get("assets", [])
        if asset.get("kind") == kind
    ]
    if not matched_assets:
        return None
    matched_assets.sort(key=lambda asset: str(asset.get("updated_at") or ""), reverse=True)
    return manifest_path.parent / str(matched_assets[0]["path"])


def _download_asset(output_dir: Path, link: DPCSourceLink, url_reader: URLReader) -> DownloadedAsset:
    """Download one linked DPC source asset and return its manifest entry."""

    kind = _classify_link_kind(link.label)
    updated_suffix = (link.updated_at or "unknown").replace("-", "")
    identifier = _build_asset_identifier(link.url)
    suffix = _build_asset_suffix(link.url)
    filename = f"dpc_{kind}_{updated_suffix}_{identifier}{suffix}"
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
    """Classify a DPC link label into official, provisional, or other."""

    if "正式版" in label:
        return "official"
    if "暫定版" in label:
        return "provisional"
    return "other"


def _read_bytes(url_reader: URLReader, url: str) -> bytes:
    """Read all bytes from a URL reader response."""

    with url_reader(url) as response:
        return response.read()


def _build_asset_identifier(url: str) -> str:
    """Build a stable filename identifier from a source URL."""

    path = Path(urlparse(url).path)
    if path.stem:
        return path.stem
    return "asset"


def _build_asset_suffix(url: str) -> str:
    """Build a filename suffix from the source URL, defaulting to .bin."""

    suffix = Path(urlparse(url).path).suffix.lower()
    if suffix:
        return suffix
    return ".bin"
