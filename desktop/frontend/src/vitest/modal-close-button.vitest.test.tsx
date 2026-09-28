import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ModalCloseButton } from "../components/ModalCloseButton";

afterEach(() => {
  vi.useRealTimers();
});

describe("ModalCloseButton tooltip", () => {
  it("keeps the accessible close action without showing a disabled tooltip", () => {
    vi.useFakeTimers();
    const onClick = vi.fn();
    const { container } = render(
      <ModalCloseButton label="关闭" showTooltip={false} onClick={onClick} />,
    );
    const button = screen.getByRole("button", { name: "关闭" });
    const trigger = container.querySelector(".tooltip-trigger");
    if (!trigger) throw new Error("Missing tooltip trigger");

    fireEvent.mouseEnter(trigger);
    act(() => vi.advanceTimersByTime(181));
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.focus(button);
    act(() => vi.runOnlyPendingTimers());
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();

    fireEvent.click(button);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("shows the tooltip on hover by default", () => {
    vi.useFakeTimers();
    const { container } = render(<ModalCloseButton label="关闭" />);
    const trigger = container.querySelector(".tooltip-trigger");
    if (!trigger) throw new Error("Missing tooltip trigger");

    fireEvent.mouseEnter(trigger);
    act(() => vi.advanceTimersByTime(180));

    expect(screen.getByRole("tooltip")).toHaveTextContent("关闭");
  });
});
