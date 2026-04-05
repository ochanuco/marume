from __future__ import annotations

from collections.abc import Callable

import pytest


class FakeResponse:
    """Minimal fake response object for URL reader tests."""

    def __init__(self, body: bytes) -> None:
        self._body = body

    def __enter__(self) -> FakeResponse:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> bool | None:
        return None

    def read(self) -> bytes:
        return self._body


type FakeURLReader = Callable[[str], FakeResponse]


def build_fake_url_reader(responses: dict[str, bytes]) -> FakeURLReader:
    """Build a deterministic URL reader backed by in-memory byte responses."""

    def _reader(url: str) -> FakeResponse:
        if url not in responses:
            raise KeyError(f"build_fake_url_reader: response is not defined for URL: {url}")
        return FakeResponse(responses[url])

    return _reader


@pytest.fixture
def fake_url_reader_factory() -> Callable[[dict[str, bytes]], FakeURLReader]:
    """Provide the fake URL reader builder as a pytest fixture."""

    return build_fake_url_reader
