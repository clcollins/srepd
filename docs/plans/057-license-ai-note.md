# 057: Add AI-Assisted Contributions Note to LICENSE

## Summary

Add a transparency notice to the LICENSE file acknowledging that portions of the
codebase were developed with AI assistance (Claude by Anthropic). The note is
appended below the existing MIT license text and does not modify the license
itself.

## Motivation

As AI-assisted development becomes standard practice, it is important to be
transparent about the use of AI tools in the development process. This note
clarifies the licensing intent for any AI-generated contributions and
acknowledges the evolving legal landscape around AI-generated code.

## Changes

- `LICENSE`: Append a clearly separated "Note on AI-Assisted Contributions"
  section below the existing MIT license text.

## Risks

- None. This is a documentation-only change that does not alter the license
  terms.

## Lessons Learned

**GENUINE ERROR — non-standard LICENSE text broke GitHub license detection**
(Fixed by: [077-fix-license-detection.md](077-fix-license-detection.md))

Appending a "Note on AI-Assisted Contributions" section to the LICENSE
file caused GitHub's license detection tool (licensee) to fail to
recognize the file as MIT. The repository's license badge changed to
"unknown".

Why it wasn't caught: the license badge was not checked after the
change, and the "Risks" section above incorrectly stated "None" without
considering tooling that parses LICENSE files.

Prevention: tooling-sensitive files (LICENSE, go.mod, Dockerfile) must
be validated against their respective automated checks after any
modification. Transparency notes belong in README or NOTICE files,
never in the LICENSE file itself.
