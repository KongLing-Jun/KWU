import { X } from "lucide-react";
import type { ExpertSelection } from "./ExpertDirectory";

interface ActiveExpertBannerProps {
  expert: ExpertSelection;
  clearLabel: string;
  onClear: () => void;
}

function expertInitials(name: string): string {
  const chars = Array.from(name.trim());
  return (chars[0] ?? "?").toUpperCase();
}

export function ActiveExpertBanner({ expert, clearLabel, onClear }: ActiveExpertBannerProps) {
  return (
    <div className="expert-active-banner" aria-label={`${expert.name} ${expert.mode}`}>
      <div className="expert-active-banner__main">
        <span className="expert-active-banner__avatar" aria-hidden="true">
          {expert.avatarSrc ? (
            <img src={expert.avatarSrc} alt="" loading="lazy" decoding="async" />
          ) : (
            <span>{expertInitials(expert.name)}</span>
          )}
        </span>
        <div className="expert-active-banner__copy">
          <strong>{expert.name}</strong>
        </div>
      </div>
      <button className="expert-active-banner__clear" type="button" aria-label={clearLabel} onClick={onClear}>
        <X size={15} aria-hidden="true" />
      </button>
    </div>
  );
}
