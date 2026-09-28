import { Fragment } from "react";
import { Hammer } from "lucide-react";
import { displaySkillTokenLabel, parseSkillTokenParts, parseSlashCommandParts } from "../lib/skillTokens";

function submittedSkillNames(value: string, submitText?: string): string[] {
  if (submitText === undefined) return [];
  const submittedCommands = new Set<string>();
  for (const part of parseSlashCommandParts(submitText)) {
    if (part.kind === "skill") submittedCommands.add(part.name);
  }
  const names = new Set<string>();
  for (const part of parseSlashCommandParts(value)) {
    if (part.kind === "skill" && !submittedCommands.has(part.name)) names.add(part.name);
  }
  return Array.from(names);
}

export function SkillTokenText({ value, submitText }: { value: string; submitText?: string }) {
  const skillNames = submittedSkillNames(value, submitText);
  return (
    <>
      {parseSkillTokenParts(value, skillNames).map((part, index) => (
        part.kind === "skill" ? (
          <span
            className="message-skill-token"
            data-skill-token={part.name}
            title={part.name}
            key={`${part.raw}:${index}`}
          >
            <span className="message-skill-token__icon" aria-hidden="true">
              <Hammer size={13} />
            </span>
            <span className="message-skill-token__label">{displaySkillTokenLabel(part.name)}</span>
          </span>
        ) : (
          <Fragment key={`text:${index}`}>{part.text}</Fragment>
        )
      ))}
    </>
  );
}
