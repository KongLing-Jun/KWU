# Prototype Brief

## Hypothesis

The remaining product loop can be completed non-invasively by making task evidence durable, delegating Desktop inference to the existing session controller, and preserving navigation from every answer citation back to the generated source card.

## Primary user flow

The Desktop opens the capability task and initializes validated evidence JSON. The user selects several cards and sends a question. The workspace builds bounded hidden evidence context and submits it through the current active session while continuing to render the live assistant text in place. Clicking a numbered source expands the canvas when needed, scrolls to the card and briefly highlights it. Reloading reuses the same evidence and dashboard sidecars.

## Success criteria

- EvidenceDocument is atomically persisted under the bound capability task and rejected on scope/schema/size violations.
- Desktop execution uses the existing controller/session; browser preview is visibly identified as deterministic fallback.
- Visible user text remains clean while hidden submit text carries bounded untrusted evidence context.
- Live/final assistant text can be projected into the superpower follow-up without changing session message schemas.
- Every resolved answer source navigates to its widget, including widgets inside a collapsed canvas.
- Dashboard and evidence state survive Desktop reload under the same task binding.

## Fidelity and data mode

Runnable Desktop integration with real configured-model execution when the active session is ready. Deterministic evidence seeds remain clearly classified as Mock or official-document research; no external marketplace claim is promoted beyond its recorded provenance.

## Out of scope

Obtaining Shopify/eBay/1688 production credentials and approvals, live marketplace collection, purchasing/listing/order writes, OAuth UX, cloud sync, multi-user ACLs and production marketplace operations.
