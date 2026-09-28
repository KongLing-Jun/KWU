// Run: tsx src/__tests__/client-error.test.ts

import {
  clientErrorCode,
	clientErrorDiagnosticID,
  clientErrorMessageWithoutDiagnosticID,
  clientErrorPresentation,
  installClientErrorLocalSink,
  redactClientErrorText,
  reportClientError,
  type ClientErrorLog,
} from "../lib/clientError";
import type { ClientErrorRecord } from "../lib/bridge";

let failed = 0;

function equal(actual: unknown, expected: unknown, label: string) {
  if (actual === expected) {
    process.stdout.write(`  PASS  ${label}\n`);
    return;
  }
  process.stderr.write(`  FAIL  ${label}: expected ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}\n`);
  failed += 1;
}

console.log("\nclient error interception");

equal(
  clientErrorPresentation({ code: "SLIDER_CAPTCHA_REQUIRED", message: "raw backend detail" }).userMessage,
  "请先完成滑块验证后重试。",
  "已知验证码错误显示中文行动提示",
);
equal(
  clientErrorPresentation("DURABLE_CREDENTIAL_STATE_INVALID: local authentication state was rejected").userMessage,
  "当前登录状态异常，请重新登录后重试。",
  "本地登录状态异常提示重新登录",
);
equal(
  clientErrorPresentation(new Error("database connection string"), { fallbackMessage: "保存失败，请稍后重试。" }).userMessage,
  "保存失败，请稍后重试。",
  "未知错误不直接暴露原始消息",
);
equal(clientErrorCode("request failed: PERMISSION_DENIED: internal detail"), "PERMISSION_DENIED", "可从消息中提取错误码");
const cloudFailure = new Error("CLOUD_ERROR: cloud request failed [request_id=req-1] [diagnostic_id=diag-safe_123]");
equal(clientErrorDiagnosticID(cloudFailure), "diag-safe_123", "可从 Wails 错误字符串提取诊断编号");
equal(clientErrorMessageWithoutDiagnosticID(cloudFailure), "CLOUD_ERROR: cloud request failed [request_id=req-1]", "可见文本剥离诊断编号传输元数据");
equal(clientErrorPresentation(cloudFailure).userMessage, "操作失败，请稍后重试。", "generic Wails string 错误不改写用户正文");
equal(
  clientErrorPresentation({ code: "CLOUD_ERROR", message: "cloud request failed", diagnostic_id: "diag-object_123" }).userMessage,
  "操作失败，请稍后重试。",
  "generic structured 错误不改写用户正文",
);
equal(
  clientErrorPresentation({ code: "CLOUD_ERROR", diagnostic_id: "invalid:id" }).userMessage,
  "操作失败，请稍后重试。",
  "非法诊断编号不追加到用户消息",
);
equal(clientErrorPresentation("CLOUD_ERROR: cloud request failed").userMessage, "操作失败，请稍后重试。", "无诊断编号时不追加用户消息");
equal(
  clientErrorPresentation(new Error("SKILL_INSTALL_BUSY: rename access denied"), { fallbackMessage: "安装技能失败，请稍后重试。" }).userMessage,
  "技能文件正被其他程序占用，请稍后重试。",
  "技能落盘占用错误显示重试提示",
);

const redacted = redactClientErrorText(
  "Bearer abc.def phone=13800138000 email=user@example.com path=C:\\Users\\alice\\secret token=visible",
);
equal(redacted.includes("abc.def"), false, "脱敏授权 token");
equal(redacted.includes("13800138000"), false, "脱敏手机号");
equal(redacted.includes("user@example.com"), false, "脱敏邮箱");
equal(redacted.includes("C:\\Users\\alice"), false, "脱敏本地路径");

const logs: ClientErrorLog[] = [];
const repeated = new Error("password=secret C:\\Users\\alice\\project");
reportClientError("settings.save", repeated, { fallbackMessage: "保存设置失败。", log: (entry) => logs.push(entry) });
reportClientError("settings.save", repeated, { fallbackMessage: "保存设置失败。", log: (entry) => logs.push(entry) });
equal(logs.length, 1, "同一错误对象只记录一次");
equal(logs[0]?.summary.includes("secret"), false, "诊断摘要不包含敏感值");
equal(logs[0]?.scope, "settings.save", "诊断日志包含业务场景");

const localRecords: ClientErrorRecord[] = [];
installClientErrorLocalSink((record) => localRecords.push(record));
const repeatedForJournal = new Error("NETWORK_ERROR: password=secret C:\\Users\\alice\\private.txt");
const consoleRecords: ClientErrorLog[] = [];
reportClientError("workspace.preview", repeatedForJournal, { log: (entry) => consoleRecords.push(entry) });
reportClientError("workspace.preview", repeatedForJournal, { log: (entry) => consoleRecords.push(entry) });
equal(consoleRecords.length, 1, "同一错误对象的既有 console 去重保持");
equal(localRecords.length, 2, "local sink 位于永久对象去重之外");
equal(localRecords[0]?.scope, "workspace.preview", "local sink 只传 allowlisted scope");
equal(localRecords[0]?.code, "NETWORK_ERROR", "local sink 只传 allowlisted code");
equal("summary" in (localRecords[0] ?? {}), false, "local sink DTO 不包含摘要");
reportClientError("account.operation", cloudFailure);
equal(localRecords[2]?.code, "CLOUD_ERROR", "cloud fallback code 保留到 local sink");
equal(localRecords[2]?.diagnosticId, "diag-safe_123", "diagnostic id 保留到 local sink");
reportClientError("workspace.preview", new Error("ordinary failure"), {
  stackFrames: [{ category: "app", ordinal: 0, line: 1, column: 1 }],
});
equal("stackFrames" in (localRecords[3] ?? {}), false, "普通业务 failure 不附加 frames");

const canaryRecords: ClientErrorRecord[] = [];
installClientErrorLocalSink((record) => canaryRecords.push(record));
reportClientError("C:\\Users\\alice\\private\\workspace.preview", {
  name: "function=send",
  code: "LEGITIMATE_UNKNOWN_CODE",
  message: "body=password=secret Bearer abc.def user@example.com 13800138000",
});
const canaryPayload = JSON.stringify(canaryRecords[0]);
equal(canaryRecords[0]?.scope, "frontend.unknown", "恶意 scope 在 renderer boundary 前固定化");
equal(canaryRecords[0]?.code, "UNKNOWN", "合法格式但未知 code 在 renderer boundary 前固定化");
equal(canaryRecords[0]?.errorType, "Error", "恶意 error type 在 renderer boundary 前固定化");
equal(canaryPayload.includes("password"), false, "local sink DTO 不携带错误正文");
equal(canaryPayload.includes("private"), false, "local sink DTO 不携带路径尾段");

installClientErrorLocalSink();

if (failed > 0) process.exit(1);
