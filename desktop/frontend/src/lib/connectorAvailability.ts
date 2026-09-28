import type { ServerView } from "./types";

export type ConnectorAvailabilityState =
  | "uninstalled"
  | "connecting"
  | "configuration_error"
  | "connected";

export interface ConnectorAvailability {
  state: ConnectorAvailabilityState;
  server?: ServerView;
  selectable: boolean;
  label: string;
}

export function resolveConnectorAvailability(
  serverName: string | undefined,
  servers: readonly ServerView[],
): ConnectorAvailability {
  if (!serverName) {
    return { state: "uninstalled", selectable: false, label: "未连接" };
  }
  const server = servers.find((candidate) => candidate.name === serverName);
  if (!server) {
    return { state: "uninstalled", selectable: false, label: "未连接" };
  }
  if (server.status === "initializing" || server.status === "deferred") {
    return { state: "connecting", server, selectable: false, label: "正在连接" };
  }
  if (server.status === "connected" && server.tools > 0) {
    return {
      state: "connected",
      server,
      selectable: true,
      label: `已连接 · ${server.tools} 个工具`,
    };
  }
  return {
    state: "configuration_error",
    server,
    selectable: false,
    label: server.error || "连接失败",
  };
}
