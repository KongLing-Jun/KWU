import { useState } from "react";
import { app } from "../../../lib/bridge";
import { useT } from "../../../lib/i18n";
import type { SettingsView } from "../../../lib/types";
import { RuleList } from "./RuleList";
import { SettingsField, SettingsSection } from "./SettingsPrimitives";

export function SandboxSettingsSection({
  settings,
  busy,
  apply,
}: {
  settings: SettingsView;
  busy: boolean;
  apply: (fn: () => Promise<unknown>) => Promise<void>;
}) {
  const t = useT();
  const sandbox = settings.sandbox;
  const [root, setRoot] = useState(sandbox.workspaceRoot);
  const set = (next: Partial<typeof sandbox>) =>
    apply(() => app.SetSandbox(
      next.bash ?? sandbox.bash,
      next.network ?? sandbox.network,
      next.workspaceRoot ?? sandbox.workspaceRoot,
      next.allowWrite ?? sandbox.allowWrite,
      next.shell ?? sandbox.shell,
    ));

  return (
    <SettingsSection title={t("settings.sandboxTitle")}>
      <SettingsField label={t("settings.shellInterpreter")}>
        <select className="mem-select set-grow" value={sandbox.shell || "auto"} disabled={busy} onChange={(event) => void set({ shell: event.target.value })}>
          <option value="auto">{t("settings.shellAuto")}</option>
          <option value="bash">{t("settings.shellBash")}</option>
          <option value="powershell">{t("settings.shellPowershell")}</option>
          <option value="pwsh">{t("settings.shellPwsh")}</option>
        </select>
      </SettingsField>
      <SettingsField label={t("settings.bashSandbox")}>
        <select className="mem-select set-grow" value={sandbox.bash} disabled={busy} onChange={(event) => void set({ bash: event.target.value })}>
          <option value="enforce">{t("settings.bashEnforce")}</option>
          <option value="off">{t("settings.bashOff")}</option>
        </select>
      </SettingsField>
      <SettingsField label={t("settings.allowNetwork")}>
        <label className="set-check set-check--inline">
          <input type="checkbox" checked={sandbox.network} disabled={busy} onChange={(event) => void set({ network: event.target.checked })} />
          {t("settings.allowNetwork")}
        </label>
      </SettingsField>
      <SettingsField label={t("settings.workspaceRoot")}>
        <input
          className="mem-input set-grow"
          placeholder={t("settings.workspaceDefault")}
          value={root}
          disabled={busy}
          onChange={(event) => setRoot(event.target.value)}
          onBlur={() => root !== sandbox.workspaceRoot && void set({ workspaceRoot: root })}
        />
      </SettingsField>
      <RuleList
        list="allow_write"
        rules={sandbox.allowWrite}
        busy={busy}
        onAdd={(rule) => set({ allowWrite: [...sandbox.allowWrite, rule] })}
        onRemove={(rule) => set({ allowWrite: sandbox.allowWrite.filter((value) => value !== rule) })}
      />
    </SettingsSection>
  );
}
