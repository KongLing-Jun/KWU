# Prototype Brief

## Hypothesis

Generated components feel more direct and less cluttered when citation is a card-level hover/right-click interaction with animated selection feedback rather than an explicit button.

## Primary user flow

The user hovers any generated card and sees a subtle fading outline plus a “右键引用” affordance. Right-clicking prevents the browser context menu, selects the card, shows a persistent selected treatment, and places its reference in the composer. Removing the reference clears the selected treatment; right-clicking another card moves it.

## Success criteria

- No visible “引用” button remains inside generated components.
- Every non-loading generated card responds to right-click.
- Hover and selected states animate with reduced-motion support.
- Only one card is selected at a time and composer removal clears it.
- Native context menu is prevented only for the handled card interaction.

## Fidelity and data mode

High-fidelity pointer interaction over existing deterministic/persisted widgets.

## Out of scope

Touch long-press, multi-select, drag selection, card preview popovers and system-level context-menu extension.
