---
name: dpc-coding
description: Use this skill when working on DPC coding logic, DPC sample case candidates, marume DPC test data, or MHLW DPC/PDPS coding text examples. Helps developers with limited DPC domain knowledge avoid treating generated coding-case JSON as ground truth and check diagnosis, DPC 6-digit grouping, procedures, comorbidities, and source-page evidence correctly.
---

# DPC Coding

Use this skill when DPC coding examples, sample cases, or test data are part of the task. Treat it as a domain guardrail for developers who do not know DPC well.

## Scope

- Use for `marume` DPC sample case candidates and tests derived from MHLW coding text examples.
- Use for reviewing generated cases before moving them into `testdata/` or evaluator fixtures.
- Do not use this as a substitute for official MHLW materials or clinical coding review.

## Repo Workflow

First read `docs/dpc-coding-sample-cases.md` for the current extraction workflow and known limitations.

If data needs to be generated, use the repository tasks instead of reimplementing extraction logic:

```sh
mise run coding-cases
```

This produces `.local/dpc-coding-cases-<version>.json` and `.local/dpc-sample-case-candidates-<version>.json`. Files under `.local/` are working data and should not be committed unless the repo explicitly changes that policy.

## Domain Guardrails

- Generated JSON is a sample-case candidate, not answer data.
- `dpc_code_6` is the 6-digit disease grouping entrance, not necessarily the final full DPC code.
- `main_diagnosis` may be a heuristic extraction from text; verify whether it is actually the medical-resource diagnosis before using it as a fixture expectation.
- Read `example_text` and `guidance_text` together. Do not mix up admission-trigger diagnosis, medical-resource diagnosis, comorbidity, complication, and post-admission disease.
- Treat `procedures` as simple code extraction unless the surrounding text supports the procedure condition. Do not assume it is a complete surgery/procedure basis.
- Leave `age` and `sex` unknown when the source example does not support them; do not invent values just to complete a test case.
- Use `source_page` to return to the original PDF when a case is ambiguous.

## Review Checklist

Before promoting a generated candidate into a stable fixture:

1. Confirm the source is the intended fiscal year/version.
2. Confirm the DPC 6-digit grouping matches the example and DPC name.
3. Identify the medical-resource diagnosis from the example/guidance text, not only from the first ICD-like code.
4. Check whether procedure codes are relevant to the DPC condition being tested.
5. Check whether comorbidities or post-admission diseases are being used as the main diagnosis by mistake.
6. Keep uncertainty in notes rather than forcing a precise field value.

## Output Expectations

When reporting DPC sample-case work, state:

- which source/version was used
- whether data was generated or only reviewed
- which fields remain heuristic or uncertain
- whether any candidate was promoted to stable test data
