from __future__ import annotations

import urllib.request

from marume_data.fetch import URLReaderResponse


def url_reader_with_timeout(url: str) -> URLReaderResponse:
    return urllib.request.urlopen(url, timeout=10)  # noqa: S310
