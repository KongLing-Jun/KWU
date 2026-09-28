import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";

interface MarkdownNode {
  type: string;
  value?: string;
  position?: {
    start: { offset?: number };
    end: { offset?: number };
  };
  children?: MarkdownNode[];
}

interface TextReplacement {
  start: number;
  end: number;
  value: string;
}

interface SourceRange {
  start: number;
  end: number;
}

const parser = unified().use(remarkParse).use(remarkGfm);
const protectedParents = new Set(["link", "linkReference", "image", "imageReference"]);
const voidHTMLTags = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"]);
const htmlTagStartRe = /<\s*(\/?)\s*([A-Za-z][A-Za-z0-9:-]*)\b/y;

function rawHTMLRanges(source: string): SourceRange[] {
  const ranges: SourceRange[] = [];
  const openTags = new Map<string, number[]>();
  for (let start = source.indexOf("<"); start >= 0;) {
    htmlTagStartRe.lastIndex = start;
    const tagMatch = htmlTagStartRe.exec(source);
    const special = source.startsWith("<!--", start) || source.startsWith("<![", start) || source.startsWith("<?", start) || source.startsWith("<!", start);
    if (!tagMatch && !special) {
      start = source.indexOf("<", start + 1);
      continue;
    }
    let quote = "";
    let end = start + 1;
    for (; end < source.length; end += 1) {
      const char = source[end];
      if (quote) {
        if (char === quote) quote = "";
      } else if (char === '"' || char === "'") {
        quote = char;
      } else if (char === ">") {
        end += 1;
        break;
      }
    }
    ranges.push({ start, end });
    if (tagMatch) {
      const closing = Boolean(tagMatch[1]);
      const tag = tagMatch[2].toLowerCase();
      const rawTag = source.slice(start, end);
      if (closing) {
        const starts = openTags.get(tag);
        const matchingStart = starts?.pop();
        if (matchingStart !== undefined) {
          ranges.push({ start: matchingStart, end });
        }
      } else if (!voidHTMLTags.has(tag) && !/\/\s*>$/.test(rawTag)) {
        const starts = openTags.get(tag) ?? [];
        starts.push(start);
        openTags.set(tag, starts);
      }
    }
    start = source.indexOf("<", Math.max(end, start + 1));
  }
  ranges.sort((a, b) => a.start - b.start || a.end - b.end);
  const merged: SourceRange[] = [];
  for (const range of ranges) {
    const previous = merged[merged.length - 1];
    if (!previous || range.start > previous.end) merged.push({ ...range });
    else previous.end = Math.max(previous.end, range.end);
  }
  return merged;
}

function replaceOutsideRanges(source: string, start: number, end: number, ranges: SourceRange[], replacePlainText: (plainText: string) => string): string {
  let low = 0;
  let high = ranges.length;
  while (low < high) {
    const mid = (low + high) >>> 1;
    if (ranges[mid].end <= start) low = mid + 1;
    else high = mid;
  }
  let cursor = start;
  let value = "";
  for (let index = low; index < ranges.length && ranges[index].start < end; index += 1) {
    const range = ranges[index];
    value += replacePlainText(source.slice(cursor, Math.max(cursor, range.start)));
    const protectedEnd = Math.min(end, range.end);
    value += source.slice(Math.max(cursor, range.start), protectedEnd);
    cursor = Math.max(cursor, protectedEnd);
  }
  return value + replacePlainText(source.slice(cursor, end));
}

function collectTextReplacements(
  node: MarkdownNode,
  source: string,
  protectedRanges: SourceRange[],
  replacePlainText: (plainText: string) => string,
  replacements: TextReplacement[],
  protectedContent = false,
): void {
  if (protectedContent || protectedParents.has(node.type)) return;
  if (node.type === "text") {
    const start = node.position?.start.offset;
    const end = node.position?.end.offset;
    if (start === undefined || end === undefined || end <= start) return;
    const value = replaceOutsideRanges(source, start, end, protectedRanges, replacePlainText);
    if (value !== source.slice(start, end)) replacements.push({ start, end, value });
    return;
  }
  if (!node.children) return;

  for (const child of node.children) {
    collectTextReplacements(child, source, protectedRanges, replacePlainText, replacements);
  }
}

export function replaceOutsideMarkdown(text: string, replacePlainText: (plainText: string) => string): string {
  let tree: MarkdownNode;
  try {
    tree = parser.parse(text) as MarkdownNode;
  } catch {
    return text;
  }
  const replacements: TextReplacement[] = [];
  collectTextReplacements(tree, text, rawHTMLRanges(text), replacePlainText, replacements);
  replacements.sort((a, b) => b.start - a.start);
  let result = text;
  for (const replacement of replacements) {
    result = result.slice(0, replacement.start) + replacement.value + result.slice(replacement.end);
  }
  return result;
}
