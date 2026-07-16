# 406: Document AWS Bedrock llm_api setup; default to an inference-profile model

## Problem

Two gaps surfaced while helping a user configure the `anthropic-bedrock`
provider live against a real AWS account:

1. **The default Bedrock model can't be invoked.** The default was the bare
   foundation-model ID `anthropic.claude-sonnet-4-6-20250514-v1:0`. On current
   AWS accounts, Anthropic Claude models on Bedrock are **inference-profile
   only** — they do not support on-demand throughput by their bare model ID.
   Invoking the bare ID fails at query time. Confirmed live:
   `aws bedrock list-foundation-models --by-inference-type ON_DEMAND` returns
   **no** Anthropic models, while `aws bedrock list-inference-profiles` shows
   `us.anthropic.claude-sonnet-4-6` (and siblings) as `ACTIVE`. A test invoke
   of `us.anthropic.claude-sonnet-4-6` via `bedrock-runtime invoke-model`
   succeeded and returned a real completion.

2. **The docs don't mention Bedrock at all.** `docs/llm-providers.md` and
   `docs/configuration.md` predated the vertex/bedrock providers (PR #397,
   plan 394) and still listed only `ollama`/`anthropic`/`openai`/`ramalama`,
   omitting `anthropic-vertex`, `anthropic-bedrock`, and the `region` /
   `project_id` fields. A user following the docs had no way to learn the
   Bedrock setup, the credential model, or the inference-profile requirement.

## Approach

### Code

Change the Bedrock default model from the bare foundation-model ID to the US
cross-region inference profile ID `us.anthropic.claude-sonnet-4-6`, so the
minimal config (`provider: anthropic-bedrock` with no `model`) works out of
the box in US regions. Updated in both places that hold the default
(`pkg/ai/bedrock.go` const and `pkg/ai/factory.go` registry), with a comment
explaining why it's a profile ID. TDD: the two test assertions that pin the
default (`pkg/ai/bedrock_test.go`, `pkg/ai/factory_test.go`) were updated
first.

No behavior change beyond the default string — the provider already passes
`llm_api.model` straight through, so users on non-US regions or wanting a
different model set `model:` to their own profile ID.

### Docs

- `docs/llm-providers.md`: add full `anthropic-bedrock` and `anthropic-vertex`
  sections; add `region` / `project_id` to the fields table; update the
  provider list. The Bedrock section documents:
  - Credentials come from the **AWS SDK default credential chain** (env vars,
    `~/.aws`, SSO/roles/IMDS) — **nothing** in the SREPD config. Region comes
    from `AWS_REGION` / `~/.aws/config`, not `llm_api.region`.
  - The minimal config and the model-override config.
  - One-time model enablement in the AWS console
    (`.../bedrock/home#/model-catalog?providerName=anthropic`).
  - Inference-profile requirement, with the `list-inference-profiles` and
    `list-foundation-models --by-inference-type ON_DEMAND` commands to
    discover valid IDs.
- `docs/configuration.md`: add the two providers and the `region` /
  `project_id` keys to the LLM API table; add a Bedrock pointer note.
- `README.md`: add a short Bedrock callout under the config reference table
  linking to the detailed doc.

## Notes / lessons

- **Vertex reads `region`/`project_id` from config; Bedrock does not.** This
  asymmetry is real (`pkg/ai/vertex.go` vs `pkg/ai/bedrock.go`, which only
  calls `bedrock.WithLoadDefaultConfig`) and is now documented explicitly so
  users don't waste time setting `llm_api.region` for Bedrock expecting it to
  take effect.
- **Bedrock model IDs are not foundation-model IDs.** The inference-profile
  requirement is the single biggest Bedrock footgun; both the default and the
  docs now steer users to `us.`-prefixed profile IDs.
