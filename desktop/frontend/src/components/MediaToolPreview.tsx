import { memo, useEffect, useMemo, useState } from "react";
import type { GeneratedMediaReference } from "../lib/mediaToolResult";
import {
  openWorkspacePathForOwner,
  readWorkspaceFileForOwner,
  resolveWorkspaceFileOwner,
  useWorkspaceFileScope,
} from "../lib/workspaceFileScope";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";

type ResolvedMedia = GeneratedMediaReference & { source: string };

function mediaLabel(reference: GeneratedMediaReference, index: number): string {
  const value = reference.path || reference.url || "";
  const normalized = value.replace(/\\/g, "/").replace(/\/+$/, "");
  const name = normalized.slice(normalized.lastIndexOf("/") + 1);
  return name || `${reference.type}-${index + 1}`;
}

function mediaLocation(reference: GeneratedMediaReference): string {
  return reference.path || reference.url || "";
}

export const MediaToolPreview = memo(function MediaToolPreview({
  media,
  tabId,
}: {
  media: GeneratedMediaReference[];
  tabId?: string;
}) {
  const t = useT();
  const { showToast } = useToast();
  const workspaceFileScope = useWorkspaceFileScope();
  const owner = useMemo(
    () => resolveWorkspaceFileOwner(tabId, workspaceFileScope),
    [tabId, workspaceFileScope.allowActiveWorkspace, workspaceFileScope.tabId, workspaceFileScope.workspaceRoot],
  );
  const ownerKey = owner.ownerKey;
  const mediaKey = useMemo(() => JSON.stringify(media), [media]);
  const [localSources, setLocalSources] = useState<Record<string, string>>({});

  useEffect(() => {
    let cancelled = false;
    setLocalSources({});
    if (owner.kind === "none") return () => { cancelled = true; };
    for (const reference of media) {
      if (!reference.path) continue;
      readWorkspaceFileForOwner(owner, reference.path)
        .then((file) => {
          if (cancelled || file.err || !file.url || file.kind !== reference.type) return;
          setLocalSources((current) => ({ ...current, [reference.path!]: file.url! }));
        })
        .catch(() => {});
    }
    return () => { cancelled = true; };
  }, [mediaKey, ownerKey]);

  const resolved = media.flatMap<ResolvedMedia>((reference) => {
    const source = reference.path ? localSources[reference.path] : reference.url;
    return source ? [{ ...reference, source }] : [];
  });
  if (resolved.length === 0) return null;

  return (
    <div className="tool-media" aria-label="Generated media">
      {resolved.map((reference, index) => {
        const label = mediaLabel(reference, index);
        const location = mediaLocation(reference);
        const localPath = reference.path;
        return (
          <figure className={`tool-media__item tool-media__item--${reference.type}`} key={`${reference.type}:${reference.path ?? reference.url}`}>
            {reference.type === "image" ? (
              <img src={reference.source} alt={label} decoding="async" draggable={false} />
            ) : (
              <video src={reference.source} controls playsInline preload="metadata" aria-label={label} />
            )}
            <figcaption title={location}>
              {localPath ? (
                <a
                  href={localPath}
                  onClick={(event) => {
                    event.preventDefault();
                    void openWorkspacePathForOwner(owner, localPath).catch(() => {
                      showToast(t("msg.openFileFailed"), "warn");
                    });
                  }}
                >
                  {location}
                </a>
              ) : (
                <a href={reference.url} target="_blank" rel="noreferrer">{location}</a>
              )}
            </figcaption>
          </figure>
        );
      })}
    </div>
  );
});
