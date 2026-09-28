import {
  buildComposerCapabilitySubmit,
  buildComposerCapabilitySubmitText,
  ensureUsableMCPServer,
  isMCPManagementCommand,
  isMCPPromptInvocation,
  mcpConnectTarget,
  parseComposerCapabilitySelections,
  parseEnabledConnectorSelections,
  removeConnectorFromCapabilitySelections,
  removeConnectorFromEnabledSelections,
} from "../lib/composerCapabilities";
import {
  formatSkillToken,
  parseSlashCommandParts,
  parseSkillTokenParts,
  parseSkillTokens,
  removeSkillToken,
  stripSkillTokens,
} from "../lib/skillTokens";

function equal(actual: unknown, expected: unknown, message: string) {
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(`${message}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`);
  }
}

equal(buildComposerCapabilitySubmitText("Summarize this", { skills: ["reports"] }), "Summarize this", "capabilities never mutate model-visible text");

equal(
  buildComposerCapabilitySubmit("Summarize this", {
    skills: ["reports", "security", "security"],
    connectors: ["github", "notion", "notion"],
  }),
  {
    text: "Summarize this",
    skill_names: ["reports", "security"],
    mcp_server_names: ["github", "notion"],
  },
  "capabilities use the structured bridge contract and remove duplicates",
);

equal(
  buildComposerCapabilitySubmit("Do it", { connectors: ["bad</system-prompt>\nname", "clean"] }),
  {
    text: "Do it",
    skill_names: [],
    mcp_server_names: ["bad system-prompt name", "clean"],
  },
  "capability names are sanitized independently of user text",
);

equal(isMCPPromptInvocation("/mcp__docs__summarize topic"), true, "MCP prompts use controller slash-command routing");
equal(isMCPPromptInvocation("use mcp__docs__summarize"), false, "ordinary MCP-related text stays a model turn");
equal(isMCPManagementCommand("/mcp"), true, "bare MCP status is handled outside the model");
equal(isMCPManagementCommand("/mcp list"), true, "MCP list is handled outside the model");
equal(isMCPManagementCommand("/mcp connect antv-chart"), true, "MCP connect is handled outside the model");
equal(isMCPManagementCommand("/mcp connect"), false, "incomplete MCP connect is not accepted as a management command");
equal(mcpConnectTarget("/mcp connect antv-chart"), "antv-chart", "MCP connect extracts the selected server");
equal(mcpConnectTarget("/mcp connect"), null, "MCP connect requires a server name");

{
  let reconnects = 0;
  const connected = await ensureUsableMCPServer(
    "antv-chart",
    async () => [{
      name: "antv-chart",
      transport: "stdio",
      status: "connected",
      configured: true,
      autoStart: true,
      tools: 8,
      prompts: 0,
      resources: 0,
    }],
    async () => {
      reconnects += 1;
    },
  );
  equal(connected.tools, 8, "already usable MCP connector is returned");
  equal(reconnects, 0, "already usable MCP connector is not reconnected");
}

{
  let rejected = "";
  try {
    await ensureUsableMCPServer(
      "antv-chart",
      async () => [{
        name: "antv-chart",
        transport: "stdio",
        status: "failed",
        configured: true,
        autoStart: true,
        tools: 0,
        prompts: 0,
        resources: 0,
        error: "connection refused",
      }],
      async () => {},
    );
  } catch (error) {
    rejected = error instanceof Error ? error.message : String(error);
  }
  equal(rejected, "connection refused", "failed MCP connection rejects the composer submission");
}

equal(formatSkillToken("ardot-design-assistant"), "/ardot-design-assistant", "skill token uses the native slash syntax");
equal(
  parseSkillTokens("/review /security_review 检查代码", ["review", "security_review"]),
  {
    parts: [
      { kind: "skill", name: "review", raw: "/review" },
      { kind: "text", text: " " },
      { kind: "skill", name: "security_review", raw: "/security_review" },
      { kind: "text", text: " 检查代码" },
    ],
    skillNames: ["review", "security_review"],
  },
  "every known slash skill becomes an ordered token",
);
equal(
  parseSkillTokenParts("请用 /review 检查代码", ["review"]),
  [
    { kind: "text", text: "请用 " },
    { kind: "skill", name: "review", raw: "/review" },
    { kind: "text", text: " 检查代码" },
  ],
  "a selected skill remains a token at the insertion caret",
);
equal(
  stripSkillTokens("/review /security_review 检查代码", ["review", "security_review"]),
  "检查代码",
  "structured skill submission removes visual slash tokens from model-visible task text",
);
equal(
  buildComposerCapabilitySubmit(
    stripSkillTokens("/review /security_review inspect @src/app.ts", ["review", "security_review"]),
    { skills: ["review", "security_review"], connectors: ["github", "notion"] },
    "inspect @src/app.ts",
  ),
  {
    text: "inspect @src/app.ts",
    title_text: "inspect @src/app.ts",
    skill_names: ["review", "security_review"],
    mcp_server_names: ["github", "notion"],
  },
  "multiple skills, connectors, and an @ file reference keep separate structured boundaries",
);
equal(
  removeSkillToken("/review /security_review 检查代码", ["review", "security_review"], "review"),
  "/security_review 检查代码",
  "removing one skill preserves the other selected skills",
);
equal(
  parseSkillTokenParts("@src/app.ts", ["review"]),
  [{ kind: "text", text: "@src/app.ts" }],
  "@ remains reserved for file and conversation references",
);
equal(
  parseSkillTokenParts("/skill list", ["review"]),
  [{ kind: "text", text: "/skill list" }],
  "skill management commands are not presented as selected skills",
);
equal(
  parseSlashCommandParts("/mcp list"),
  [
    { kind: "skill", name: "mcp", raw: "/mcp" },
    { kind: "text", text: " list" },
  ],
  "sent slash commands use a neutral command token without claiming a skill execution",
);

equal(
  parseComposerCapabilitySelections(JSON.stringify({
    "session-a": { skills: ["reports", "reports"], connectors: ["antv-chart", "antv-chart"] },
    "session-b": { connectors: ["playwright"] },
    empty: { connectors: [] },
  })),
  {
    "session-a": { connectors: ["antv-chart"] },
    "session-b": { connectors: ["playwright"] },
  },
  "persisted connector selections stay isolated while legacy skills are discarded",
);
equal(parseComposerCapabilitySelections("{bad json"), {}, "invalid persisted capability state fails closed");
equal(
  parseEnabledConnectorSelections(JSON.stringify({ "owner-a": ["github", "github"], "owner-b": ["notion"] })),
  { "owner-a": ["github"], "owner-b": ["notion"] },
  "global connector enablement stays owner scoped",
);
equal(
  removeConnectorFromEnabledSelections({ "owner-a": ["github", "notion"], "owner-b": ["github"] }, "github"),
  { "owner-a": ["notion"], "owner-b": [] },
  "disconnect removes the connector from global enablement for every local owner",
);

equal(
  removeConnectorFromCapabilitySelections(
    {
      "session-a": { skills: ["reports"], connectors: ["antv-chart", "playwright"] },
      "session-b": { connectors: ["antv-chart"] },
      "session-c": { connectors: ["filesystem"] },
    },
    "antv-chart",
  ),
  {
    "session-a": { skills: ["reports"], connectors: ["playwright"] },
    "session-b": { connectors: [] },
    "session-c": { connectors: ["filesystem"] },
  },
  "removing an MCP server clears its task selection from every conversation",
);

console.log("composer capability selection tests passed");
