import { describe, expect, it } from "vitest";
import { initialOperationalProjection, projectOperational } from "../features/session/operationalProjection";
import type { OperationalAggregateKind, WireOperationalEnvelope } from "../lib/types";

function fact(aggregateKind: OperationalAggregateKind, sequence: number, kind: string): WireOperationalEnvelope {
  const id = `${aggregateKind}-1`;
  return {
    schemaVersion: 1, eventId: `${id}-${sequence}`, operationId: id,
    aggregate: { kind: aggregateKind, id, sequence }, scope: { tabId: "tab-1" }, kind,
  };
}

describe("operational lifecycle projection", () => {
  it.each([
    ["task", "task.completed"], ["model_call", "model_call.failed"], ["tool_call", "tool_call.cancelled"],
    ["session", "session.closed"], ["dashboard_write", "dashboard_write.committed"], ["plan", "plan.exited"],
  ] as const)("deduplicates and tombstones %s", (aggregateKind, terminalKind) => {
    let state = initialOperationalProjection("tab-1");
    state = projectOperational(state, fact(aggregateKind, 1, `${aggregateKind}.running`));
    state = projectOperational(state, fact(aggregateKind, 2, terminalKind));
    const terminal = state;
    state = projectOperational(state, { ...fact(aggregateKind, 3, `${aggregateKind}.running`), eventId: `${aggregateKind}-late` });
    expect(state).toBe(terminal);
    expect(state.aggregates[`${aggregateKind}:${aggregateKind}-1`]?.terminal).toBe(true);
  });

  it("rejects duplicate, out-of-order, and wrong-tab facts", () => {
    let state = projectOperational(initialOperationalProjection("tab-1"), fact("task", 2, "task.running"));
    const accepted = state;
    state = projectOperational(state, fact("task", 1, "task.ready"));
    state = projectOperational(state, fact("task", 2, "task.running"));
    state = projectOperational(state, { ...fact("task", 3, "task.completed"), scope: { tabId: "tab-2" } });
    expect(state).toBe(accepted);
  });
});
