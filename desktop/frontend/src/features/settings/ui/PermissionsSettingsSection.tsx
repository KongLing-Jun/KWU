import { app } from "../../../lib/bridge";
import { useT } from "../../../lib/i18n";
import type { SettingsView } from "../../../lib/types";
import { RuleList } from "./RuleList";
import { SettingsField, SettingsSection } from "./SettingsPrimitives";

export function PermissionsSettingsSection({
  settings,
  busy,
  apply,
}: {
  settings: SettingsView;
  busy: boolean;
  apply: (fn: () => Promise<unknown>) => Promise<void>;
}) {
  const t = useT();
  return (
    <>
      <SettingsSection title={t("settings.permissions")} description={t("settings.permissionsModeHint")}>
        <SettingsField label={t("settings.writerMode")}>
          <select
            className="mem-select set-grow"
            value={settings.permissions.mode}
            disabled={busy}
            onChange={(event) => void apply(() => app.SetPermissionMode(event.target.value))}
          >
            <option value="ask">{t("settings.modeAsk")}</option>
            <option value="allow">{t("settings.modeAllow")}</option>
            <option value="deny">{t("settings.modeDeny")}</option>
          </select>
        </SettingsField>
      </SettingsSection>
      <SettingsSection title={t("settings.permissionRules")} description={t("settings.ruleForm")}>
        <div className="set-rules-grid">
          {(["deny", "ask", "allow"] as const).map((list) => (
            <RuleList
              key={list}
              list={list}
              rules={settings.permissions[list]}
              busy={busy}
              onAdd={(rule) => apply(() => app.AddPermissionRule(list, rule))}
              onRemove={(rule) => apply(() => app.RemovePermissionRule(list, rule))}
            />
          ))}
        </div>
      </SettingsSection>
    </>
  );
}
