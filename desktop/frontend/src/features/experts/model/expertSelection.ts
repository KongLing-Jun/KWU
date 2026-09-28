import { expertAvatarSrcForInfo, expertMemberAvatarSrc } from "../../../lib/expertAssets";
import type { ExpertInfo, ExpertState, UserExpertInfo } from "../../../lib/types";
import type { ExpertSelection } from "../ui/ExpertDirectory";

export function expertSelectionFromState(
  state: ExpertState | null | undefined,
  experts: ExpertInfo[],
  userExperts: UserExpertInfo[] = [],
): ExpertSelection | null {
  if (!state?.selectedId) return null;
  const info = experts.find((item) => item.id === state.selectedId);
  const userInfo = userExperts.find((item) => item.id === state.selectedId);
  if (!info && !userInfo) {
    if (!state.selectedId.startsWith("project-expert-team-")) return null;
    return {
      id: state.selectedId,
      name: state.selectedName || "项目专家团",
      role: "项目专家协作",
      mode: "team",
      prompt: "",
    };
  }
  // User records carry uploaded team/member avatars that the flattened runtime
  // catalog may omit. Prefer that richer record when both describe the same id.
  const source = userInfo ?? info!;
  const mode = source.mode === "team" ? "team" : "expert";
  return {
    id: state.selectedId,
    name: state.selectedName || source.name || state.selectedId,
    role: info?.role || source.category || info?.categories?.[0] || "expert",
    mode,
    prompt: userInfo?.displayPrompt || info?.prompts?.[0] || "",
    systemPrompt: undefined,
    avatarSrc: expertAvatarSrcForInfo(source),
    members: source.members?.map((member) => ({
      kind: member.kind,
      expertId: member.expertId,
      name: member.name,
      role: member.role,
      task: member.task,
      output: member.output,
      tone: (member.tone as NonNullable<ExpertSelection["members"]>[number]["tone"]) || "stone",
      avatarKey: member.avatarKey,
      avatarSrc: expertMemberAvatarSrc(member),
    })),
  };
}

export function expertSelectionsFromRecentIds(
  recentIds: string[],
  experts: ExpertInfo[],
  userExperts: UserExpertInfo[] = [],
  limit = 5,
): ExpertSelection[] {
  const selections: ExpertSelection[] = [];
  const seen = new Set<string>();
  for (const value of recentIds) {
    const id = value.trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    const selection = expertSelectionFromState({
      selectedId: id,
      selectedName: "",
      source: "session",
      canSwitch: true,
      requiresNewSession: false,
    }, experts, userExperts);
    if (selection) selections.push(selection);
    if (selections.length >= limit) break;
  }
  return selections;
}
