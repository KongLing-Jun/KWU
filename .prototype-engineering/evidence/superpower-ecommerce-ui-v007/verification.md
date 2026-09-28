# v007 Verification

Date: 2026-08-21

## Frontend production build

Command:

```text
cd desktop/frontend && pnpm build
```

Result: passed.

- CSS syntax checks passed.
- z-index token check passed.
- TypeScript `tsc --noEmit` passed.
- Vite production build completed successfully.
- Existing large-chunk warnings remain and are not introduced as a functional failure by this prototype.

## Browser interaction verification

Target: `http://127.0.0.1:5174/`

Verified through the running KWU Desktop development page:

1. Sidebar “超能力” opens the catalog and “跨境电商超能力” opens the research workspace.
2. The complete scenario renders one `conversation-inline` summary and four `research-canvas` widgets from DashboardDocument revision 10.
3. Default copy uses 1688 supply discovery plus eBay US active-listing semantics and exposes Shopify as an authorized merchant-operation source.
4. The source coverage widget lists exact market monthly sales, eBay review text and all-market natural keyword trends as unavailable.
5. Clicking the eBay snapshot citation attached the session evidence reference and increased the persisted document from revision 10 to revision 11.
6. The partial scenario displayed both “部分证据不可获得” and “需求趋势不可用”, with document revision 12.
7. The empty scenario displayed the centered research capability chooser and blank canvas, with no Amazon content present.
8. After a full page reload and reopening the capability, the complete document restored at revision 11 from the browser-local serialized adapter.
9. No new browser console errors appeared after the final full reload.

## Intentionally skipped

Focused Vitest tests were updated for the new semantics but not executed, following the user's rapid-iteration instruction to avoid expanding the test loop. Real MCP, external APIs, OAuth and Go sidecar persistence remain outside the frozen v007 boundary.
