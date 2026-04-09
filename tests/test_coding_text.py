from __future__ import annotations

from marume_data.coding_text import parse_coding_cases_from_text


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
