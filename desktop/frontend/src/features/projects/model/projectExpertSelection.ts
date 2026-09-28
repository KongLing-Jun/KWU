import type { ExpertInfo, UserExpertInfo } from "../../../lib/types";
import { expertSelectionFromState } from "../../experts/model/expertSelection";
import type { ExpertSelection } from "../../experts/ui/ExpertDirectory";
import type { CloudProjectCapabilityBinding } from "./projectTypes";

export function projectExpertTeamId(ids: readonly string[]): string {
  let hash = 14695981039346656037n;
  const bytes = new TextEncoder().encode(ids.join("\0"));
  for (const byte of bytes) {
    hash ^= BigInt(byte);
    hash = BigInt.asUintN(64, hash * 1099511628211n);
  }
  return `project-expert-team-${hash.toString(16).padStart(16, "0")}`;
}

export function projectExpertSelections(
  bindings: readonly CloudProjectCapabilityBinding[],
  experts: readonly ExpertInfo[],
  userExperts: readonly UserExpertInfo[] = [],
): ExpertSelection[] {
  const ids = [...bindings]
    .filter((binding) => binding.kind === "expert" && !binding.publicDisabled)
    .sort((left, right) => left.position - right.position)
    .map((binding) => binding.registryRef.trim())
    .filter((id, index, values) => Boolean(id) && values.indexOf(id) === index);
  return ids
    .map((id) => expertSelectionFromState({
      selectedId: id,
      selectedName: "",
      source: "project",
      canSwitch: true,
      requiresNewSession: false,
    }, [...experts], [...userExperts]))
    .filter((selection): selection is ExpertSelection => Boolean(selection));
}
