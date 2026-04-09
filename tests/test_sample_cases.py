from __future__ import annotations

from marume_data.sample_cases import build_sample_case_candidates, split_dpc_name_and_example


def test_dpc_nameから食い込んだ事例文を分離できる() -> None:
    name, leaked = split_dpc_name_and_example("未破裂脳動脈瘤 硬膜動静脈瘻のため、")

    assert name == "未破裂脳動脈瘤"
    assert leaked == "硬膜動静脈瘻のため、"


def test_extracted_casesからsample_case候補を組み立てられる() -> None:
    extracted = [
        {
            "dpc_code": "010030",
            "dpc_name": "未破裂脳動脈瘤 硬膜動静脈瘻のため、",
            "example_text": "血管内手術を施行した場合。",
            "guidance_text": "入院契機病名、医療資源病名ともに、 硬膜動静脈瘻（I671）を選択する。",
            "raw_text": "",
            "source_page": 36,
        },
        {
            "dpc_code": "010020",
            "dpc_name": "くも膜下出血、 破裂脳動脈瘤 中大脳動脈瘤破裂に対し、脳動脈瘤頚部クリ",
            "example_text": "ッピング手術を施行した場合。",
            "guidance_text": "医療資源病名は 中大脳動脈瘤破裂によるくも膜下出血（I601）を選択する。",
            "raw_text": "",
            "source_page": 36,
        },
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 2
    assert cases[0].case_id == "dpc-010030-0001"
    assert cases[0].dpc_name == "未破裂脳動脈瘤"
    assert cases[0].main_diagnosis == "I671"
    assert "硬膜動静脈瘻のため、 血管内手術を施行した場合。" == cases[0].example_text
    assert cases[0].procedures == []
    assert "処置コード未抽出" in cases[0].notes
    assert cases[1].main_diagnosis == "I601"
