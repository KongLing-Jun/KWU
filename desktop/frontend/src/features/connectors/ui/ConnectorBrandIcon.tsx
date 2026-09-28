import { useEffect, useState, type CSSProperties } from "react";
import type { ConnectorCatalogItemView } from "../../../lib/bridge";

export function ConnectorBrandIcon({ item, name, size = 24 }: {
  item?: Pick<ConnectorCatalogItemView, "icon_url">;
  name: string;
  size?: number;
}) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [item?.icon_url]);
  const fallback = Array.from(name.trim())[0]?.toUpperCase() || "C";
  return (
    <span
      className="connector-brand-icon"
      style={{ "--connector-icon-size": `${size}px` } as CSSProperties}
      aria-hidden="true"
    >
      {item?.icon_url && !failed
        ? <img src={item.icon_url} alt="" onError={() => setFailed(true)} />
        : <span>{fallback}</span>}
    </span>
  );
}
