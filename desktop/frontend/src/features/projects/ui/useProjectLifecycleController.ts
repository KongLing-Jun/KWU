import { useState } from "react";
import type { CloudProject } from "../model/projectTypes";
import type { ProjectTemplateId } from "../model/projectTemplateInstructions";
import type { ProjectDialogKind } from "./ProjectActionDialog";

export function useProjectLifecycleController() {
  const [dialog, setDialog] = useState<ProjectDialogKind | null>(null);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const [draftName, setDraftName] = useState("");
  const [draftDescription, setDraftDescription] = useState("");
  const [templateId, setTemplateId] = useState<ProjectTemplateId | "">("");
  const [templateInstruction, setTemplateInstruction] = useState("");
  const [dialogError, setDialogError] = useState("");
  const [dialogNameInvalid, setDialogNameInvalid] = useState(false);
  const [pending, setPending] = useState(false);
  const [liveMessage, setLiveMessage] = useState("");
  const [actionProject, setActionProject] = useState<CloudProject | null>(null);

  return {
    dialog,
    setDialog,
    returnFocus,
    setReturnFocus,
    draftName,
    setDraftName,
    draftDescription,
    setDraftDescription,
    templateId,
    setTemplateId,
    templateInstruction,
    setTemplateInstruction,
    dialogError,
    setDialogError,
    dialogNameInvalid,
    setDialogNameInvalid,
    pending,
    setPending,
    liveMessage,
    setLiveMessage,
    actionProject,
    setActionProject,
  };
}
