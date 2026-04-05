from __future__ import annotations

from collections.abc import Callable
from typing import TypeAlias

import pytest


class FakeResponse:
    def __init__(self, body: bytes) -> None:
        self._body = body

    def __enter__(self) -> FakeResponse:
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> bool | None:
        return None

    def read(self) -> bytes:
        return self._body


FakeURLReader: TypeAlias = Callable[[str], FakeResponse]


def build_fake_url_reader(responses: dict[str, bytes]) -> FakeURLReader:
    def _reader(url: str) -> FakeResponse:
        return FakeResponse(responses[url])

    return _reader


@pytest.fixture
def fake_url_reader_factory() -> Callable[[dict[str, bytes]], FakeURLReader]:
    return build_fake_url_reader
