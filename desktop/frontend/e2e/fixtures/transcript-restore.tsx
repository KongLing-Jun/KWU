import { useCallback, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import gsap from "gsap";
import { ScrollToPlugin } from "gsap/ScrollToPlugin";
import { Transcript } from "../../src/components/Transcript";
import { LocaleProvider } from "../../src/lib/i18n";
import type { Item, SessionScrollAnchor } from "../../src/lib/useController";
import { markNoticeForReveal } from "../../src/features/session/sessionItems";
import "../../src/styles.css";
gsap.registerPlugin(ScrollToPlugin);

const params = new URLSearchParams(location.search);
const pending = params.get("pending") === "true";
const initial: SessionScrollAnchor = params.has("bottom") ? { atBottom: true } : { atBottom: false, anchorMessageId: params.get("missing") ? "missing" : "q0", offset: 10000 };
let resolveTeam: (value: unknown) => void = () => {};
const teamPromise = new Promise((resolve) => { resolveTeam = resolve; });
window.go = { main: { App: {
  ExpertTeamForTab: () => pending ? teamPromise : Promise.resolve(null),
} } } as unknown as typeof window.go;
const messages: Item[] = Array.from({ length: Number(params.get("turns") || 2) }, (_, index) => [
  { kind: "user", id: `q${index}`, text: `Question ${index}` },
  { kind: "assistant", id: `a${index}`, text: `Readable answer ${index}.\n\n` + "A synthetic history paragraph. ".repeat(8), reasoning: "", streaming: false },
]).flat() as Item[];

function Fixture() {
  const [view, setView] = useState("fixture-a");
  const [writes, setWrites] = useState<SessionScrollAnchor[]>([]);
  const [interrupted, setInterrupted] = useState(false);
  const displayedItems = useMemo(() => interrupted ? [...messages, markNoticeForReveal({ kind: "notice", id: "interrupted", level: "warn", text: "Interrupted" })] : messages, [interrupted]);
  const saveAnchor = useCallback((_: string, anchor: SessionScrollAnchor) => setWrites((previous) => JSON.stringify(previous.at(-1)) === JSON.stringify(anchor) ? previous : [...previous, anchor]), []);
  return <LocaleProvider>
    <div style={{ height: "100dvh", display: "flex", flexDirection: "column" }}>
      <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
        <button onClick={() => resolveTeam({ state: "team_archived", name: "Fixture team", lead: { name: "Lead" }, members: [] })}>Resolve team</button>
        <button onClick={() => setView("fixture-b")}>Switch session</button>
        <button onClick={() => setInterrupted(true)}>Interrupt</button>
        <output data-testid="saved-anchor">{JSON.stringify(writes.at(-1) ?? initial)}</output>
      </div>
      <Transcript items={displayedItems} viewKey={view} tabId={view} initialScrollAnchor={view === "fixture-a" ? initial : { atBottom: true }}
        activeExpert={pending && !params.has("archived") ? { id: "fixture-team", name: "Fixture team", mode: "team" } : null}
        onScrollAnchorChange={saveAnchor}
        questionNavigator={false} onPrompt={() => {}} />
    </div>
  </LocaleProvider>;
}
createRoot(document.getElementById("root")!).render(<Fixture />);
