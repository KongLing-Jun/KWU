import { useId, useRef, type FormEvent } from "react";
import { useT } from "../../../lib/i18n";
import { ProjectModalSurface } from "./ProjectModalSurface";

interface ProjectJoinDialogProps {
  projectId: string;
  pending: boolean;
  error: string;
  invalid: boolean;
  returnFocus: HTMLElement | null;
  onProjectIdChange(value: string): void;
  onClose(): void;
  onSubmit(): void;
}

export function ProjectJoinDialog(props: ProjectJoinDialogProps) {
  const t = useT();
  const titleId = useId();
  const descriptionId = useId();
  const errorId = useId();
  const inputRef = useRef<HTMLInputElement>(null);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!props.pending) props.onSubmit();
  };

  return (
    <ProjectModalSurface
      backdropClassName="cloud-project-dialog__backdrop"
      className="cloud-project-dialog cloud-project-join-dialog"
      titleId={titleId}
      descriptionId={props.error ? `${descriptionId} ${errorId}` : descriptionId}
      pending={props.pending}
      closeLabel={t("common.close")}
      returnFocus={props.returnFocus}
      initialFocusRef={inputRef}
      onClose={props.onClose}
    >
      <header><h3 id={titleId}>{t("workspace.cloudProjects.dialog.join.title")}</h3></header>
      <form onSubmit={submit}>
        <div className="cloud-project-dialog__body">
          <p id={descriptionId} className="cloud-project-dialog__message">{t("workspace.cloudProjects.dialog.join.body")}</p>
          <section className="cloud-project-dialog__field">
            <label htmlFor={`${titleId}-project-id`}>{t("workspace.cloudProjects.dialog.join.projectId")}</label>
            <input
              ref={inputRef}
              id={`${titleId}-project-id`}
              value={props.projectId}
              placeholder={t("workspace.cloudProjects.dialog.join.placeholder")}
              autoComplete="off"
              spellCheck={false}
              aria-invalid={props.invalid || undefined}
              aria-describedby={props.error ? errorId : undefined}
              onChange={(event) => props.onProjectIdChange(event.currentTarget.value)}
            />
          </section>
          {props.error ? <p id={errorId} className="cloud-project-dialog__error" role="alert">{props.error}</p> : null}
        </div>
        <div className="cloud-project-dialog__actions">
          <button type="button" onClick={props.onClose} disabled={props.pending}>{t("common.cancel")}</button>
          <button type="submit" className="cloud-project-dialog__confirm" disabled={props.pending} aria-busy={props.pending || undefined}>
            {props.pending ? t("workspace.cloudProjects.pending") : t("workspace.cloudProjects.dialog.join.confirm")}
          </button>
        </div>
      </form>
    </ProjectModalSurface>
  );
}
