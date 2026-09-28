import { fireEvent, render } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";

vi.mock("gsap", () => {
  const gsap = { to: () => ({ kill() {} }) };
  return { default: gsap, gsap };
});

class TestPointerEvent extends MouseEvent {
  readonly pointerId: number;
  readonly pointerType: string;

  constructor(type: string, init: PointerEventInit = {}) {
    super(type, init);
    this.pointerId = init.pointerId ?? 0;
    this.pointerType = init.pointerType ?? "";
  }
}

beforeAll(() => {
  Object.defineProperty(window, "PointerEvent", { configurable: true, value: TestPointerEvent });
  Object.defineProperty(globalThis, "PointerEvent", { configurable: true, value: TestPointerEvent });
});

function renderProjectTranscript() {
  return render(
    <LocaleProvider>
      <Transcript
        items={[
          { kind: "user", id: "user-1", text: "Project prompt" },
          { kind: "assistant", id: "assistant-1", text: "Project response", reasoning: "", streaming: false },
        ]}
        onPrompt={() => {}}
        questionNavigator={false}
        dragScroll
      />
    </LocaleProvider>,
  );
}

describe("project conversation drag scrolling", () => {
  it("starts at the 6px threshold, scrolls opposite the pointer, and cleans up capture", () => {
    const view = renderProjectTranscript();
    const transcript = view.container.querySelector<HTMLDivElement>(".transcript")!;
    let scrollTop = 200;
    Object.defineProperty(transcript, "scrollTop", {
      configurable: true,
      get: () => scrollTop,
      set: (value) => { scrollTop = Number(value); },
    });
    const captured = new Set<number>();
    transcript.setPointerCapture = (pointerId) => captured.add(pointerId);
    transcript.hasPointerCapture = (pointerId) => captured.has(pointerId);
    transcript.releasePointerCapture = (pointerId) => captured.delete(pointerId);

    fireEvent.pointerDown(transcript, { pointerId: 7, pointerType: "mouse", button: 0, clientY: 100 });
    fireEvent.pointerMove(transcript, { pointerId: 7, pointerType: "mouse", clientY: 95 });
    expect(scrollTop).toBe(200);
    expect(transcript).not.toHaveClass("transcript--drag-scrolling");

    fireEvent.pointerMove(transcript, { pointerId: 7, pointerType: "mouse", clientY: 94 });
    expect(scrollTop).toBe(206);
    expect(transcript).toHaveClass("transcript--drag-scrolling");

    fireEvent.pointerUp(transcript, { pointerId: 7, pointerType: "mouse", clientY: 94 });
    expect(transcript).not.toHaveClass("transcript--drag-scrolling");
    expect(captured.size).toBe(0);
    expect(fireEvent.click(transcript)).toBe(false);
    expect(fireEvent.click(transcript)).toBe(true);

    fireEvent.pointerDown(transcript, { pointerId: 10, pointerType: "mouse", button: 0, clientY: 100 });
    fireEvent.pointerMove(transcript, { pointerId: 10, pointerType: "mouse", clientY: 80 });
    expect(transcript).toHaveClass("transcript--drag-scrolling");
    fireEvent.pointerCancel(transcript, { pointerId: 10, pointerType: "mouse", clientY: 80 });
    expect(transcript).not.toHaveClass("transcript--drag-scrolling");
    expect(captured.size).toBe(0);
  });

  it("does not take over interactive controls or touch scrolling", () => {
    const view = renderProjectTranscript();
    const transcript = view.container.querySelector<HTMLDivElement>(".transcript")!;
    const button = document.createElement("button");
    transcript.append(button);
    transcript.scrollTop = 120;

    fireEvent.pointerDown(button, { pointerId: 8, pointerType: "mouse", button: 0, clientY: 100 });
    fireEvent.pointerMove(transcript, { pointerId: 8, pointerType: "mouse", clientY: 70 });
    expect(transcript.scrollTop).toBe(120);
    expect(transcript).not.toHaveClass("transcript--drag-scrolling");

    fireEvent.pointerDown(transcript, { pointerId: 9, pointerType: "touch", button: 0, clientY: 100 });
    fireEvent.pointerMove(transcript, { pointerId: 9, pointerType: "touch", clientY: 70 });
    expect(transcript.scrollTop).toBe(120);
  });
});
