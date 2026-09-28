import { act, render } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { Transcript } from "../components/Transcript";
import { LocaleProvider } from "../lib/i18n";
import type { Item, SessionScrollAnchor } from "../lib/useController";

vi.mock("gsap", () => {
  const gsap = { to: () => ({ kill() {} }) };
  return { default: gsap, gsap };
});

function transcriptItems(rowCount: number): Item[] {
  return [
    { kind: "user", id: "live-question", text: "Create the requested deliverables" },
    ...Array.from({ length: rowCount - 1 }, (_, index): Item => ({
      kind: "notice",
      id: `live-progress-${index}`,
      level: "info",
      text: `progress ${index}`,
    })),
  ];
}

function LiveTranscript({
  viewKey,
  rowCount,
  scrollAnchor,
  onScrollAnchorChange,
}: {
  viewKey: string;
  rowCount: number;
  scrollAnchor: SessionScrollAnchor;
  onScrollAnchorChange?: (key: string, anchor: SessionScrollAnchor) => void;
}) {
  return (
    <LocaleProvider>
      <Transcript
        items={transcriptItems(rowCount)}
        viewKey={viewKey}
        initialScrollAnchor={scrollAnchor}
        onScrollAnchorChange={onScrollAnchorChange}
        onPrompt={() => {}}
        questionNavigator={false}
        running
      />
    </LocaleProvider>
  );
}

test("live ordinary transcript stays visible when it crosses the virtualization threshold", async () => {
  Object.defineProperty(window, "go", {
    configurable: true,
    value: { main: { App: { ExpertTeamForTab: async () => null } } },
  });
  const bottomAnchor: SessionScrollAnchor = { atBottom: true };
  const capturedVirtualizationAnchor: SessionScrollAnchor = {
    atBottom: false,
    anchorMessageId: "live-question",
    offset: -50,
  };
  const anchorWrites: SessionScrollAnchor[] = [];
  const onScrollAnchorChange = (_key: string, anchor: SessionScrollAnchor) => anchorWrites.push(anchor);
  const view = render(<LiveTranscript viewKey="draft-tab" rowCount={9} scrollAnchor={bottomAnchor} onScrollAnchorChange={onScrollAnchorChange} />);
  await act(async () => Promise.resolve());

  await act(async () => {
    view.rerender(<LiveTranscript viewKey="persisted-session" rowCount={9} scrollAnchor={bottomAnchor} onScrollAnchorChange={onScrollAnchorChange} />);
    await Promise.resolve();
  });
  expect(document.querySelector(".transcript-virtual-list")).toBeNull();

  const transcript = document.querySelector<HTMLDivElement>(".transcript");
  expect(transcript).not.toBeNull();
  let scrollTop = 600;
  let scrollHeight = 1_000;
  Object.defineProperty(transcript, "clientHeight", { configurable: true, value: 400 });
  Object.defineProperty(transcript, "scrollHeight", { configurable: true, get: () => scrollHeight });
  Object.defineProperty(transcript, "scrollTop", {
    configurable: true,
    get: () => scrollTop,
    set: (value) => { scrollTop = Math.max(0, Math.min(Number(value), scrollHeight - 400)); },
  });
  await act(async () => {
    transcript?.dispatchEvent(new Event("scroll", { bubbles: true }));
    await Promise.resolve();
  });
  expect(anchorWrites[anchorWrites.length - 1]).toEqual({ atBottom: true });

  scrollHeight = 1_800;
  await act(async () => {
    // Runtime tracing proves that the ordinary-to-virtual transition writes this
    // programmatically captured anchor back through the moving cache prop.
    view.rerender(<LiveTranscript viewKey="persisted-session" rowCount={10} scrollAnchor={capturedVirtualizationAnchor} onScrollAnchorChange={onScrollAnchorChange} />);
    await Promise.resolve();
  });
  expect(document.querySelector(".transcript-virtual-list")).not.toBeNull();

  for (let reset = 0; reset < 2; reset += 1) {
    await act(async () => {
      scrollTop = 0;
      transcript?.dispatchEvent(new Event("scroll", { bubbles: true }));
      await Promise.resolve();
    });
    expect(scrollTop).toBe(1_400);
    expect(anchorWrites[anchorWrites.length - 1]).toEqual({ atBottom: true });
  }

  const presentation = document.querySelector<HTMLElement>('[data-testid="ordinary-semantic-presentation"]');
  expect(presentation).not.toBeNull();
  expect(presentation).not.toHaveAttribute("aria-hidden", "true");
  expect(presentation?.style.visibility).not.toBe("hidden");
  expect(document.querySelector(".transcript__session-loading")).toBeNull();
});
