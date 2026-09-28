import { stripHiddenSystemPrompt } from "../../lib/expertPrompt";
import type { State } from "./sessionState";

// A missing terminal event must not let the next turn inherit a live row.
export function sealCurrentAssistant(state: State): State {
  const items = state.items.map((item) => {
    if (item.kind === "assistant" && item.id === state.live?.id) {
      return { ...item, text: stripHiddenSystemPrompt(state.live.text), reasoning: state.live.reasoning, streaming: false };
    }
    if (item.kind === "assistant" && item.id === state.currentAssistant && item.streaming) return { ...item, streaming: false };
    if (item.kind === "tool" && item.status === "running") return { ...item, status: "stopped" as const };
    return item;
  });
  return {
    ...state,
    items: items.every((item, index) => item === state.items[index]) ? state.items : items,
    currentAssistant: undefined,
    live: undefined,
    rootFinalCandidateId: undefined,
    turnActive: false,
  };
}
