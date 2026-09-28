import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TurnActions, type TurnActionMenu } from "../components/Message";
import { LocaleProvider } from "../lib/i18n";
import type { TurnMetadata } from "../lib/types";

afterEach(() => {
  vi.useRealTimers();
});

function ControlledTurnActions({
  hoverMenus,
  onOpenMenu,
  turnMeta,
}: {
  hoverMenus: boolean;
  onOpenMenu?: (menu: TurnActionMenu | null) => void;
  turnMeta?: TurnMetadata;
}) {
  const [openMenu, setOpenMenu] = useState<TurnActionMenu | null>(null);
  return (
    <LocaleProvider>
      <TurnActions
        text="answer"
        turn={1}
        openMenu={openMenu}
        onOpenMenu={(menu) => {
          onOpenMenu?.(menu);
          setOpenMenu(menu);
        }}
        onRewind={vi.fn()}
        checkpoint={{
          turn: 1,
          prompt: "prompt",
          files: [],
          time: 1,
          canCode: true,
          canConversation: true,
        }}
        hoverMenus={hoverMenus}
        turnMeta={turnMeta}
      />
    </LocaleProvider>
  );
}

function menuGroup(buttonName: string): HTMLElement {
  const group = screen.getByRole("button", { name: buttonName }).closest(".turn-actions__group");
  if (!group) throw new Error(`Missing action group for ${buttonName}`);
  return group as HTMLElement;
}

describe("TurnActions hover menu lifecycle", () => {
  it("renders exact subscription credits and the model without hiding message actions", () => {
    render(<ControlledTurnActions hoverMenus={false} turnMeta={{
      taskId: "task-1",
      modelRef: "provider/glm-5.2",
      modelId: "glm-5.2",
      billingSource: "subscription_pool",
      chargedCredits: "7.9000",
    }} />);

    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("共消耗✧7.9");
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("glm-5.2");
    expect(screen.getByRole("button", { name: "复制" })).toBeInTheDocument();
  });

  it("shows a settling state until subscription credits arrive", () => {
    const pendingTurn = {
      taskId: "task-pending",
      modelRef: "provider/glm-5.2",
      modelId: "glm-5.2",
      billingSource: "subscription_pool",
    };
    const { rerender } = render(<ControlledTurnActions hoverMenus={false} turnMeta={pendingTurn} />);

    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("共消耗✧结算中…");
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("glm-5.2");

    rerender(<ControlledTurnActions hoverMenus={false} turnMeta={{ ...pendingTurn, chargedCredits: "7.9000" }} />);
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("共消耗✧7.9");
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).not.toHaveTextContent("结算中");
  });

  it("keeps model-only BYOK metadata and displays a legal zero settlement", () => {
    const { rerender } = render(<ControlledTurnActions hoverMenus={false} turnMeta={{
      taskId: "task-byok",
      modelRef: "provider/qwen-max",
      billingSource: "newapi_byok",
    }} />);
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("qwen-max");
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).not.toHaveTextContent("共消耗");
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).not.toHaveTextContent("结算中");

    rerender(<ControlledTurnActions hoverMenus={false} turnMeta={{
      taskId: "task-zero",
      modelRef: "provider/glm-5.2",
      billingSource: "subscription_pool",
      chargedCredits: "0",
    }} />);
    expect(screen.getByRole("group", { name: "本轮模型与积分" })).toHaveTextContent("共消耗✧0");
  });

  it("omits the conversation fork control while preserving the other message actions", () => {
    render(<ControlledTurnActions hoverMenus={false} />);

    expect(screen.getByRole("button", { name: "复制" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "总结" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "回溯" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "分叉会话" })).not.toBeInTheDocument();
  });

  it("opens on hover, keeps the menu while crossing the gap, and closes after the grace period", () => {
    vi.useFakeTimers();
    render(<ControlledTurnActions hoverMenus />);

    const summaryButton = screen.getByRole("button", { name: "总结" });
    fireEvent.mouseEnter(menuGroup("总结"));
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.mouseLeave(menuGroup("总结"));
    act(() => vi.advanceTimersByTime(100));
    fireEvent.mouseEnter(screen.getByRole("menu"));
    act(() => vi.advanceTimersByTime(50));
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");

    fireEvent.mouseLeave(menuGroup("总结"));
    act(() => vi.advanceTimersByTime(149));
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");
    act(() => vi.advanceTimersByTime(1));
    expect(summaryButton).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("cancels closing when the pointer re-enters during the grace period", () => {
    vi.useFakeTimers();
    render(<ControlledTurnActions hoverMenus />);

    fireEvent.mouseEnter(menuGroup("总结"));
    fireEvent.mouseLeave(menuGroup("总结"));
    act(() => vi.advanceTimersByTime(100));
    fireEvent.mouseEnter(menuGroup("总结"));
    act(() => vi.advanceTimersByTime(100));

    expect(screen.getByRole("button", { name: "总结" })).toHaveAttribute("aria-expanded", "true");
  });

  it("switches menus without allowing the old close timer to dismiss the new menu", () => {
    vi.useFakeTimers();
    render(<ControlledTurnActions hoverMenus />);

    fireEvent.mouseEnter(menuGroup("总结"));
    fireEvent.mouseLeave(menuGroup("总结"));
    act(() => vi.advanceTimersByTime(100));
    fireEvent.mouseEnter(menuGroup("回溯"));
    act(() => vi.advanceTimersByTime(50));

    expect(screen.getByRole("button", { name: "总结" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("button", { name: "回溯" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("menu")).toHaveLength(1);
  });

  it("does not pin or toggle an open menu when clicked in hover mode", () => {
    vi.useFakeTimers();
    render(<ControlledTurnActions hoverMenus />);

    const summaryGroup = menuGroup("总结");
    const summaryButton = screen.getByRole("button", { name: "总结" });
    fireEvent.mouseEnter(summaryGroup);
    fireEvent.click(summaryButton);
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");

    fireEvent.mouseLeave(summaryGroup);
    act(() => vi.advanceTimersByTime(150));
    expect(summaryButton).toHaveAttribute("aria-expanded", "false");
  });

  it("preserves click toggling in non-hover mode", () => {
    render(<ControlledTurnActions hoverMenus={false} />);

    const summaryButton = screen.getByRole("button", { name: "总结" });
    fireEvent.click(summaryButton);
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");
    fireEvent.mouseLeave(menuGroup("总结"));
    expect(summaryButton).toHaveAttribute("aria-expanded", "true");
    fireEvent.click(summaryButton);
    expect(summaryButton).toHaveAttribute("aria-expanded", "false");
  });

  it("clears a pending close timer when unmounted", () => {
    vi.useFakeTimers();
    const onOpenMenu = vi.fn();
    const { unmount } = render(<ControlledTurnActions hoverMenus onOpenMenu={onOpenMenu} />);

    fireEvent.mouseEnter(menuGroup("总结"));
    fireEvent.mouseLeave(menuGroup("总结"));
    expect(onOpenMenu).toHaveBeenCalledTimes(1);
    unmount();
    act(() => vi.runOnlyPendingTimers());

    expect(onOpenMenu).toHaveBeenCalledTimes(1);
  });
});
