import { useState } from "react";
import { createRoot } from "react-dom/client";
import gsap from "gsap";
import { useGSAP } from "@gsap/react";
import { ScrollToPlugin } from "gsap/ScrollToPlugin";
import { Transcript } from "../../src/components/Transcript";
import { LocaleProvider } from "../../src/lib/i18n";
import { setDisplayMode } from "../../src/lib/displayMode";
import { scenarios, taskNoticeScenario } from "./task-notice-scenarios";
import "../../src/styles.css";

gsap.registerPlugin(useGSAP, ScrollToPlugin);

function Fixture() {
  const [scenario, setScenario] = useState("retry");
  const [compact, setCompact] = useState(false);
  const [creation, setCreation] = useState(false);
  const [settled, setSettled] = useState(false);
  const items = taskNoticeScenario(scenario);
  return (
    <LocaleProvider>
      <main style={{ height: "100vh", display: "flex", flexDirection: "column", maxWidth: 1000, margin: "auto" }}>
        <nav style={{ display: "flex", flexWrap: "wrap", gap: 16, padding: 12 }}>
          <select aria-label="Scenario" value={scenario} onChange={(event) => setScenario(event.target.value)}>
            {scenarios.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
          <label><input type="checkbox" checked={compact} onChange={(event) => { setCompact(event.target.checked); setDisplayMode(event.target.checked ? "compact" : "standard"); }} />Compact</label>
          <label><input type="checkbox" checked={creation} onChange={(event) => setCreation(event.target.checked)} />Creation</label>
          <label><input type="checkbox" checked={settled} onChange={(event) => setSettled(event.target.checked)} />Settled</label>
        </nav>
        <Transcript items={items} onPrompt={() => {}} running={!settled} creationMode={creation} questionNavigator={false} />
      </main>
    </LocaleProvider>
  );
}

setDisplayMode("standard");
createRoot(document.getElementById("root")!).render(<Fixture />);
