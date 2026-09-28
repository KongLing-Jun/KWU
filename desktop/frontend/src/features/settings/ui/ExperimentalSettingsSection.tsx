import { useT } from "../../../lib/i18n";
import { SettingsField, SettingsSection, ToggleSegment } from "./SettingsPrimitives";

export function ExperimentalSettingsSection({
  superpowersEnabled,
  onSuperpowersEnabledChange,
}: {
  superpowersEnabled: boolean;
  onSuperpowersEnabledChange: (enabled: boolean) => void;
}) {
  const t = useT();
  return (
    <SettingsSection>
      <SettingsField
        label={t("settings.experimental.superpowers")}
        hint={t("settings.experimental.superpowersHint")}
      >
        <ToggleSegment
          value={superpowersEnabled}
          disabled={false}
          onChange={onSuperpowersEnabledChange}
        />
      </SettingsField>
    </SettingsSection>
  );
}
