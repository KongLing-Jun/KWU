import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import { CodeViewer } from "./CodeViewer";
import { normalizeMath } from "./mathNormalize";
import { openExternal } from "../lib/bridge";
import { localFilePathFromHref } from "../lib/attachmentDisplay";
import { useT } from "../lib/i18n";
import { useToast } from "../lib/toast";
import { openWorkspacePathForOwner, readWorkspaceFileForOwner, resolveWorkspaceFileOwner, useWorkspaceFileScope, type WorkspaceFileOwner } from "../lib/workspaceFileScope";

void import("katex/dist/katex.min.css").catch(() => {});

// Markdown rendering via react-markdown + remark-gfm (tables, task lists,
// strike, autolinks) and remark-math + rehype-katex for $/$$ KaTeX math.
// Fenced code blocks go through CodeViewer for syntax highlighting; inline
// code is a styled <code>. Links open in the system browser.
//
// The math pre-pass in mathNormalize normalises LLM-native \(…\)/\[…\]
// delimiters to the $/$$ syntax remark-math understands, gates single-$
// pairs through a classifier to avoid false positives on $5, $PATH, etc.,
// and runs KaTeX-specific normalisations (text-mode escapes, |→\vert).

const STATUS_MARKER_RE = /(?:✅|☑|☒|✔️?|✓|\[[xX ]\])/;
const STATUS_MARKER_GLOBAL_RE = /(?:✅|☑|☒|✔️?|✓|\[[xX ]\])/g;
const BULLET_RE = /^[-*•]\s+\S/;
const DIVIDER_RE = /^[\s\-_=─━—]+$/;

function splitStatusLine(line: string): string[] {
  const parts = (line.match(STATUS_MARKER_GLOBAL_RE) ?? []).length > 1
    ? line.split(/(?=(?:✅|☑|☒|✔️?|✓|\[[xX ]\]))/)
    : [line];
  return parts
    .map((part) => part.replace(/^(?:✅|☑|☒|✔️?|✓|\[[xX ]\]|[-*•])\s*/i, "").trim())
    .filter(Boolean)
    .map((part) => part.replace(/\s{2,}/g, " · "));
}

function looksLikeDiagram(text: string): boolean {
  return /[←→↔]|<{1,2}-{2,}|-{2,}>{1,2}|[-_=─━]{6,}/.test(text);
}

function splitPlainBlock(text: string): { preText: string; statusItems: string[] } {
  const items: string[] = [];
  const preLines: string[] = [];
  const lines = text.split(/\r?\n/);
  const bulletLines = lines.filter((line) => BULLET_RE.test(line.trim())).length;
  const collectBulletLines = bulletLines >= 2 && !looksLikeDiagram(text);
  for (const rawLine of lines) {
    const line = rawLine.trim();
    const marked = STATUS_MARKER_RE.test(line) || (collectBulletLines && BULLET_RE.test(line));
    if (marked) {
      items.push(...splitStatusLine(line));
    } else if (DIVIDER_RE.test(line) && items.length > 0 && !looksLikeDiagram(text)) {
      continue;
    } else {
      preLines.push(rawLine);
    }
  }
  while (preLines.length > 0 && preLines[0].trim() === "") preLines.shift();
  while (preLines.length > 0 && preLines[preLines.length - 1].trim() === "") preLines.pop();
  return { preText: preLines.join("\n"), statusItems: items };
}

function PlainMarkdownBlock({ text }: { text: string }) {
  const { preText, statusItems } = splitPlainBlock(text);
  const asList = statusItems.length >= 2;
  return (
    <div className={`md-plain-block${asList ? " md-plain-block--split" : " md-plain-block--pre"}`}>
      <CodeViewer value={text} maxHeight={360} />
      {asList && preText && (
        <div className="md-plain-block__diagram">
          <CodeViewer value={preText} maxHeight={360} />
        </div>
      )}
      {asList && (
        <div className="md-status-list">
          {statusItems.map((item, index) => (
            <div className="md-status-list__item" key={`${index}-${item}`}>
              <span className="md-status-list__dot" aria-hidden="true" />
              <span className="md-status-list__text">{item}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function WorkspaceMarkdownImage({
  src,
  alt,
  title,
  owner,
}: {
  src?: string;
  alt?: string;
  title?: string;
  owner: WorkspaceFileOwner;
}) {
  const localPath = src ? localFilePathFromHref(src) : null;
  const [localSource, setLocalSource] = useState("");
  const [loadFailed, setLoadFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLocalSource("");
    setLoadFailed(false);
    if (!localPath) return () => { cancelled = true; };
    void readWorkspaceFileForOwner(owner, localPath).then((file) => {
      if (cancelled) return;
      if (file.err || file.kind !== "image" || !file.url) {
        setLoadFailed(true);
        return;
      }
      setLocalSource(file.url);
    }).catch(() => {
      if (!cancelled) setLoadFailed(true);
    });
    return () => { cancelled = true; };
  }, [localPath, owner]);

  if (localPath && !localSource) {
    return loadFailed ? <span className="md-image-fallback">{alt || localPath}</span> : null;
  }
  return <img src={localSource || src} alt={alt || ""} title={title} />;
}

function createComponents(plainStatusBlocks: boolean, onOpenLocalFailure: () => void, owner: WorkspaceFileOwner): Components {
  return {
    pre: ({ children }) => <>{children}</>,
    code: ({ className, children }) => {
      const text = String(children ?? "");
      const match = /language-([\w-]+)/.exec(className ?? "");
      const isBlock = match !== null || text.includes("\n");
      if (isBlock) {
        if (!match && plainStatusBlocks) return <PlainMarkdownBlock text={text.replace(/\n$/, "")} />;
        return <CodeViewer value={text.replace(/\n$/, "")} language={match?.[1]} maxHeight={360} />;
      }
      return <code className="md-code">{children}</code>;
    },
    img: ({ src, alt, title }) => (
      <WorkspaceMarkdownImage src={src} alt={alt} title={title} owner={owner} />
    ),
    a: ({ href, children }) => (
      <a
        href={href}
        onClick={(e) => {
          e.preventDefault();
          const localPath = href ? localFilePathFromHref(href) : null;
          if (localPath) {
            void openWorkspacePathForOwner(owner, localPath).catch(onOpenLocalFailure);
          } else if (href) {
            openExternal(href);
          }
        }}
        onAuxClick={(e) => {
          e.preventDefault();
          const localPath = href ? localFilePathFromHref(href) : null;
          if (localPath) {
            void openWorkspacePathForOwner(owner, localPath).catch(onOpenLocalFailure);
          } else if (href) {
            openExternal(href);
          }
        }}
        onMouseDown={(e) => {
          if (e.button === 1) e.preventDefault();
        }}
      >
        {children}
      </a>
    ),
  };
}

const MarkdownRenderer = memo(function MarkdownRenderer({
  text,
  plainStatusBlocks = false,
  tabId,
}: {
  text: string;
  plainStatusBlocks?: boolean;
  tabId?: string;
}) {
  const t = useT();
  const { showToast } = useToast();
  const workspaceFileScope = useWorkspaceFileScope();
  const workspaceFileOwner = useMemo(
    () => resolveWorkspaceFileOwner(tabId, workspaceFileScope),
    [tabId, workspaceFileScope.allowActiveWorkspace, workspaceFileScope.tabId, workspaceFileScope.workspaceRoot],
  );
  const containerRef = useRef<HTMLDivElement>(null);
  const mathContent = useMemo(() => normalizeMath(text), [text]);
  const handleOpenLocalFailure = useCallback(() => {
    showToast(t("msg.openFileFailed"), "warn");
  }, [showToast, t]);
  const components = useMemo(
    () => createComponents(plainStatusBlocks, handleOpenLocalFailure, workspaceFileOwner),
    [handleOpenLocalFailure, plainStatusBlocks, workspaceFileOwner],
  );
  return (
    <div className="md" ref={containerRef}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={components}
      >
        {mathContent}
      </ReactMarkdown>
    </div>
  );
});

export default MarkdownRenderer;
