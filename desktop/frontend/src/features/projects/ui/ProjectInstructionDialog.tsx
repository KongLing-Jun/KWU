import { useId, useRef } from "react";
import { InlineConfirmButton } from "../../../components/InlineConfirmButton";
import { ProjectModalSurface } from "./ProjectModalSurface";

interface Props {
  humanDescription: string;
  agentInstruction: string;
  pending: boolean;
  error: string;
  readOnly: boolean;
  returnFocus: HTMLElement | null;
  onAgentInstructionChange(value: string): void;
  onSave(): void;
  onClear(): void;
  onClose(): void;
}

export function ProjectInstructionDialog(props: Props) {
  const titleId = useId();
  const editorRef = useRef<HTMLTextAreaElement>(null);

  return <ProjectModalSurface
    backdropClassName="cloud-project-dialog__backdrop"
    className="cloud-project-dialog cloud-project-instruction-dialog"
    titleId={titleId}
    pending={props.pending}
    showClose
    returnFocus={props.returnFocus}
    initialFocusRef={editorRef}
    onClose={props.onClose}
  >
    <header><h3 id={titleId}>项目指令</h3></header>
    <form onSubmit={(event) => { event.preventDefault(); props.onSave(); }}>
      <div className="cloud-project-dialog__body">
        {props.humanDescription.trim() ? <p className="cloud-project-instruction-dialog__context" aria-label="项目说明"><strong>项目说明</strong>{props.humanDescription}</p> : null}
        <section className="cloud-project-dialog__field cloud-project-dialog__field--instruction">
          <div className="cloud-project-dialog__field-heading">
            <label htmlFor={`${titleId}-agent`}>项目指令</label>
            <span aria-live="polite">{Array.from(props.agentInstruction).length.toLocaleString()} / 10,000</span>
          </div>
          <textarea ref={editorRef} id={`${titleId}-agent`} className="cloud-project-dialog__instruction" value={props.agentInstruction} readOnly={props.readOnly} maxLength={10000} onChange={(event) => props.onAgentInstructionChange(event.currentTarget.value)} placeholder="提供项目背景、执行规范、阶段和输出要求" />
        </section>
        {props.error ? <p role="alert" className="cloud-project-dialog__error">{props.error}</p> : null}
      </div>
      <footer className="cloud-project-dialog__actions cloud-project-instruction-dialog__actions">
        <InlineConfirmButton label="清空项目指令" confirmLabel="确认清空项目指令" cancelLabel="取消" danger disabled={props.pending || props.readOnly} onConfirm={props.onClear} />
        <button type="button" onClick={props.onClose} disabled={props.pending}>取消</button>
        <button type="submit" className="cloud-project-dialog__confirm" disabled={props.pending || props.readOnly}>{props.pending ? "保存中…" : "保存项目指令"}</button>
      </footer>
    </form>
  </ProjectModalSurface>;
}
