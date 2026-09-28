import { displayAttachmentPathsEquivalent } from "./attachmentDisplay";
import type { ArtifactDelivery } from "./types";

export const ARTIFACT_DELIVERY_VERSION = 1;
export const MAX_ARTIFACT_DELIVERY_PATHS = 32;

function isCanonicalArtifactPath(value: string): boolean {
  if (value === "" || value !== value.trim() || value.includes("\\") || value.includes("\0")) return false;
  if (value.startsWith("/") || value.startsWith("./") || value.startsWith("../") || /^[a-zA-Z]:/.test(value)) return false;
  const parts = value.split("/");
  return parts.every((part) => part !== "" && part !== "." && part !== "..");
}

export function normalizeArtifactDeliveryV1(value: unknown): ArtifactDelivery | undefined {
  if (!value || typeof value !== "object") return undefined;
  const candidate = value as Partial<ArtifactDelivery>;
  if (candidate.version !== ARTIFACT_DELIVERY_VERSION || !Array.isArray(candidate.paths)) return undefined;
  if (candidate.paths.length > MAX_ARTIFACT_DELIVERY_PATHS) return undefined;
  if (
    candidate.deliveredAt !== undefined
    && (typeof candidate.deliveredAt !== "number" || !Number.isFinite(candidate.deliveredAt) || candidate.deliveredAt <= 0)
  ) return undefined;

  const paths: string[] = [];
  for (const rawPath of candidate.paths) {
    if (typeof rawPath !== "string") return undefined;
    if (!isCanonicalArtifactPath(rawPath)) return undefined;
    if (paths.some((existing) => displayAttachmentPathsEquivalent(existing, rawPath))) return undefined;
    paths.push(rawPath);
  }

  return {
    version: ARTIFACT_DELIVERY_VERSION,
    paths,
    ...(candidate.deliveredAt !== undefined ? { deliveredAt: candidate.deliveredAt } : {}),
  };
}
