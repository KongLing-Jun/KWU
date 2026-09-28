import { replaceOutsideMarkdown } from "./markdownProtection";

const attachmentRefRe = /@((?:\.clouvia|\.kwu|\.suiyuan|\.reasonix)\/attachments\/[^\s]+)/g;
const namedAttachmentRefRe = /(^|\s)@\[([^\]\r\n]+)\]\(([^)\s]+)\)/g;
const referenceRefRe = /(^|\s)@([^\s]+)/g;
const markdownLocalFileLinkRe = /(!?)\[([^\]\r\n]+)\]\((<[^>\r\n]+>|[^)\s]+)\)/g;
const inlineGeneratedFileRefRe = /`([^`\r\n]+)`/g;
const bareGeneratedFileCandidateRe = /[A-Za-z0-9_\-\p{Script=Han}.\/\\:]+(?:[ \t]+[A-Za-z0-9_\-\p{Script=Han}.\/\\:]+)*\.(?:html|htm|docx|pptx|xlsx|xlsm|csv|doc|pdf|ppt|tsv|xls|zip)(?![A-Za-z0-9])/giu;
const windowsDrivePathRe = /^[A-Za-z]:[\\/]/;
const genericSchemeRe = /^[A-Za-z][A-Za-z0-9+.-]*:/;
const trailingPunctuationRe = /[.,;!?)\]}，。；！？）】]+$/;
const generatedFileExts = new Set([
  ".csv",
  ".doc",
  ".docx",
  ".htm",
  ".html",
  ".pdf",
  ".ppt",
  ".pptx",
  ".tsv",
  ".xls",
  ".xlsm",
  ".xlsx",
  ".zip",
]);

export interface DisplayAttachment {
  path: string;
  name: string;
  kind: "image" | "file" | "folder";
  source: "attachment" | "workspace";
  ext: string;
  delivered?: true;
}

export function displayAttachmentPathKey(path: string): string {
  let normalized = path.trim().replace(/\\/g, "/");
  normalized = normalized.replace(/(^|\/)\.\//g, "$1").replace(/\/{2,}/g, "/");
  if (normalized.length > 1 && !/^[A-Za-z]:\/$/.test(normalized)) {
    normalized = normalized.replace(/\/+$/, "");
  }
  return windowsDrivePathRe.test(normalized) ? normalized.toLowerCase() : normalized;
}

function isAbsoluteDisplayPath(path: string): boolean {
  return path.startsWith("/") || windowsDrivePathRe.test(path);
}

export function displayAttachmentPathsEquivalent(left: string, right: string): boolean {
  const leftKey = displayAttachmentPathKey(left);
  const rightKey = displayAttachmentPathKey(right);
  if (!leftKey || !rightKey) return false;
  if (leftKey === rightKey) return true;
  if (isAbsoluteDisplayPath(leftKey) === isAbsoluteDisplayPath(rightKey)) return false;
  const absolute = isAbsoluteDisplayPath(leftKey) ? leftKey : rightKey;
  const relative = isAbsoluteDisplayPath(leftKey) ? rightKey : leftKey;
  const comparableRelative = windowsDrivePathRe.test(absolute) ? relative.toLowerCase() : relative;
  return absolute.endsWith(`/${comparableRelative}`);
}

function hasEquivalentDisplayPath(paths: Iterable<string>, path: string): boolean {
  for (const existing of paths) {
    if (displayAttachmentPathsEquivalent(existing, path)) return true;
  }
  return false;
}

function hasDisplayPathKey(paths: ReadonlySet<string>, path: string): boolean {
  const key = displayAttachmentPathKey(path);
  return Boolean(key) && paths.has(key);
}

function pushUniqueAttachment(attachments: DisplayAttachment[], seen: Set<string>, attachment: DisplayAttachment): void {
  const key = displayAttachmentPathKey(attachment.path);
  if (!key || hasEquivalentDisplayPath(seen, key)) return;
  seen.add(key);
  attachments.push(attachment);
}

function splitTrailingPunctuation(token: string): { core: string; suffix: string } {
  const m = token.match(trailingPunctuationRe);
  if (!m || m.index === undefined) return { core: token, suffix: "" };
  return { core: token.slice(0, m.index), suffix: m[0] };
}

export function baseName(path: string): string {
  const clean = path.replace(/[\\/]+$/, "");
  const slash = clean.lastIndexOf("/");
  const backslash = clean.lastIndexOf("\\");
  const idx = Math.max(slash, backslash);
  return idx >= 0 ? clean.slice(idx + 1) : clean;
}

function isImageAttachmentRef(path: string): boolean {
  const ext = attachmentExt(path);
  switch (ext) {
    case ".png":
    case ".jpg":
    case ".jpeg":
    case ".gif":
    case ".webp":
    case ".bmp":
    case ".svg":
    case ".tif":
    case ".tiff":
      return true;
    default:
      return false;
  }
}

function attachmentExt(path: string): string {
  const name = baseName(path).toLowerCase();
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot) : "";
}

function cleanDisplayName(name: string): string {
  return name.replace(/\\([\[\]])/g, "$1").replace(/\s+/g, " ").trim();
}

function cleanMarkdownHref(href: string): string {
  const trimmed = href.trim();
  if (trimmed.startsWith("<") && trimmed.endsWith(">")) return trimmed.slice(1, -1).trim();
  return trimmed;
}

function decodeLocalHrefPath(path: string): string {
  return path.replace(/(?:%[0-9A-Fa-f]{2})+/g, (encoded) => {
    try {
      return decodeURIComponent(encoded);
    } catch {
      return encoded;
    }
  });
}

export function localFilePathFromHref(href: string): string | null {
  const target = cleanMarkdownHref(href);
  if (!target || target.startsWith("#") || target.startsWith("//")) return null;
  const decodedTarget = decodeLocalHrefPath(target);
  if (windowsDrivePathRe.test(decodedTarget)) return decodedTarget;
  if (genericSchemeRe.test(target)) {
    if (!target.toLowerCase().startsWith("file:")) return null;
    try {
      const parsed = new URL(target);
      let path = decodeLocalHrefPath(parsed.pathname);
      if (/^\/[A-Za-z]:\//.test(path)) path = path.slice(1);
      return path || null;
    } catch {
      return null;
    }
  }
  if (decodedTarget.startsWith("/") || isDisplayReference(decodedTarget)) return decodedTarget;
  return null;
}

function generatedFilePathFromInlineCode(value: string): string | null {
  const target = value.trim();
  if (!target || target.startsWith("#") || target.startsWith("//")) return null;
  const path = localFilePathFromHref(target) ?? (genericSchemeRe.test(target) ? null : target);
  if (!path || !generatedFileExts.has(attachmentExt(path))) return null;
  return path;
}

export function inlineGeneratedFilePathsForDisplay(text: string): string[] {
  const paths: string[] = [];
  const seen = new Set<string>();
  for (const match of text.matchAll(inlineGeneratedFileRefRe)) {
    const path = generatedFilePathFromInlineCode(match[1] ?? "");
    if (!path || seen.has(path)) continue;
    seen.add(path);
    paths.push(path);
  }
  return paths;
}

export function bareGeneratedFilePathsForDisplay(text: string): string[] {
  const paths: string[] = [];
  const seen = new Set<string>();
  for (const match of text.matchAll(bareGeneratedFileCandidateRe)) {
    for (const path of generatedFileSuffixCandidates(match[0] ?? "")) {
      const key = displayAttachmentPathKey(path);
      if (!key || seen.has(key)) continue;
      seen.add(key);
      paths.push(path);
    }
  }
  return paths;
}

function generatedFileSuffixCandidates(candidate: string): string[] {
  const paths: string[] = [];
  let suffix = candidate.trim();
  while (suffix) {
    const path = generatedFilePathFromInlineCode(suffix);
    if (path) paths.push(path);
    const separator = suffix.search(/[ \t]+/);
    if (separator < 0) break;
    suffix = suffix.slice(separator).trimStart();
  }
  return paths;
}

export function linkVerifiedBareGeneratedFilesForDisplay(text: string, verifiedPathKeys: ReadonlySet<string>): string {
  if (verifiedPathKeys.size === 0) return text;
  const linkedPathKeys = new Set<string>();
  const linkBarePaths = (plainText: string) => plainText.replace(bareGeneratedFileCandidateRe, (candidate, offset: number) => {
    if (offset > 0 && plainText[offset - 1] === "@") return candidate;
    for (const path of generatedFileSuffixCandidates(candidate)) {
      const key = displayAttachmentPathKey(path);
      if (!key || !hasDisplayPathKey(verifiedPathKeys, path)) continue;
      const prefix = candidate.slice(0, candidate.length - path.length);
      if (hasEquivalentDisplayPath(linkedPathKeys, path)) return prefix;
      linkedPathKeys.add(key);
      return `${prefix}[${path}](<${path}>)`;
    }
    return candidate;
  });

  return replaceOutsideMarkdown(text, linkBarePaths);
}

export function replaceAttachmentRefsForDisplay(text: string): string {
  return text
    .replace(namedAttachmentRefRe, (_full, lead: string, label: string, token: string) => {
      const { core, suffix } = splitTrailingPunctuation(token);
      if (!core || !isDisplayReference(core)) return _full;
      const name = cleanDisplayName(label) || baseName(core) || "attachment";
      return `${lead}${isImageAttachmentRef(core) ? "[image]" : `[file:${name}]`}${suffix}`;
    })
    .replace(attachmentRefRe, (_full, token: string) => {
      const { core, suffix } = splitTrailingPunctuation(token);
      if (!core) return _full;
      if (isImageAttachmentRef(core)) return `[image]${suffix}`;
      const name = baseName(core) || "attachment";
      return `[file:${name}]${suffix}`;
    });
}

export function restoreAttachmentRefsForSubmit(text: string): string {
  return text.replace(namedAttachmentRefRe, (_full, lead: string, _label: string, token: string) => {
    const { core, suffix } = splitTrailingPunctuation(token);
    if (!core || !isDisplayReference(core)) return _full;
    return `${lead}@${core}${suffix}`;
  });
}

function displayRefName(name: string): string {
  return name.replace(/[\[\]\(\)\r\n]+/g, " ").replace(/\s+/g, " ").trim() || "attachment";
}

export function formatAttachmentRefForDisplay(attachment: Pick<DisplayAttachment, "path" | "name" | "source">): string {
  if (attachment.source === "attachment") {
    return `@[${displayRefName(attachment.name || baseName(attachment.path) || "attachment")}](${attachment.path})`;
  }
  return `@${attachment.path}`;
}

export function formatAttachmentRefForSubmit(attachment: Pick<DisplayAttachment, "path">): string {
  return `@${attachment.path}`;
}

export function parseAttachmentRefsForDisplay(
  text: string,
  options: { acceptedPathKeys?: ReadonlySet<string>; exactAcceptedPaths?: boolean } = {},
): { text: string; attachments: DisplayAttachment[] } {
  const attachments: DisplayAttachment[] = [];
  const seen = new Set<string>();
  const acceptedPathKeys = options.acceptedPathKeys;
  const cleaned = text
    .replace(namedAttachmentRefRe, (_full, lead: string, label: string, token: string) => {
      const { core, suffix } = splitTrailingPunctuation(token);
      if (!core || !isDisplayReference(core)) return _full;
      const name = cleanDisplayName(label) || baseName(core) || "attachment";
      if (acceptedPathKeys && !(options.exactAcceptedPaths ? hasDisplayPathKey(acceptedPathKeys, core) : hasEquivalentDisplayPath(acceptedPathKeys, core))) return `${lead}${name}${suffix}`;
      pushUniqueAttachment(attachments, seen, displayAttachment(core, name));
      return lead + suffix;
    })
    .replace(referenceRefRe, (_full, lead: string, token: string) => {
      const { core, suffix } = splitTrailingPunctuation(token);
      if (!core || !isDisplayReference(core)) return _full;
      const name = baseName(core) || "attachment";
      if (acceptedPathKeys && !(options.exactAcceptedPaths ? hasDisplayPathKey(acceptedPathKeys, core) : hasEquivalentDisplayPath(acceptedPathKeys, core))) return `${lead}${core}${suffix}`;
      const attachment = displayAttachment(core, name);
      pushUniqueAttachment(attachments, seen, attachment);
      return lead + suffix;
    })
    .replace(/[ \t]+([.,;!?)\]}，。；！？）】])/g, "$1")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
  return { text: cleaned, attachments };
}

export function parseGeneratedFileRefsForDisplay(
  text: string,
  options: { verifiedGeneratedPathKeys?: ReadonlySet<string> } = {},
): { text: string; attachments: DisplayAttachment[] } {
  const verifiedGeneratedPathKeys = options.verifiedGeneratedPathKeys;
  const parsed = parseAttachmentRefsForDisplay(text, { acceptedPathKeys: verifiedGeneratedPathKeys, exactAcceptedPaths: true });
  const attachments = [...parsed.attachments];
  const seen = new Set(attachments.map((attachment) => displayAttachmentPathKey(attachment.path)));
  for (const match of text.matchAll(markdownLocalFileLinkRe)) {
    if (match[1]) continue;
    const label = cleanDisplayName(match[2] ?? "");
    const path = localFilePathFromHref(match[3] ?? "");
    const key = path ? displayAttachmentPathKey(path) : "";
    if (!path || !key || hasEquivalentDisplayPath(seen, path)) continue;
    if (verifiedGeneratedPathKeys && !hasDisplayPathKey(verifiedGeneratedPathKeys, path)) continue;
    seen.add(key);
    attachments.push(displayAttachment(path, label || baseName(path) || "attachment"));
  }
  const cleaned = parsed.text
    .replace(markdownLocalFileLinkRe, (full, imageMarker: string, label: string, href: string) => {
      if (imageMarker || !verifiedGeneratedPathKeys) return full;
      const path = localFilePathFromHref(href);
      if (!path || hasDisplayPathKey(verifiedGeneratedPathKeys, path)) return full;
      return cleanDisplayName(label) || path;
    })
    .replace(inlineGeneratedFileRefRe, (full, value: string) => {
      const path = generatedFilePathFromInlineCode(value);
      if (!path || (verifiedGeneratedPathKeys && !hasDisplayPathKey(verifiedGeneratedPathKeys, path))) return full;
      const key = displayAttachmentPathKey(path);
      if (key && !hasEquivalentDisplayPath(seen, path)) {
        seen.add(key);
        attachments.push(displayAttachment(path, baseName(path) || "attachment"));
      }
      return "";
    })
    .replace(/[ \t]+([.,;!?)\]}，。；！？）】])/g, "$1")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
  return { text: cleaned, attachments };
}

export function sortDisplayAttachments<T extends { kind: "image" | "file" | "folder" }>(attachments: T[]): T[] {
  return [...attachments].sort((a, b) => {
    if (a.kind === b.kind) return 0;
    if (a.kind === "image") return -1;
    if (b.kind === "image") return 1;
    return 0;
  });
}

function isDisplayReference(path: string): boolean {
  if (isStoredAttachmentPath(path)) return true;
  if (path.endsWith("/")) return true;
  if (path.includes("/")) return true;
  return attachmentExt(path) !== "";
}

function displayAttachment(path: string, name: string): DisplayAttachment {
  if (isStoredAttachmentPath(path)) {
    const kind = isImageAttachmentRef(path) ? "image" : "file";
    return {
      path,
      name,
      kind,
      source: "attachment",
      ext: attachmentExt(path).replace(/^\./, "").toUpperCase(),
    };
  }
  const isDir = path.endsWith("/");
  return {
    path,
    name,
    kind: isDir ? "folder" : "file",
    source: "workspace",
    ext: isDir ? "" : attachmentExt(path).replace(/^\./, "").toUpperCase(),
  };
}

function isStoredAttachmentPath(path: string): boolean {
  return /^(?:\.clouvia|\.kwu|\.suiyuan|\.reasonix)\/attachments\//.test(path);
}
