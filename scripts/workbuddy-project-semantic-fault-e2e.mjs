import { randomUUID } from "node:crypto";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import path from "node:path";

function option(name) {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0 || !process.argv[index + 1]) {
    throw new Error(`missing --${name}`);
  }
  return process.argv[index + 1];
}

const manifestPath = path.resolve(option("manifest"));
const barrierDirectory = path.resolve(option("barrier-directory"));
const controlDirectory = path.resolve(option("control-directory"));
const outputPath = path.resolve(option("output"));
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
const baseURL = String(manifest.base_url || "").replace(/\/$/, "");
const accessToken = String(manifest.access_token_a || "");
if (!baseURL.startsWith("http://127.0.0.1:") || !accessToken) {
  throw new Error("semantic fault manifest is missing the owned loopback Cloud or owner token");
}

const runID = randomUUID();
const key = (name) => `stage5-${name}-${runID}`;
const headers = (idempotencyKey) => ({
  Authorization: `Bearer ${accessToken}`,
  "Content-Type": "application/json",
  ...(idempotencyKey ? { "X-Idempotency-Key": idempotencyKey } : {}),
});

async function request(method, route, body, idempotencyKey, expected = [200]) {
  const response = await fetch(`${baseURL}${route}`, {
    method,
    headers: headers(idempotencyKey),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const payload = await response.json().catch(() => ({}));
  if (!expected.includes(response.status)) {
    const code = payload?.error?.code || "unknown";
    throw new Error(`${method} ${route} returned ${response.status} (${code})`);
  }
  return { status: response.status, data: payload.data };
}

async function waitForBarrier(point, timeoutMs = 20_000) {
  const barrier = path.join(barrierDirectory, `${point}.json`);
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      await stat(barrier);
      const value = JSON.parse(await readFile(barrier, "utf8"));
      if (value.point !== point || !value.idempotency_key_sha256) {
        throw new Error(`invalid ${point} barrier`);
      }
      return barrier;
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
  }
  throw new Error(`timed out waiting for ${point} barrier`);
}

async function requestDatabaseControl(action, payload, timeoutMs = 20_000) {
  const requestPath = path.join(controlDirectory, `${action}.request.json`);
  const acknowledgementPath = path.join(controlDirectory, `${action}.ack.json`);
  await writeFile(
    requestPath,
    `${JSON.stringify({ schema: "reasonix.workbuddy-project.database-control/v1", action, ...payload }, null, 2)}\n`,
    { encoding: "utf8", mode: 0o600, flag: "wx" },
  );
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const acknowledgement = JSON.parse(await readFile(acknowledgementPath, "utf8"));
      if (acknowledgement.action !== action || acknowledgement.success !== true) {
        throw new Error(`database control ${action} was not acknowledged safely`);
      }
      return;
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
  }
  throw new Error(`timed out waiting for database control ${action}`);
}

async function loseResponse(point, method, route, body, idempotencyKey) {
  const controller = new AbortController();
  const pending = fetch(`${baseURL}${route}`, {
    method,
    headers: { ...headers(idempotencyKey), "X-WorkBuddy-RC-Fault": point },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: controller.signal,
  });
  await waitForBarrier(point);
  controller.abort();
  let aborted = false;
  try {
    await pending;
  } catch (error) {
    aborted = error?.name === "AbortError";
  }
  if (!aborted) throw new Error(`${point} response was not lost at the client boundary`);
}

await mkdir(barrierDirectory, { recursive: true });
await mkdir(controlDirectory, { recursive: true });
const created = await request(
  "POST",
  "/api/v1/projects/aggregates",
  {
    name: "Stage 5 semantic fault matrix",
    human_description: "Owned release-candidate recovery proof.",
    agent_instruction: "Do not perform external side effects.",
    bindings: [],
  },
  key("project-create"),
);
const projectID = created.data?.project_id;
if (!projectID || created.data?.project_version !== 1) throw new Error("aggregate creation contract mismatch");

const inviteKey = key("invite-loss");
await loseResponse(
  "invite_secret_response_loss",
  "POST",
  `/api/v1/projects/${projectID}/invites`,
  { expires_in_seconds: 86_400 },
  inviteKey,
);
const inviteReplay = await request(
  "POST",
  `/api/v1/projects/${projectID}/invites`,
  { expires_in_seconds: 86_400 },
  inviteKey,
);
if (!inviteReplay.data?.committed_without_secret || inviteReplay.data?.token) {
  throw new Error("invite replay exposed or regenerated a one-time secret");
}
const invites = await request("GET", `/api/v1/projects/${projectID}/invites`);
const lostInvite = invites.data?.find((item) => item.invite_id === inviteReplay.data.invite_id);
if (!lostInvite || lostInvite.status !== "pending" || lostInvite.version !== 1) {
  throw new Error("lost invite was not authoritatively readable");
}
await request(
  "POST",
  `/api/v1/projects/${projectID}/invites/${lostInvite.invite_id}/revoke`,
  { expected_version: 1 },
  key("invite-revoke"),
);
const replacementInvite = await request(
  "POST",
  `/api/v1/projects/${projectID}/invites`,
  { expires_in_seconds: 86_400 },
  key("invite-replacement"),
);
if (!replacementInvite.data?.token || replacementInvite.data?.committed_without_secret) {
  throw new Error("replacement invite did not issue exactly one fresh secret");
}

const clientMessageID = `stage5-semantic-${runID}`;
const activityKey = key("activity-loss");
const activityBody = {
  client_message_id: clientMessageID,
  text: "Stage 5 response-loss recovery proof.",
  mention_user_ids: [],
};
await loseResponse(
  "activity_response_loss",
  "POST",
  `/api/v1/projects/${projectID}/messages`,
  activityBody,
  activityKey,
);
const activityReadback = await request(
  "GET",
  `/api/v1/projects/${projectID}/messages/by-client-id/${encodeURIComponent(clientMessageID)}`,
);
const activityReplay = await request(
  "POST",
  `/api/v1/projects/${projectID}/messages`,
  activityBody,
  activityKey,
);
if (
  activityReadback.data?.client_message_id !== clientMessageID ||
  activityReplay.data?.resource_id !== activityReadback.data?.activity_id
) {
  throw new Error("Activity replay did not converge on the authoritative message");
}

const deleteKey = key("delete-loss");
const deleteBody = { expected_version: 1 };
await loseResponse(
  "permanent_delete_post_commit",
  "DELETE",
  `/api/v1/projects/${projectID}`,
  deleteBody,
  deleteKey,
);
await request("GET", `/api/v1/projects/${projectID}/aggregate`, undefined, undefined, [404]);
const deleteReplay = await request("DELETE", `/api/v1/projects/${projectID}`, deleteBody, deleteKey);
if (deleteReplay.data?.resource_id !== projectID || deleteReplay.data?.committed_version !== 2) {
  throw new Error("permanent delete tombstone replay contract mismatch");
}

const report = {
  schema: "reasonix.workbuddy-project.semantic-fault-e2e/v1",
  verdict: "PASS",
  projectID,
  faults: {
    inviteSecretResponseLoss: {
      barrier: true,
      committedWithoutSecret: true,
      replacementIssuedFreshSecret: true,
      secretPersistedInReport: false,
    },
    activityResponseLoss: {
      barrier: true,
      clientMessageID,
      authoritativeReadback: true,
      replayConverged: true,
    },
    permanentDeletePostCommit: {
      barrier: true,
      aggregateNotFound: true,
      tombstoneReplayConverged: true,
    },
  },
};
await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(outputPath, `${JSON.stringify(report, null, 2)}\n`, { encoding: "utf8", mode: 0o600 });
process.stdout.write(`${JSON.stringify(report)}\n`);
