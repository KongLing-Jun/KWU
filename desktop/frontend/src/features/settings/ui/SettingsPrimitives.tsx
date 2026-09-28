import type { ReactNode } from "react";
import { useT } from "../../../lib/i18n";
import { Tooltip } from "../../../components/Tooltip";

export function SettingsSection({
  title,
  description,
  actions,
  children,
}: {
  title?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const hasHead = Boolean(title || description || actions);
  return (
    <section className="settings-section">
      {hasHead && (
        <div className="settings-section__head">
          <div>
            {title && <div className="settings-section__title">{title}</div>}
            {description && (
              <div className="settings-section__desc">
                <SettingsHint hint={description} />
              </div>
            )}
          </div>
          {actions && <div className="settings-section__actions">{actions}</div>}
        </div>
      )}
      <div className="settings-section__body">{children}</div>
    </section>
  );
}

export function SettingsField({
  label,
  hint,
  children,
  className,
  stacked = false,
}: {
  label: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
  className?: string;
  stacked?: boolean;
}) {
  return (
    <div className={`settings-field${stacked ? " settings-field--stacked" : ""}${className ? ` ${className}` : ""}`}>
      <div className="settings-field__copy">
        <div className="settings-field__label">{label}</div>
        {hint && (
          <div className="settings-field__hint">
            <SettingsHint hint={hint} />
          </div>
        )}
      </div>
      <div className="settings-field__control">{children}</div>
    </div>
  );
}

export function ToggleSegment({
  value,
  disabled,
  onLabel,
  offLabel,
  onChange,
}: {
  value: boolean;
  disabled: boolean;
  onLabel?: string;
  offLabel?: string;
  onChange: (value: boolean) => void;
}) {
  const t = useT();
  return (
    <div className="set-seg">
      <button
        type="button"
        className={`set-seg__btn${value ? " set-seg__btn--on" : ""}`}
        disabled={disabled}
        onClick={() => onChange(true)}
      >
        {onLabel ?? t("settings.toggleOn")}
      </button>
      <button
        type="button"
        className={`set-seg__btn${!value ? " set-seg__btn--on" : ""}`}
        disabled={disabled}
        onClick={() => onChange(false)}
      >
        {offLabel ?? t("settings.toggleOff")}
      </button>
    </div>
  );
}

function SettingsHint({ hint }: { hint: ReactNode }) {
  if (typeof hint === "string" || typeof hint === "number") {
    const label = String(hint);
    return (
      <Tooltip label={label} fill block className="settings-field__hint-tooltip">
        <span className="settings-field__hint-line">{label}</span>
      </Tooltip>
    );
  }
  return hint;
}
