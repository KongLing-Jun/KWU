import {
  PROJECT_TREE_SAFE_TOPIC_TITLE,
  projectTreeDisplayLabel,
} from "../components/ProjectTree";
import type { ProjectNode } from "../lib/types";

function ok(value: unknown, message: string): asserts value {
  if (!value) throw new Error(message);
}

const internalTopicId = "topic_01internal";
const emptyTopic: ProjectNode = {
  key: "global_topic_internal",
  kind: "global_topic",
  label: "",
  topicId: internalTopicId,
};
ok(
  projectTreeDisplayLabel(emptyTopic) === PROJECT_TREE_SAFE_TOPIC_TITLE,
  "an empty backend label must not expose topicId",
);
ok(
  !projectTreeDisplayLabel(emptyTopic).includes(internalTopicId),
  "the safe fallback must not contain an internal topic identity",
);

const persistedTitle: ProjectNode = {
  ...emptyTopic,
  label: "  用户保存的标题  ",
};
ok(
  projectTreeDisplayLabel(persistedTitle) === "用户保存的标题",
  "a persisted user title must retain precedence",
);

const statusPrefixedTitle: ProjectNode = {
  ...emptyTopic,
  label: "● 正在处理",
};
ok(
  projectTreeDisplayLabel(statusPrefixedTitle) === "正在处理",
  "the legacy status bullet should still be removed",
);

console.log("project tree safe label tests passed");
