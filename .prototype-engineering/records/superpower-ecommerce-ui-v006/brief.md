# Prototype Brief

## Hypothesis

Using one constrained widget-envelope concept for both inline chat results and the right research canvas will make MCP-driven UI assembly coherent across surfaces without allowing model-authored executable code.

## Primary user flow

Open a completed research conversation and review the AI-composed inline conclusion widget alongside the AI-composed research-canvas widgets. The conversation timestamp remains visible without a redundant research-mode label.

## Success criteria

- The conversation date row no longer displays “综合研究”.
- The completed research conclusion is rendered as an explicitly identified inline generated widget.
- Inline and canvas widgets expose the same conceptual widget-envelope metadata: identity, type, and target surface.
- Existing visual presentation, evidence actions, scenarios, and responsive behavior remain intact.

## Fidelity and data mode

High-fidelity local UI with deterministic widget envelopes representing future MCP tool output.

## Out of scope

Real MCP transport, runtime schema validation, remote data, model execution, persistence, and arbitrary code generation remain out of scope.
