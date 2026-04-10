from __future__ import annotations

import json

import pytest

from marume_data.sample_cases import (
    build_case_input_candidate_report,
    build_case_input_candidates,
    build_sample_case_candidates,
    split_dpc_name_and_example,
    write_case_input_candidates_jsonl,
)


def _make_extracted_case(**overrides: object) -> dict[str, object]:
    """Factory to create a base extracted case dict with optional overrides."""
    base = {
        "dpc_code": "010030",
        "dpc_name": "未破裂脳動脈瘤",
        "example_text": "血管内手術を施行した場合。",
        "guidance_text": "医療資源病名は未破裂脳動脈瘤（I671）を選択する。",
        "raw_text": "",
        "source_page": 36,
    }
    base.update(overrides)
    return base


def test_dpc_nameから食い込んだ事例文を分離できる() -> None:
    name, leaked = split_dpc_name_and_example("未破裂脳動脈瘤 硬膜動静脈瘻のため、")

    assert name == "未破裂脳動脈瘤"
    assert leaked == "硬膜動静脈瘻のため、"


def test_dpc_nameは最も手前の文脈トークンで分離する() -> None:
    name, leaked = split_dpc_name_and_example("脳腫瘍 脳梗塞のため入院した場合")

    assert name == "脳腫瘍"
    assert leaked == "脳梗塞のため入院した場合"


def test_extracted_casesからsample_case候補を組み立てられる() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
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
    assert cases[1].case_id == "dpc-010020-0002"
    assert cases[0].dpc_name == "未破裂脳動脈瘤"
    assert cases[0].main_diagnosis == "I671"
    assert cases[0].example_text == "硬膜動静脈瘻のため、 血管内手術を施行した場合。"
    assert cases[0].procedures == []
    assert "処置コード未抽出" in cases[0].notes
    assert cases[1].main_diagnosis == "I601"


def test_case_idはdpc_codeごとにリセットせず通し番号になる() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(),
        _make_extracted_case(
            example_text="脳動脈瘤頚部クリッピング術を施行した場合。",
            source_page=37,
        ),
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 2
    assert cases[0].case_id == "dpc-010030-0001"
    assert cases[1].case_id == "dpc-010030-0002"


def test_main_diagnosisは不適切コードではなく選択されるコードを優先する() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(
            dpc_code="010010",
            dpc_name="脳腫瘍",
            example_text="神経膠腫（C719）は、部位が不明確であり不適切である。部位を明確にし、頭頂葉神経膠腫（C713）のように表す。",
            guidance_text="",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].main_diagnosis == "C713"
    assert cases[0].comorbidities == ["C719"]


def test_main_diagnosisは本分類が該当するコードを優先する() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(
            dpc_code="010020",
            dpc_name="くも膜下出血、破裂脳動脈瘤",
            example_text="くも膜下出血について。",
            guidance_text="本分類は、非外傷性のくも膜下出血（I60$）が該当する。外傷による場合は、外傷性くも膜下出血（S066$）を選択し、他分類となる。",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].main_diagnosis == "I60$"
    assert cases[0].comorbidities == ["S066$"]


def test_処置コードは4桁コードも保持する() -> None:
    extracted = [
        _make_extracted_case(
            example_text="脳動脈瘤頚部クリッピング術 K177 を施行した場合。",
            guidance_text="",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].procedures == ["K177"]


def test_kコード以外は処置コードとして拾わない() -> None:
    extracted = [
        _make_extracted_case(
            example_text="J045 を施行した場合。",
            guidance_text="",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].procedures == []


def test_ガイダンス中のicdコードは処置コードとして拾わない() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(
            example_text="脳動脈瘤頚部クリッピング術を施行した場合。",
            guidance_text="医療資源病名は硬膜動静脈瘻（I671）を選択する。",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].procedures == []


def test_例文中のicdコードは処置コードとして拾わない() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(
            dpc_code="010010",
            dpc_name="脳腫瘍",
            example_text="神経膠腫（C719）は不適切であり、頭頂葉神経膠腫（C713）のように表す。",
            guidance_text="",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].procedures == []


def test_main_diagnosisは正式文言の医療資源病名も優先する() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(
            guidance_text="入院契機病名は未破裂脳動脈瘤（I671）、医療資源を最も投入した傷病名は硬膜動静脈瘻（I672）を選択する。",
        )
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].main_diagnosis == "I672"


def test_必須文字列が欠けていると失敗する() -> None:
    extracted = [
        _make_extracted_case(guidance_text=None)
    ]

    with pytest.raises(TypeError, match="guidance_text must be a string"):
        build_sample_case_candidates(extracted, fiscal_year=2026)


def test_source_pageは文字列の数字を許容する() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(source_page="36")
    ]

    cases = build_sample_case_candidates(extracted, fiscal_year=2026)

    assert len(cases) == 1
    assert cases[0].source_page == 36


def test_source_pageは不正値なら行番号付きで失敗する() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    extracted = [
        _make_extracted_case(source_page=True)
    ]

    with pytest.raises(TypeError, match="source_page must be an integer at row 1"):
        build_sample_case_candidates(extracted, fiscal_year=2026)


def test_fiscal_yearは正の整数だけを許容する() -> None:
    with pytest.raises(ValueError, match="fiscal_year must be positive"):
        build_sample_case_candidates([_make_extracted_case()], fiscal_year=0)


def test_marume_case_input候補はmain_diagnosisがあるものだけをjsonl化する(tmp_path) -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    candidates = build_sample_case_candidates(
        [
            _make_extracted_case(
                example_text="脳動脈瘤頚部クリッピング術 K177 を施行した場合。",
            ),
            _make_extracted_case(
                dpc_code="010040",
                dpc_name="なし",
                example_text="コードを含まない事例。",
                guidance_text="",
            ),
        ],
        fiscal_year=2026,
    )

    case_inputs = build_case_input_candidates(candidates)
    output_path = tmp_path / "cases.jsonl"
    write_case_input_candidates_jsonl(output_path, case_inputs)

    assert len(case_inputs) == 1
    assert case_inputs[0].case_id == "dpc-010030-0001"
    assert case_inputs[0].main_diagnosis == "I671"
    lines = output_path.read_text(encoding="utf-8").splitlines()
    assert len(lines) == 1
    assert json.loads(lines[0]) == {
        "case_id": "dpc-010030-0001",
        "fiscal_year": 2026,
        "sex": "",
        "main_diagnosis": "I671",
        "diagnoses": ["I671"],
        "procedures": ["K177"],
        "comorbidities": [],
    }


def test_case_input候補レポートはskip理由とレビューnote件数を返す() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    candidates = build_sample_case_candidates(
        [
            _make_extracted_case(),
            _make_extracted_case(
                dpc_code="010040",
                dpc_name="なし",
                example_text="コードを含まない事例。",
                guidance_text="",
            ),
        ],
        fiscal_year=2026,
    )

    report = build_case_input_candidate_report(candidates)

    assert report["total_candidates"] == 2
    assert report["case_input_candidates"] == 1
    assert report["skipped_candidates"] == 1
    assert report["skipped_reasons"] == {"main_diagnosis 未抽出": 1}
    assert report["review_note_counts"] == {
        "処置コード未抽出": 2,
        "ICD コード未抽出": 1,
    }


def test_case_input候補はmain_diagnosisが空白だけならskipする() -> None:
    # Test data intentionally uses fullwidth parentheses to match source document format
    # ruff: noqa: RUF001
    candidates = build_sample_case_candidates([_make_extracted_case()], fiscal_year=2026)
    candidates[0].main_diagnosis = "   "

    case_inputs = build_case_input_candidates(candidates)
    report = build_case_input_candidate_report(candidates)

    assert case_inputs == []
    assert report["case_input_candidates"] == 0
    assert report["skipped_candidates"] == 1
    assert report["skipped_reasons"] == {"main_diagnosis 未抽出": 1}
