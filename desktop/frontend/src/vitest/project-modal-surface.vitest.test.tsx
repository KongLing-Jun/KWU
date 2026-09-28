import { useRef, useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ProjectModalSurface } from "../features/projects/ui/ProjectModalSurface";

function Fixture({ pending = false, suspended = false, onClosed = vi.fn() }: { pending?: boolean; suspended?: boolean; onClosed?: () => void }) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const firstRef = useRef<HTMLButtonElement>(null);
  const close = () => { onClosed(); setOpen(false); };
  return <><div data-testid="background"><button ref={triggerRef} type="button" onClick={() => setOpen(true)}>打开项目弹窗</button><label>后台搜索<input aria-label="后台搜索" /></label></div>{open ? <ProjectModalSurface backdropClassName="backdrop" className="surface" titleId="fixture-title" descriptionId="fixture-description" pending={pending} suspended={suspended} returnFocus={triggerRef.current} initialFocusRef={firstRef} onClose={close}><h2 id="fixture-title">项目弹窗</h2><p id="fixture-description">共享键盘语义</p><button ref={firstRef} type="button">第一个</button><button type="button" style={{ display: "none" }}>旧关闭按钮</button><button type="button" onClick={close}>最后一个</button></ProjectModalSurface> : null}</>;
}

describe("ProjectModalSurface", () => {
  it("links accessible text, traps Tab, closes with Escape, and restores trigger focus", async () => {
    const onClosed = vi.fn();
    render(<Fixture onClosed={onClosed} />);
    const trigger = screen.getByRole("button", { name: "打开项目弹窗" });
    await userEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "项目弹窗", description: "共享键盘语义" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(screen.getByTestId("background")).toHaveAttribute("inert");
    expect(screen.getByRole("button", { name: "第一个" })).toHaveFocus();
    screen.getByRole("button", { name: "放大弹窗" }).focus();
    fireEvent.keyDown(dialog, { key: "Tab", shiftKey: true });
    expect(screen.getByRole("button", { name: "最后一个" })).toHaveFocus();
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(onClosed).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("background")).not.toHaveAttribute("inert");
    expect(trigger).toHaveFocus();
  });

  it("cannot close through Escape or backdrop while a mutation is pending", async () => {
    const onClosed = vi.fn();
    const { container } = render(<Fixture pending onClosed={onClosed} />);
    await userEvent.click(screen.getByRole("button", { name: "打开项目弹窗" }));
    const dialog = screen.getByRole("dialog", { name: "项目弹窗" });
    expect(dialog).toHaveAttribute("aria-busy", "true");
    fireEvent.keyDown(dialog, { key: "Escape" });
    fireEvent.mouseDown(container.querySelector(".backdrop")!);
    expect(onClosed).not.toHaveBeenCalled();
    expect(dialog).toBeVisible();
  });

  it("removes a suspended parent modal from interaction and accessibility exposure", async () => {
    const onClosed = vi.fn();
    const { container } = render(<Fixture suspended onClosed={onClosed} />);
    await userEvent.click(screen.getByRole("button", { name: "打开项目弹窗" }));
    const dialog = container.querySelector<HTMLElement>(".surface")!;
    expect(dialog).toHaveAttribute("aria-hidden", "true");
    expect(dialog).toHaveAttribute("inert");
    expect(dialog).not.toHaveAttribute("aria-modal");
    fireEvent.keyDown(dialog, { key: "Escape" });
    fireEvent.mouseDown(container.querySelector(".backdrop")!);
    expect(onClosed).not.toHaveBeenCalled();
  });

  it("maximizes and restores without changing dialog semantics", async () => {
    render(<Fixture />);
    await userEvent.click(screen.getByRole("button", { name: "打开项目弹窗" }));
    const dialog = screen.getByRole("dialog", { name: "项目弹窗" });
    const maximize = screen.getByRole("button", { name: "放大弹窗" });
    const close = screen.getByRole("button", { name: "关闭" });
    expect(maximize.parentElement).toBe(close.parentElement);
    expect(maximize.parentElement).toHaveClass("project-modal-surface__controls");
    expect(maximize).toHaveAttribute("aria-pressed", "false");

    await userEvent.click(maximize);
    expect(dialog).toHaveClass("project-modal-surface--maximized");
    const restore = screen.getByRole("button", { name: "还原弹窗" });
    expect(restore).toHaveAttribute("aria-pressed", "true");

    await userEvent.click(restore);
    expect(dialog).not.toHaveClass("project-modal-surface--maximized");
  });
});
