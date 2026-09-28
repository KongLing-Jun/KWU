import { useEffect, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { RefreshCw } from "lucide-react";
import {
  comboFromKeyboardEvent,
  detectShortcutPlatform,
  formatShortcutCombo,
  onShortcutsChanged,
  resetCustomShortcuts,
  resolvedShortcutCombo,
  saveCustomShortcut,
  shortcutConflict,
  shortcutDefinitions,
  type ShortcutAction,
} from "../../../lib/keyboardShortcuts";
import { useT } from "../../../lib/i18n";
import { ShortcutComboDisplay } from "../../../components/ShortcutComboDisplay";
import { SettingsSection } from "./SettingsPrimitives";

export function ShortcutsSettingsSection() {
  const t = useT();
  const [platform] = useState(() => detectShortcutPlatform());
  const [revision, setRevision] = useState(0);
  const [recording, setRecording] = useState<ShortcutAction | null>(null);
  const [conflict, setConflict] = useState<{ action: ShortcutAction; conflictAction: ShortcutAction } | null>(null);

  useEffect(() => onShortcutsChanged(() => setRevision((value) => value + 1)), []);

  const definitions = shortcutDefinitions();
  const commitShortcut = (action: ShortcutAction, event: ReactKeyboardEvent<HTMLButtonElement>) => {
    const combo = comboFromKeyboardEvent(event.nativeEvent);
    if (!combo) return;
    event.preventDefault();
    event.stopPropagation();
    const conflictDefinition = shortcutConflict(action, combo, platform);
    if (conflictDefinition) {
      setConflict({ action, conflictAction: conflictDefinition.action });
      return;
    }
    saveCustomShortcut(action, combo);
    setConflict(null);
    setRecording(null);
    setRevision((value) => value + 1);
  };

  return (
    <SettingsSection
      title={t("settings.shortcutsTitle")}
      description={t("settings.shortcutsHint")}
      actions={
        <button
          className="chip chip--icon"
          type="button"
          title={t("settings.shortcutsResetAll")}
          aria-label={t("settings.shortcutsResetAll")}
          onClick={() => {
            resetCustomShortcuts();
            setConflict(null);
            setRecording(null);
            setRevision((value) => value + 1);
          }}
        >
          <RefreshCw size={14} />
        </button>
      }
    >
      <div className="shortcuts-settings" data-revision={revision}>
        {conflict && (
          <div className="shortcuts-settings__conflict" role="alert">
            {t("settings.shortcutsConflict", {
              action: t(definitions.find((definition) => definition.action === conflict.action)?.labelKey ?? "settings.tab.shortcuts"),
              conflict: t(definitions.find((definition) => definition.action === conflict.conflictAction)?.labelKey ?? "settings.tab.shortcuts"),
            })}
          </div>
        )}
        {definitions.map((definition) => {
          const resolved = resolvedShortcutCombo(definition.action, platform);
          const defaultCombo = definition.defaults[platform];
          const display = formatShortcutCombo(resolved, platform);
          const isCustom = formatShortcutCombo(resolved, platform) !== formatShortcutCombo(defaultCombo, platform);
          const isRecording = recording === definition.action;
          return (
            <div className="shortcuts-settings__row" key={definition.action}>
              <div className="shortcuts-settings__copy">
                <div className="shortcuts-settings__label">{t(definition.labelKey)}</div>
                <div className="shortcuts-settings__desc">{t(definition.descriptionKey)}</div>
              </div>
              <div className="shortcuts-settings__control">
                <button
                  className={`shortcuts-settings__key${isRecording ? " shortcuts-settings__key--recording" : ""}${definition.configurable === false ? " shortcuts-settings__key--locked" : ""}`}
                  type="button"
                  disabled={definition.configurable === false}
                  aria-label={isRecording ? t("settings.shortcutsRecording") : display}
                  aria-pressed={isRecording}
                  onClick={() => {
                    setRecording(definition.action);
                    setConflict(null);
                  }}
                  onKeyDown={(event) => isRecording && commitShortcut(definition.action, event)}
                >
                  {isRecording ? t("settings.shortcutsRecording") : <ShortcutComboDisplay combo={resolved} platform={platform} />}
                </button>
                <button
                  className="chip"
                  type="button"
                  disabled={!isCustom}
                  onClick={() => {
                    saveCustomShortcut(definition.action, null);
                    setConflict(null);
                    setRecording(null);
                    setRevision((value) => value + 1);
                  }}
                >
                  {t("settings.shortcutsReset")}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </SettingsSection>
  );
}
