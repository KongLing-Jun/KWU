import { act, render, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StatusBar } from "../components/StatusBar";
import { LocaleProvider } from "../lib/i18n";
import {
  resolveSubscriptionModelGateLabel,
  useDelayedSubscriptionModelPending,
} from "../lib/subscriptionModelGatePresentation";

afterEach(() => {
  vi.useRealTimers();
});

describe("subscription model gate presentation", () => {
  it("keeps the target session label while facts load and uses a neutral fallback", () => {
    expect(resolveSubscriptionModelGateLabel({
      state: "loading",
      routeDisplayName: "Previous resolved route",
      sessionMetaLabel: "Target session model",
      tabLabel: "Target tab model",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Target session model");

    expect(resolveSubscriptionModelGateLabel({
      state: "loading",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Select model");

    expect(resolveSubscriptionModelGateLabel({
      state: "allowed",
      routeDisplayName: "Resolved route",
      sessionMetaLabel: "Session model",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Resolved route");

    expect(resolveSubscriptionModelGateLabel({
      state: "blocked",
      sessionMetaLabel: "Session model",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Current model unavailable");
  });

  it("ignores stale route and tab labels during a surface transition", () => {
    expect(resolveSubscriptionModelGateLabel({
      state: "allowed",
      transitioning: true,
      routeDisplayName: "Previous resolved route",
      routeModel: "previous-route-model",
      sessionMetaLabel: "Target session model",
      tabLabel: "Previous tab model",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Target session model");

    expect(resolveSubscriptionModelGateLabel({
      state: "loading",
      transitioning: true,
      routeDisplayName: "Previous resolved route",
      routeModel: "previous-route-model",
      tabLabel: "Previous tab model",
      fallbackLabel: "Select model",
      blockedLabel: "Current model unavailable",
    })).toBe("Select model");
  });

  it("delays pending per facts key and rejects stale timers", () => {
    vi.useFakeTimers();
    const { result, rerender, unmount } = renderHook(
      ({ factsKey, loading }) => useDelayedSubscriptionModelPending(factsKey, loading),
      { initialProps: { factsKey: "tab-a/model-a", loading: true } },
    );

    expect(result.current).toBe(false);
    act(() => vi.advanceTimersByTime(1_999));
    expect(result.current).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe(true);

    rerender({ factsKey: "tab-b/model-b", loading: true });
    expect(result.current).toBe(false);
    act(() => vi.advanceTimersByTime(1_999));
    expect(result.current).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe(true);

    rerender({ factsKey: "tab-b/model-b", loading: false });
    expect(result.current).toBe(false);
    unmount();
    act(() => vi.runOnlyPendingTimers());
  });

  it("changes only the fixed-size status dot after the quiet period", () => {
    const { container, rerender } = render(
      <LocaleProvider>
        <StatusBar
          context={{ used: 0, window: 0, sessionTokens: 0 }}
          running
          collaborationMode="normal"
          toolApprovalMode="ask"
          items={["model"]}
          modelLabel="Target session model"
        />
      </LocaleProvider>,
    );

    expect(container.querySelector(".statusbar__model")).toHaveTextContent("Target session model");
    expect(container.querySelector("[data-model-pending]")).not.toBeInTheDocument();

    rerender(
      <LocaleProvider>
        <StatusBar
          context={{ used: 0, window: 0, sessionTokens: 0 }}
          running
          collaborationMode="normal"
          toolApprovalMode="ask"
          items={["model"]}
          modelLabel="Target session model"
          modelPending
        />
      </LocaleProvider>,
    );

    expect(container.querySelector(".statusbar__model")).toHaveTextContent("Target session model");
    expect(container.querySelector("[data-model-pending]")).toHaveClass("statusbar__dot--pending");
    expect(container.querySelector("[data-model-pending]")).not.toHaveClass("statusbar__dot--busy");
  });
});
