import { useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import type { Todo } from "../lib/tools";
import { PromptBadge, PromptHeaderAction, PromptShelf } from "./PromptShelf";

const STORAGE_KEY = "todoPanel:open";

function loadOpenState(): boolean {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    return saved === null ? true : saved === "1";
  } catch {
    return true;
  }
}

function saveOpenState(open: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, open ? "1" : "0");
  } catch {
    /* ignore quota errors */
  }
}

// TodoPanel is the live task list pinned just above the composer — the kernel's
// latest todo_write call drives it, and it updates in place as the agent flips
// items to in_progress / completed. The collapsed/expanded state is persisted
// globally in localStorage so the user's choice sticks across sessions.
export function TodoPanel({
  todos,
  active,
  onDismiss,
}: {
  todos: Todo[];
  active: boolean;
  onDismiss: () => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(loadOpenState);
  const currentRef = useRef<HTMLLIElement | null>(null);

  const done = todos.filter((todo) => normalizeTodoStatus(todo.status, true) === "completed").length;
  const current = active ? todos.find((todo) => normalizeTodoStatus(todo.status, true) === "in_progress") : undefined;
  const unfinished = todos.find((todo) => normalizeTodoStatus(todo.status, true) !== "completed");
  const allDone = todos.length > 0 && done === todos.length;
  const summary = (active ? current?.activeForm : "") || current?.content || unfinished?.content || todos[todos.length - 1]?.content || "";
  const dismissible = !active || allDone;

  useEffect(() => {
    if (allDone) setOpen(false);
  }, [allDone]);

  useEffect(() => {
    if (!open) return;
    currentRef.current?.scrollIntoView({ block: "nearest" });
  }, [open, current?.content, current?.activeForm]);

  if (todos.length === 0) return null;

  return (
    <PromptShelf
      titleId="todo-shelf-title"
      title={t("todo.title")}
      badges={<PromptBadge>{done}/{todos.length}</PromptBadge>}
      meta={summary}
      role="region"
      headerActions={
        <>
          <PromptHeaderAction onClick={() => setOpen((value) => { const next = !value; saveOpenState(next); return next; })}>
            {open ? t("common.collapse") : t("common.expand")}
          </PromptHeaderAction>
          {dismissible ? (
            <PromptHeaderAction onClick={onDismiss}>
              {t("common.close")}
            </PromptHeaderAction>
          ) : null}
        </>
      }
    >
      {open && (
        <ul className="todobar__list">
          {todos.map((todo, index) => {
            const status = normalizeTodoStatus(todo.status, active);
            return (
              <li
                key={index}
                ref={status === "in_progress" ? currentRef : undefined}
                className={`todobar__item todobar__item--${status}${todo.level ? " todobar__item--sub" : ""}`}
              >
                <span className={`todobar__status todobar__status--${status}`}>
                  {t(todoStatusLabelKey(status))}
                </span>
                <span className="todobar__text">
                  {status === "in_progress" && todo.activeForm ? todo.activeForm : todo.content}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </PromptShelf>
  );
}

type TodoDisplayStatus = "pending" | "in_progress" | "unfinished" | "completed";

function normalizeTodoStatus(status: Todo["status"], active: boolean): TodoDisplayStatus {
  switch (String(status ?? "").trim()) {
    case "completed":
      return "completed";
    case "in_progress":
      return active ? "in_progress" : "unfinished";
    default:
      return "pending";
  }
}

function todoStatusLabelKey(status: TodoDisplayStatus): "todo.pending" | "todo.inProgress" | "todo.unfinished" | "todo.completed" {
  switch (status) {
    case "completed":
      return "todo.completed";
    case "in_progress":
      return "todo.inProgress";
    case "unfinished":
      return "todo.unfinished";
    default:
      return "todo.pending";
  }
}
