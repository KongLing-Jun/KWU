import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
} from "react";
import type {
  ClipboardEvent,
  CSSProperties,
  FormEvent,
  KeyboardEvent,
  MouseEvent,
} from "react";
import {
  formatSkillToken,
  parseSkillTokenParts,
  parseSkillTokens,
  removeSkillToken as removeSkillTokenFromText,
} from "../../../lib/skillTokens";

type SelectionOffsets = { start: number; end: number };

export type SkillTokenEditorHandle = {
  element: HTMLDivElement | null;
  focus: () => void;
  value: () => string;
  selection: () => SelectionOffsets;
  clear: () => void;
  setSelectionRange: (start: number, end?: number) => void;
  replaceSelection: (value: string) => void;
};

type SkillTokenEditorProps = {
  id?: string;
  value: string;
  skillNames: readonly string[];
  skillLabels?: Readonly<Record<string, string>>;
  className?: string;
  style?: CSSProperties;
  placeholder?: string;
  ariaLabel: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  onSelectionChange?: (selection: SelectionOffsets) => void;
  onPaste?: (event: ClipboardEvent<HTMLDivElement>) => void;
  onKeyDown?: (event: KeyboardEvent<HTMLDivElement>) => void;
  onCompositionStart?: () => void;
  onCompositionEnd?: () => void;
};

const SKILL_NODE_ATTR = "data-skill-token";
const SKILL_REMOVE_ATTR = "data-skill-remove";

function skillNode(name: string, displayLabel = name): HTMLSpanElement {
  const node = document.createElement("span");
  node.className = "composer-skill-token";
  node.setAttribute(SKILL_NODE_ATTR, name);
  node.setAttribute("contenteditable", "false");
  node.setAttribute("aria-label", `技能 ${displayLabel}`);
  node.title = name;

  const icon = document.createElement("span");
  icon.className = "composer-skill-token__icon";
  icon.setAttribute("aria-hidden", "true");
  icon.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-hammer"><path d="m15 12-9.373 9.373a1 1 0 0 1-3.001-3L12 9"></path><path d="m18 15 4-4"></path><path d="m21.5 11.5-1.914-1.914A2 2 0 0 1 19 8.172v-.344a2 2 0 0 0-.586-1.414l-1.657-1.657A6 6 0 0 0 12.516 3H9l1.243 1.243A6 6 0 0 1 12 8.485V10l2 2h1.172a2 2 0 0 1 1.414.586L18.5 14.5"></path></svg>';

  const label = document.createElement("span");
  label.className = "composer-skill-token__label";
  label.textContent = displayLabel;

  const remove = document.createElement("button");
  remove.type = "button";
  remove.tabIndex = -1;
  remove.className = "composer-skill-token__remove";
  remove.setAttribute(SKILL_REMOVE_ATTR, "");
  remove.setAttribute("aria-label", `移除技能 ${displayLabel}`);
  remove.title = `移除 ${displayLabel}`;
  remove.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" aria-hidden="true"><path d="M18 6 6 18"></path><path d="m6 6 12 12"></path></svg>';

  node.append(icon, label, remove);
  return node;
}

function appendText(fragment: DocumentFragment, value: string): void {
  const lines = value.split("\n");
  lines.forEach((line, index) => {
    if (line) fragment.append(document.createTextNode(line));
    if (index < lines.length - 1) fragment.append(document.createElement("br"));
  });
}

function fragmentForValue(
  value: string,
  skillNames: readonly string[],
  skillLabels: Readonly<Record<string, string>> = {},
): DocumentFragment {
  const fragment = document.createDocumentFragment();
  let previousWasSkill = false;
  for (const part of parseSkillTokenParts(value, skillNames)) {
    if (part.kind === "skill") {
      fragment.append(skillNode(part.name, skillLabels[part.name] || part.name));
      previousWasSkill = true;
      continue;
    }
    // A regular trailing space after a contenteditable=false chip can be
    // collapsed by WebView selection, which makes the caret appear before
    // the chip. Keep the boundary node visible to the browser while
    // nodeValue() normalizes NBSP back to a regular model space.
    const text = previousWasSkill && part.text.startsWith(" ")
      ? `${part.text}\u200B`
      : part.text;
    appendText(fragment, text);
    previousWasSkill = false;
  }
  return fragment;
}

function nodeValue(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) {
    return (node.nodeValue ?? "").replace(/\u00a0/g, " ").replace(/\u200B/g, "");
  }
  if (!(node instanceof HTMLElement)) return "";
  const skillName = node.getAttribute(SKILL_NODE_ATTR);
  if (skillName) return formatSkillToken(skillName);
  if (node.tagName === "BR") return "\n";
  const body = Array.from(node.childNodes).map(nodeValue).join("");
  return node.tagName === "DIV" || node.tagName === "P" ? `${body}\n` : body;
}

type EditorSnapshot = {
  value: string;
  insertedBoundaryOffsets: number[];
};

function editorSnapshot(root: HTMLElement): EditorSnapshot {
  const children = Array.from(root.childNodes);
  const values = children.map(nodeValue);
  const insertedBoundaryOffsets: number[] = [];
  let rawOffset = 0;
  let value = "";
  for (let index = 0; index < children.length; index += 1) {
    const child = children[index];
    const childValue = values[index];
    const skill = child instanceof HTMLElement && child.hasAttribute(SKILL_NODE_ATTR);
    if (skill && value && !/\s$/.test(value)) {
      insertedBoundaryOffsets.push(rawOffset);
      value += " ";
    }
    value += childValue;
    rawOffset += childValue.length;
    const nextValue = values[index + 1] ?? "";
    if (skill && nextValue && !/^\s/.test(nextValue)) {
      insertedBoundaryOffsets.push(rawOffset);
      value += " ";
    }
  }
  return {
    value: value.replace(/\n$/, ""),
    insertedBoundaryOffsets,
  };
}

function editorValue(root: HTMLElement): string {
  return editorSnapshot(root).value;
}

function selectionAfterInsertedBoundaries(
  selection: SelectionOffsets,
  insertedBoundaryOffsets: readonly number[],
  max: number,
): SelectionOffsets {
  const adjust = (offset: number) =>
    Math.min(max, offset + insertedBoundaryOffsets.filter((boundary) => boundary <= offset).length);
  return {
    start: adjust(selection.start),
    end: adjust(selection.end),
  };
}

function containsNode(root: Node, node: Node | null): boolean {
  return !!node && (node === root || root.contains(node));
}

function lengthBeforePoint(root: HTMLElement, container: Node, offset: number): number {
  const range = document.createRange();
  range.setStart(root, 0);
  range.setEnd(container, offset);
  return Array.from(range.cloneContents().childNodes).map(nodeValue).join("").length;
}

function pointForOffset(root: HTMLElement, requested: number): { node: Node; offset: number } {
  let remaining = Math.max(0, requested);
  const children = Array.from(root.childNodes);
  for (let index = 0; index < children.length; index += 1) {
    const child = children[index];
    const value = nodeValue(child);
    if (remaining > value.length) {
      remaining -= value.length;
      continue;
    }
    if (child.nodeType === Node.TEXT_NODE) {
      const rawLength = child.nodeValue?.length ?? 0;
      const normalizedLength = nodeValue(child).length;
      const offset = remaining >= normalizedLength && rawLength > normalizedLength
        ? rawLength
        : Math.min(remaining, rawLength);
      return { node: child, offset };
    }
    if (child instanceof HTMLElement && child.hasAttribute(SKILL_NODE_ATTR)) {
      return remaining < value.length / 2
        ? { node: root, offset: index }
        : { node: root, offset: index + 1 };
    }
    if (child instanceof HTMLElement && child.tagName === "BR") {
      return { node: root, offset: remaining === 0 ? index : index + 1 };
    }
    const nested = pointForOffset(child as HTMLElement, remaining);
    return nested;
  }
  return { node: root, offset: children.length };
}

function currentSelection(root: HTMLElement, fallback: SelectionOffsets): SelectionOffsets {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) return fallback;
  const range = selection.getRangeAt(0);
  if (!containsNode(root, range.startContainer) || !containsNode(root, range.endContainer)) return fallback;
  return {
    start: lengthBeforePoint(root, range.startContainer, range.startOffset),
    end: lengthBeforePoint(root, range.endContainer, range.endOffset),
  };
}

function applySelection(root: HTMLElement, start: number, end = start): void {
  const selection = window.getSelection();
  if (!selection) return;
  const range = document.createRange();
  const startPoint = pointForOffset(root, start);
  const endPoint = pointForOffset(root, end);
  range.setStart(startPoint.node, startPoint.offset);
  range.setEnd(endPoint.node, endPoint.offset);
  selection.removeAllRanges();
  selection.addRange(range);
}

function normalizeTextInsertion(value: string): string {
  return value.replace(/\r\n?/g, "\n");
}

function adjacentSkillRange(
  value: string,
  skillNames: readonly string[],
  caret: number,
  direction: "backward" | "forward",
): SelectionOffsets | null {
  let offset = 0;
  for (const part of parseSkillTokenParts(value, skillNames)) {
    const length = part.kind === "skill" ? part.raw.length : part.text.length;
    const start = offset;
    const end = offset + length;
    if (
      part.kind === "skill"
      && ((direction === "backward" && caret === end) || (direction === "forward" && caret === start))
    ) {
      return { start, end };
    }
    offset = end;
  }
  return null;
}

export const SkillTokenEditor = forwardRef<SkillTokenEditorHandle, SkillTokenEditorProps>(
  function SkillTokenEditor(
    {
      id,
      value,
      skillNames,
      skillLabels = {},
      className,
      style,
      placeholder,
      ariaLabel,
      disabled = false,
      onChange,
      onSelectionChange,
      onPaste,
      onKeyDown,
      onCompositionStart,
      onCompositionEnd,
    },
    ref,
  ) {
    const rootRef = useRef<HTMLDivElement>(null);
    const selectionRef = useRef<SelectionOffsets>({ start: value.length, end: value.length });

    const rememberSelection = () => {
      const root = rootRef.current;
      if (!root) return;
      const next = currentSelection(root, selectionRef.current);
      selectionRef.current = next;
      onSelectionChange?.(next);
    };

    const replaceSelection = (insertedValue: string) => {
      const root = rootRef.current;
      if (!root) return;
      const selection = currentSelection(root, selectionRef.current);
      const nextValue = normalizeTextInsertion(insertedValue);
      const next = value.slice(0, selection.start) + nextValue + value.slice(selection.end);
      const caret = selection.start + nextValue.length;
      selectionRef.current = { start: caret, end: caret };
      root.replaceChildren(fragmentForValue(next, skillNames, skillLabels));
      root.focus();
      applySelection(root, caret);
      onChange(next);
      onSelectionChange?.(selectionRef.current);
    };

    useImperativeHandle(ref, () => ({
      get element() {
        return rootRef.current;
      },
      focus() {
        rootRef.current?.focus();
      },
      value() {
        return rootRef.current ? editorValue(rootRef.current) : value;
      },
      selection() {
        return rootRef.current
          ? currentSelection(rootRef.current, selectionRef.current)
          : selectionRef.current;
      },
      clear() {
        selectionRef.current = { start: 0, end: 0 };
        rootRef.current?.replaceChildren();
        onSelectionChange?.(selectionRef.current);
      },
      setSelectionRange(start: number, end = start) {
        const root = rootRef.current;
        selectionRef.current = { start, end };
        if (!root) return;
        root.focus();
        applySelection(root, start, end);
        onSelectionChange?.(selectionRef.current);
      },
      replaceSelection,
    }), [onChange, onSelectionChange, skillLabels, skillNames, value]);

    useLayoutEffect(() => {
      const root = rootRef.current;
      if (!root) return;
      const expectedSkills = parseSkillTokens(value, skillNames).skillNames.join("\n");
      const renderedSkills = Array.from(root.querySelectorAll(`[${SKILL_NODE_ATTR}]`))
        .map((node) => node.getAttribute(SKILL_NODE_ATTR) ?? "")
        .join("\n");
      const expectedLabels = parseSkillTokens(value, skillNames).skillNames
        .map((name) => skillLabels[name] || name)
        .join("\n");
      const renderedLabels = Array.from(root.querySelectorAll(".composer-skill-token__label"))
        .map((node) => node.textContent ?? "")
        .join("\n");
      if (editorValue(root) === value && renderedSkills === expectedSkills && renderedLabels === expectedLabels) return;
      const active = document.activeElement === root;
      const selection = active ? currentSelection(root, selectionRef.current) : selectionRef.current;
      root.replaceChildren(fragmentForValue(value, skillNames, skillLabels));
      const max = value.length;
      selectionRef.current = {
        start: Math.min(selection.start, max),
        end: Math.min(selection.end, max),
      };
      if (active) applySelection(root, selectionRef.current.start, selectionRef.current.end);
    }, [skillLabels, skillNames, value]);

    useEffect(() => {
      const update = () => {
        if (document.activeElement === rootRef.current) rememberSelection();
      };
      document.addEventListener("selectionchange", update);
      return () => document.removeEventListener("selectionchange", update);
    });

    const onInput = (_event: FormEvent<HTMLDivElement>) => {
      const root = rootRef.current;
      if (!root) return;
      const rawSelection = currentSelection(root, selectionRef.current);
      const snapshot = editorSnapshot(root);
      const nextSelection = selectionAfterInsertedBoundaries(
        rawSelection,
        snapshot.insertedBoundaryOffsets,
        snapshot.value.length,
      );
      selectionRef.current = nextSelection;
      if (snapshot.insertedBoundaryOffsets.length > 0) {
        root.replaceChildren(fragmentForValue(snapshot.value, skillNames, skillLabels));
        applySelection(root, nextSelection.start, nextSelection.end);
      }
      onSelectionChange?.(nextSelection);
      onChange(snapshot.value);
    };

    const copySelection = (event: ClipboardEvent<HTMLDivElement>, cut: boolean) => {
      const root = rootRef.current;
      if (!root) return;
      const selection = currentSelection(root, selectionRef.current);
      if (selection.start === selection.end) return;
      event.preventDefault();
      event.clipboardData.setData("text/plain", value.slice(selection.start, selection.end));
      if (cut && !disabled) replaceSelection("");
    };

    const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
      if (!disabled && (event.key === "Backspace" || event.key === "Delete")) {
        const root = rootRef.current;
        if (root) {
          const selection = currentSelection(root, selectionRef.current);
          if (selection.start === selection.end) {
            const range = adjacentSkillRange(
              value,
              skillNames,
              selection.start,
              event.key === "Backspace" ? "backward" : "forward",
            );
            if (range) {
              event.preventDefault();
              selectionRef.current = range;
              applySelection(root, range.start, range.end);
              replaceSelection("");
              return;
            }
          }
        }
      }
      onKeyDown?.(event);
    };

    const removeSkillAtTarget = (target: EventTarget | null): boolean => {
      const root = rootRef.current;
      const ElementCtor = root?.ownerDocument.defaultView?.Element;
      if (!root || !ElementCtor || !(target instanceof ElementCtor)) {
        return false;
      }
      const removeButton = target.closest(`[${SKILL_REMOVE_ATTR}]`);
      if (!removeButton) {
        return false;
      }
      const token = removeButton.closest<HTMLElement>(`[${SKILL_NODE_ATTR}]`);
      const name = token?.getAttribute(SKILL_NODE_ATTR);
      if (!token || !name || disabled) return false;

      const tokenIndex = Array.from(root.childNodes).indexOf(token);
      const caret = tokenIndex >= 0 ? lengthBeforePoint(root, root, tokenIndex) : 0;
      const next = removeSkillTokenFromText(editorValue(root), skillNames, name);
      selectionRef.current = {
        start: Math.min(caret, next.length),
        end: Math.min(caret, next.length),
      };
      root.replaceChildren(fragmentForValue(next, skillNames, skillLabels));
      root.focus();
      applySelection(root, selectionRef.current.start);
      onChange(next);
      onSelectionChange?.(selectionRef.current);
      return true;
    };

    const handleClick = (event: MouseEvent<HTMLDivElement>) => {
      if (!removeSkillAtTarget(event.target)) {
        rememberSelection();
        return;
      }
      event.preventDefault();
      event.stopPropagation();
    };

    return (
      <div
        id={id}
        ref={rootRef}
        className={className}
        style={style}
        role="textbox"
        aria-label={ariaLabel}
        aria-multiline="true"
        aria-disabled={disabled}
        data-placeholder={placeholder || ""}
        data-empty={value === "" ? "true" : "false"}
        contentEditable={!disabled}
        suppressContentEditableWarning
        spellCheck
        onInput={onInput}
        onSelect={rememberSelection}
        onMouseDown={(event) => {
          if (!removeSkillAtTarget(event.target)) return;
          // WebView2 can suppress the later click for controls nested in a
          // contenteditable=false token. Handle the primary mouse gesture here
          // and retain onClick as a programmatic/accessibility fallback.
          event.preventDefault();
          event.stopPropagation();
        }}
        onClick={handleClick}
        onKeyUp={rememberSelection}
        onFocus={rememberSelection}
        onPaste={onPaste}
        onCopy={(event) => copySelection(event, false)}
        onCut={(event) => copySelection(event, true)}
        onKeyDown={handleKeyDown}
        onCompositionStart={onCompositionStart}
        onCompositionEnd={onCompositionEnd}
      />
    );
  },
);
