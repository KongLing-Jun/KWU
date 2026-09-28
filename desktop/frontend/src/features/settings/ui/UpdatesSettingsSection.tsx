import { app } from "../../../lib/bridge";
import { UPDATES_ENABLED } from "../../../lib/features";
import { useT } from "../../../lib/i18n";
import { Tooltip } from "../../../components/Tooltip";

// This page owns updater preferences only. Manual checks and installs live in
// the account menu's update center, so opening settings never starts network I/O.
export function UpdatesSettingsSection({
  configPath,
  checkUpdates,
  settingsBusy,
  applySettings,
}: {
  configPath: string;
  checkUpdates: boolean;
  settingsBusy: boolean;
  applySettings: (fn: () => Promise<void>) => Promise<void>;
}) {
  const t = useT();

  return (
    <div className="updates-settings-panel">
      <section className="updates-settings-panel__section">
        <h3 className="updates-settings-panel__section-title">{t("updater.title")}</h3>
        {UPDATES_ENABLED && (
          <div className="updates-settings-panel__card">
            <div className="updates-settings-panel__row">
              <div className="updates-settings-panel__copy">
                <div className="updates-settings-panel__label">{t("updater.autoCheckLabel")}</div>
                <div className="updates-settings-panel__description">{t("updater.autoCheckHint")}</div>
              </div>
              <label className="updates-settings-panel__switch">
                <input
                  type="checkbox"
                  checked={checkUpdates}
                  disabled={settingsBusy}
                  aria-label={t("updater.autoCheckLabel")}
                  onChange={(event) => void applySettings(() => app.SetDesktopCheckUpdates(event.target.checked))}
                />
                <span className="updates-settings-panel__switch-track" aria-hidden="true" />
              </label>
            </div>
          </div>
        )}
      </section>
      {configPath && (
        <Tooltip label={configPath} fill block className="updates-settings-panel__config settings-config-path">
          <span className="updates-settings-panel__config-text">
            {t("settings.config", { path: configPath })}
          </span>
        </Tooltip>
      )}
    </div>
  );
}
