const RESPONSE_MARKER = "A2A response: ";
const MAX_DIAGNOSTIC_LENGTH = 256 * 1024;

function field(value: unknown, key: string): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)
    || !Object.prototype.hasOwnProperty.call(value, key)) return undefined;
  return (value as Record<string, unknown>)[key];
}

function nonemptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}

// Recognize only the host's single failure envelope. Never search history or
// partial output for an error, and leave every unsupported diagnostic visible.
export function taskFailureDiagnostic(error: string, turnStatus: string): string | undefined {
  if (turnStatus !== "failed" || error.length > MAX_DIAGNOSTIC_LENGTH) return undefined;
  const markerIndex = error.indexOf(RESPONSE_MARKER);
  if (markerIndex < 0 || (markerIndex !== 0 && error.slice(markerIndex - 2, markerIndex) !== "\n\n")
    || error.indexOf(RESPONSE_MARKER, markerIndex + RESPONSE_MARKER.length) !== -1) return undefined;
  let response: unknown;
  try {
    response = JSON.parse(error.slice(markerIndex + RESPONSE_MARKER.length));
  } catch {
    return undefined;
  }
  if (field(response, "jsonrpc") !== "2.0" || !nonemptyString(field(response, "id"))
    || field(response, "error") !== undefined) return undefined;
  const task = field(field(response, "result"), "task");
  const taskId = field(task, "id");
  const contextId = field(task, "contextId");
  const metadata = field(task, "metadata");
  if (!nonemptyString(taskId) || !nonemptyString(contextId)
    || field(metadata, "protocolVersion") !== "1.0"
    || field(metadata, "protocolBinding") !== "urn:reasonix:a2a:binding:in-process:v1") return undefined;
  const status = field(task, "status");
  if (field(status, "state") !== "TASK_STATE_FAILED" || !nonemptyString(field(status, "timestamp"))) return undefined;
  const message = field(status, "message");
  if (field(message, "role") !== "ROLE_AGENT" || !nonemptyString(field(message, "messageId"))
    || field(message, "taskId") !== taskId || field(message, "contextId") !== contextId) return undefined;
  const parts = field(message, "parts");
  if (!Array.isArray(parts) || parts.length !== 1) return undefined;
  const part: unknown = parts[0];
  const text = field(part, "text");
  if (!nonemptyString(text) || field(part, "mediaType") !== "text/plain"
    || Object.keys(part as object).some((key) => key !== "text" && key !== "mediaType")) return undefined;
  return text;
}
