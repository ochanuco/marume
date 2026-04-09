from __future__ import annotations

from pathlib import Path

import pytest

from marume_data.coding_text import _parse_coding_cases_from_lines, parse_coding_cases_from_text


def test_appendix_page_textから複数事例を抽出できる() -> None:
    text = """
    DPC上６桁別 注意すべきDPCコーディングの事例集
    DPC上６桁 DPC名称 事例 対応
    010010 脳腫瘍
    頭頂葉神経膠腫の摘出術を行った場合。
    神経膠腫は発生部位を明確にした「頭頂葉神経膠腫（C713）」を選択する。
    040190
    胸水、胸膜の疾患
    左湿性胸膜炎で入院した場合。
    「左湿性胸膜炎（J90）」を選択する。
    """

    cases = parse_coding_cases_from_text(text, source_page=35)

    assert len(cases) == 2
    assert cases[0].dpc_code == "010010"
    assert cases[0].dpc_name == "脳腫瘍"
    assert cases[0].example_text == "頭頂葉神経膠腫の摘出術を行った場合。"
    assert "頭頂葉神経膠腫（C713）" in cases[0].guidance_text
    assert cases[1].dpc_name == "胸水、胸膜の疾患"
    assert cases[1].source_page == 35


def test_nameが改行されていても結合できる() -> None:
    text = """
    010020
    くも膜下出血、
    破裂脳動脈瘤
    中大脳動脈瘤破裂によるくも膜下出血で入院した場合。
    「中大脳動脈瘤破裂によるくも膜下出血（I601）」を選択する。
    """

    cases = parse_coding_cases_from_text(text, source_page=36)

    assert len(cases) == 1
    assert cases[0].dpc_name == "くも膜下出血、 破裂脳動脈瘤"
    assert "中大脳動脈瘤破裂によるくも膜下出血" in cases[0].example_text
    assert "I601" in cases[0].guidance_text


def test_page跨ぎの事例を連結して抽出できる() -> None:
    lines = [
        "<<PAGE:36>>",
        "010061 一過性脳虚血発 作",
        "脳梗塞が疑われ入院",
        "し、検査の結果、椎骨",
        "<<PAGE:37>>",
        "脳底動脈循環不全と判",
        "明した場合。",
        "入院契機病名は脳梗塞疑い（I63$）、 医療",
        "資源病名は椎骨脳底動脈循環不全（G450）",
        "を選択する。",
    ]

    cases = _parse_coding_cases_from_lines(lines, default_source_page=36)

    assert len(cases) == 1
    assert cases[0].source_page == 36
    assert "椎骨 脳底動脈循環不全と判 明した場合。" in cases[0].example_text
    assert "G450" in cases[0].guidance_text


def test_事例先頭がガイダンスでも分離できる() -> None:
    text = """
    010010 脳腫瘍
    医療資源病名は頭頂葉神経膠腫（C713）を選択する。
    """

    cases = parse_coding_cases_from_text(text, source_page=35)

    assert len(cases) == 1
    assert cases[0].example_text == ""
    assert cases[0].guidance_text == "医療資源病名は頭頂葉神経膠腫（C713）を選択する。"


def test_start_page指定時は見出し前提にせず抽出を始める(monkeypatch: pytest.MonkeyPatch) -> None:
    class FakePage:
        def __init__(self, text: str) -> None:
            self._text = text

        def extract_text(self) -> str:
            return self._text

    class FakeReader:
        def __init__(self, _: str) -> None:
            self.pages = [
                FakePage("前置き"),
                FakePage("010010 脳腫瘍\n頭頂葉神経膠腫の摘出術を行った場合。\n医療資源病名は頭頂葉神経膠腫（C713）を選択する。"),
            ]

    monkeypatch.setattr("marume_data.coding_text.PdfReader", FakeReader)

    from marume_data.coding_text import extract_coding_cases_from_pdf

    cases = extract_coding_cases_from_pdf(Path("dummy.pdf"), start_page=2)

    assert len(cases) == 1
    assert cases[0].dpc_code == "010010"
