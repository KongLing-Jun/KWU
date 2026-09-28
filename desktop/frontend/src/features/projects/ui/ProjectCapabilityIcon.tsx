import { useEffect, useState } from "react";
import { Boxes, Sparkles, Users } from "lucide-react";

export type ProjectCapabilityIconKind = "connector" | "expert" | "skill";

export function ProjectCapabilityIcon({ kind, iconUrl, className = "" }: {
  kind: ProjectCapabilityIconKind;
  iconUrl?: string;
  className?: string;
}) {
  const [failed, setFailed] = useState(false);
  const Icon = { connector: Boxes, expert: Users, skill: Sparkles }[kind];

  useEffect(() => setFailed(false), [iconUrl]);

  if (iconUrl && !failed) {
    return <img className={`project-capability-icon__image${className ? ` ${className}` : ""}`} src={iconUrl} alt="" onError={() => setFailed(true)} />;
  }
  return <Icon className={className} size={18} aria-hidden="true" />;
}
