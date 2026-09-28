# Prototype Brief

## Hypothesis

Citation can be a uniform registered-widget capability, independent of widget type, surface and business props, while retaining a stable snapshot fallback when source evidence is absent.

## Primary user flow

The user clicks “引用” on any generated component in the conversation or research canvas. The composer receives the component title plus its first evidence reference, or a stable widget revision snapshot when no evidence exists, and the existing Host action records the active citation.

## Success criteria

- Every non-loading registered widget rendered with a citation handler exposes “引用”.
- No widget props such as `citable` control the capability.
- Inline and canvas widgets use the same callback contract.
- Widgets without evidence references still produce an intelligible revision snapshot reference.
- Existing follow-up flow continues to work.

## Fidelity and data mode

High-fidelity UI interaction over the existing deterministic/persisted DashboardDocument.

## Out of scope

Multi-select citation, citation chips containing thumbnails, evidence preview drawers, quote-range selection and backend retrieval of cited source content.
