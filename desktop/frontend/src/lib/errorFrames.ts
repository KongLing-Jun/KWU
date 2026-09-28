import type { ClientErrorFrame } from "./bridge";

const FRAME_LOCATION_RE = /:(\d+):(\d+)\)?\s*$/;
const MAX_FRAME_COORDINATE = 1_000_000_000;

function frameCategory(line: string): ClientErrorFrame["category"] {
  const normalized = line.toLowerCase();
  if (/(?:node_modules|react(?:-dom)?|scheduler|webpack|vite|wails|runtime(?:\.js)?)/.test(normalized)) {
    return /wails|runtime(?:\.js)?/.test(normalized) ? "runtime" : "framework";
  }
  if (/(?:^|[/(])(?:src|frontend|assets)(?:[/\\]|$)|\.(?:tsx?|jsx?)(?::|$)/.test(normalized)) {
    return "app";
  }
  return "unknown";
}

export function crashFrameMetadata(stack?: string): ClientErrorFrame[] | undefined {
  if (!stack) return undefined;
  const frames: ClientErrorFrame[] = [];
  for (const line of stack.split("\n")) {
    const trimmed = line.trim();
    if (!/^at\b/.test(trimmed) && !/^[^\s@]+@/.test(trimmed)) continue;
    const location = FRAME_LOCATION_RE.exec(trimmed);
    if (!location) continue;
    const lineNumber = Number(location[1]);
    const columnNumber = Number(location[2]);
    if (!Number.isSafeInteger(lineNumber) || !Number.isSafeInteger(columnNumber)) continue;
    if (lineNumber > MAX_FRAME_COORDINATE || columnNumber > MAX_FRAME_COORDINATE) continue;
    frames.push({ category: frameCategory(line), ordinal: frames.length, line: lineNumber, column: columnNumber });
    if (frames.length >= 64) break;
  }
  return frames.length ? frames : undefined;
}
