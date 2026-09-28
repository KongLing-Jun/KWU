import { useMemo, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { getAvailableFontFamilies, getAvailableMonoFontFamilies } from "../../../lib/fontAvailability";
import type { FontFamily, MonoFontFamily } from "../../../lib/fontFamily";
import { useT } from "../../../lib/i18n";
import { TEXT_SIZES, type TextSize } from "../../../lib/textSize";
import type { Theme } from "../../../lib/theme";

function handleRadioKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>) {
  const group = event.currentTarget.closest<HTMLElement>('[role="radiogroup"]');
  if (!group) return;
  const options = Array.from(group.querySelectorAll<HTMLButtonElement>('button[role="radio"]:not(:disabled)'));
  const currentIndex = options.indexOf(event.currentTarget);
  if (currentIndex < 0) return;

  let nextIndex: number | undefined;
  switch (event.key) {
    case "ArrowRight":
    case "ArrowDown":
      nextIndex = (currentIndex + 1) % options.length;
      break;
    case "ArrowLeft":
    case "ArrowUp":
      nextIndex = (currentIndex - 1 + options.length) % options.length;
      break;
    case "Home":
      nextIndex = 0;
      break;
    case "End":
      nextIndex = options.length - 1;
      break;
    default:
      return;
  }

  event.preventDefault();
  options[nextIndex]?.focus();
  options[nextIndex]?.click();
}

export function AppearanceSection({
  theme,
  textSize,
  fontFamily,
  monoFontFamily,
  customFontName,
  customMonoFontName,
  onTheme,
  onTextSize,
  onFontFamily,
  onMonoFontFamily,
  onCustomFontNameChange,
  onCustomMonoFontNameChange,
}: {
  theme: Theme;
  textSize: TextSize;
  fontFamily: FontFamily;
  monoFontFamily: MonoFontFamily;
  customFontName: string;
  customMonoFontName: string;
  onTheme: (theme: Theme) => void;
  onTextSize: (size: TextSize) => void;
  onFontFamily: (font: FontFamily) => void;
  onMonoFontFamily: (font: MonoFontFamily) => void;
  onCustomFontNameChange: (name: string) => void;
  onCustomMonoFontNameChange: (name: string) => void;
}) {
  const t = useT();
  const themeOptions: Theme[] = ["auto", "light", "dark"];
  const availableFontFamilies = useMemo(() => getAvailableFontFamilies(fontFamily), [fontFamily]);
  const availableMonoFontFamilies = useMemo(() => getAvailableMonoFontFamilies(monoFontFamily), [monoFontFamily]);

  return (
    <div className="appearance-panel">
      <section className="appearance-panel__section">
        <div className="appearance-panel__setting-row">
          <div className="appearance-panel__setting-text">
            <div className="appearance-panel__setting-name">{t("settings.theme")}</div>
            <div className="appearance-panel__setting-desc">{t("settings.themeDesc")}</div>
          </div>
          <div className="appearance-panel__control">
            <div className="set-seg" role="radiogroup" aria-label={t("settings.theme")}>
              {themeOptions.map((option) => {
                const selected = theme === option;
                return (
                  <button
                    key={option}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    tabIndex={selected ? 0 : -1}
                    className={`set-seg__btn${selected ? " set-seg__btn--on" : ""}`}
                    onClick={() => onTheme(option)}
                    onKeyDown={handleRadioKeyDown}
                  >
                    {themeName(option, t)}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </section>

      <section className="appearance-panel__section">
        <div className="appearance-panel__setting-row">
          <div className="appearance-panel__setting-text">
            <div className="appearance-panel__setting-name">{t("settings.textSize")}</div>
            <div className="appearance-panel__setting-desc">{t("settings.textSizeDesc")}</div>
          </div>
          <div className="appearance-panel__control">
            <div className="set-seg" role="radiogroup" aria-label={t("settings.textSize")}>
              {TEXT_SIZES.map((size) => {
                const selected = textSize === size;
                return (
                  <button
                    key={size}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    tabIndex={selected ? 0 : -1}
                    className={`set-seg__btn${selected ? " set-seg__btn--on" : ""}`}
                    onClick={() => onTextSize(size)}
                    onKeyDown={handleRadioKeyDown}
                  >
                    {textSizeName(size, t)}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </section>

      <section className="appearance-panel__section">
        <div className="appearance-panel__setting-row">
          <div className="appearance-panel__setting-text">
            <div className="appearance-panel__setting-name">{t("settings.fontFamily")}</div>
            <div className="appearance-panel__setting-desc">{t("settings.fontFamilyDesc")}</div>
          </div>
          <div className="appearance-panel__control">
            <div className="set-seg" role="radiogroup" aria-label={t("settings.fontFamily")}>
              {availableFontFamilies.map((font) => {
                const selected = fontFamily === font;
                return (
                  <button
                    key={font}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    tabIndex={selected ? 0 : -1}
                    className={`set-seg__btn${selected ? " set-seg__btn--on" : ""}`}
                    onClick={() => onFontFamily(font)}
                    onKeyDown={handleRadioKeyDown}
                  >
                    {fontFamilyName(font, t)}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
        {fontFamily === "custom" && (
          <div className="appearance-panel__custom-field">
            <textarea
              className="appearance-panel__custom-input"
              rows={2}
              name="custom-font-family"
              autoComplete="off"
              spellCheck={false}
              aria-label={t("settings.fontFamilyCustomName")}
              placeholder={t("settings.fontFamilyCustomPlaceholder")}
              value={customFontName}
              onChange={(event) => onCustomFontNameChange(event.target.value)}
            />
          </div>
        )}
      </section>

      <section className="appearance-panel__section">
        <div className="appearance-panel__setting-row">
          <div className="appearance-panel__setting-text">
            <div className="appearance-panel__setting-name">{t("settings.monoFontFamily")}</div>
            <div className="appearance-panel__setting-desc">{t("settings.monoFontFamilyDesc")}</div>
          </div>
          <div className="appearance-panel__control">
            <div className="set-seg" role="radiogroup" aria-label={t("settings.monoFontFamily")}>
              {availableMonoFontFamilies.map((font) => {
                const selected = monoFontFamily === font;
                return (
                  <button
                    key={font}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    tabIndex={selected ? 0 : -1}
                    className={`set-seg__btn${selected ? " set-seg__btn--on" : ""}`}
                    onClick={() => onMonoFontFamily(font)}
                    onKeyDown={handleRadioKeyDown}
                  >
                    {monoFontFamilyName(font, t)}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
        {monoFontFamily === "custom" && (
          <div className="appearance-panel__custom-field">
            <textarea
              className="appearance-panel__custom-input"
              rows={2}
              name="custom-monospace-font-family"
              autoComplete="off"
              spellCheck={false}
              aria-label={t("settings.monoFontFamilyCustomName")}
              placeholder={t("settings.monoFontFamilyCustomPlaceholder")}
              value={customMonoFontName}
              onChange={(event) => onCustomMonoFontNameChange(event.target.value)}
            />
          </div>
        )}
      </section>
    </div>
  );
}

function themeName(theme: Theme, t: ReturnType<typeof useT>): string {
  switch (theme) {
    case "auto": return t("settings.themeAuto");
    case "light": return t("settings.themeLight");
    case "dark": return t("settings.themeDark");
  }
}

function textSizeName(size: TextSize, t: ReturnType<typeof useT>): string {
  switch (size) {
    case "small": return t("settings.textSizeSmall");
    case "default": return t("settings.textSizeDefault");
    case "large": return t("settings.textSizeLarge");
    case "xlarge": return t("settings.textSizeXLarge");
    case "xxlarge": return t("settings.textSizeXXLarge");
  }
}

function fontFamilyName(font: FontFamily, t: ReturnType<typeof useT>): string {
  switch (font) {
    case "system": return t("settings.fontFamilySystem");
    case "yahei": return t("settings.fontFamilyYaHei");
    case "pingfang": return t("settings.fontFamilyPingFang");
    case "noto": return t("settings.fontFamilyNoto");
    case "custom": return t("settings.fontFamilyCustom");
  }
}

function monoFontFamilyName(font: MonoFontFamily, t: ReturnType<typeof useT>): string {
  switch (font) {
    case "system": return t("settings.monoFontFamilySystem");
    case "cascadia": return t("settings.monoFontFamilyCascadia");
    case "jetbrains": return t("settings.monoFontFamilyJetBrains");
    case "sfmono": return t("settings.monoFontFamilySFMono");
    case "custom": return t("settings.monoFontFamilyCustom");
  }
}
