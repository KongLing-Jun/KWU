import { useRef, useState } from "react";
import type { CloudProjectCapabilityBinding, CloudProjectInstruction, CloudProjectMember } from "../model/projectTypes";

type InstructionAvailability = "idle" | "loading" | "ready" | "unavailable" | "error";

export function useProjectContextController() {
  const instructionRequestRef = useRef(0);
  const [capabilityBindings, setCapabilityBindings] = useState<CloudProjectCapabilityBinding[]>([]);
  const [instructionStatus, setInstructionStatus] = useState<InstructionAvailability>("idle");
  const [instruction, setInstruction] = useState<CloudProjectInstruction | null>(null);
  const [instructionHumanDraft, setInstructionHumanDraft] = useState("");
  const [instructionAgentDraft, setInstructionAgentDraft] = useState("");
  const [instructionEditing, setInstructionEditing] = useState(false);
  const [instructionPending, setInstructionPending] = useState(false);
  const [instructionError, setInstructionError] = useState("");
  const [instructionNotice, setInstructionNotice] = useState("");
  const [workItemsAvailable, setWorkItemsAvailable] = useState(false);
  const [instructionsAvailable, setInstructionsAvailable] = useState(false);
  const [membersAvailable, setMembersAvailable] = useState(false);
  const [activityAvailable, setActivityAvailable] = useState(false);
  const [assetsAvailable, setAssetsAvailable] = useState(false);
  const [projectMembers, setProjectMembers] = useState<CloudProjectMember[]>([]);
  const [currentRole, setCurrentRole] = useState<"owner" | "member" | null>(null);

  return {
    instructionRequestRef,
    capabilityBindings,
    setCapabilityBindings,
    instructionStatus,
    setInstructionStatus,
    instruction,
    setInstruction,
    instructionHumanDraft,
    setInstructionHumanDraft,
    instructionAgentDraft,
    setInstructionAgentDraft,
    instructionEditing,
    setInstructionEditing,
    instructionPending,
    setInstructionPending,
    instructionError,
    setInstructionError,
    instructionNotice,
    setInstructionNotice,
    workItemsAvailable,
    setWorkItemsAvailable,
    instructionsAvailable,
    setInstructionsAvailable,
    membersAvailable,
    setMembersAvailable,
    activityAvailable,
    setActivityAvailable,
    assetsAvailable,
    setAssetsAvailable,
    projectMembers,
    setProjectMembers,
    currentRole,
    setCurrentRole,
  };
}
