import type { WireEvent } from "./types";

export function shouldRefreshWorkspaceForEvent(event: Pick<WireEvent, "kind" | "tool">): boolean {
  return event.kind === "turn_done"
    || event.kind === "notice"
    || (event.kind === "tool_result" && event.tool?.readOnly !== true);
}
