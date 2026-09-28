import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { JSDOM } from "jsdom";
import { SkillTokenText } from "../components/SkillTokenText";

function ok(value: boolean, message: string) {
  if (!value) throw new Error(message);
}

const html = renderToStaticMarkup(
  <SkillTokenText value="/ardot-design-assistant /code-refactor 你好" submitText="你好" />,
);
const visibleText = new JSDOM(html).window.document.body.textContent ?? "";

ok(html.includes('data-skill-token="ardot-design-assistant"'), "user message renders the slash invocation as a token");
ok(html.includes('data-skill-token="code-refactor"'), "user message renders every selected skill token");
ok(html.includes("ardot-design-assistant"), "rendered token keeps the visible command name");
ok(html.includes("你好"), "rendered token keeps surrounding user text");
ok(html.includes("lucide-hammer"), "sent skill tokens reuse the shared skill icon");
ok(!visibleText.includes("/ardot-design-assistant"), "the visual token hides the serialized slash prefix");

const builtinHtml = renderToStaticMarkup(
  <SkillTokenText value="/mcp connect antv-chart" />,
);
ok(!builtinHtml.includes("message-skill-token"), "ordinary slash commands are not mislabeled as skills");
ok(builtinHtml.includes("/mcp connect antv-chart"), "ordinary slash commands remain visible as text");

const creationHtml = renderToStaticMarkup(
  <SkillTokenText value="/document 你好" submitText="你好" />,
);
ok(creationHtml.includes('data-skill-token="document"'), "creation shortcut keeps its stable skill identifier");
ok(creationHtml.includes("文档处理"), "creation shortcut uses its localized label in sent messages");
ok(!creationHtml.includes(">document<"), "creation shortcut does not expose its internal identifier as the visible label");

console.log("skill token message rendering tests passed");
