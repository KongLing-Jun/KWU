import { test as base, expect } from "@playwright/test";

type BrowserEvidence = {
  console: Array<{ type: string; text: string }>;
  pageErrors: string[];
  failedRequests: Array<{ method: string; url: string; failure?: string }>;
};

export const test = base.extend<{ runtimeEvidence: void }>({
  runtimeEvidence: [async ({ page }, use, testInfo) => {
    const evidence: BrowserEvidence = {
      console: [],
      pageErrors: [],
      failedRequests: [],
    };

    page.on("console", (message) => {
      evidence.console.push({ type: message.type(), text: message.text() });
    });
    page.on("pageerror", (error) => {
      evidence.pageErrors.push(error.message);
    });
    page.on("requestfailed", (request) => {
      evidence.failedRequests.push({
        method: request.method(),
        url: request.url(),
        failure: request.failure()?.errorText,
      });
    });

    await use();

    await testInfo.attach("runtime-browser-evidence.json", {
      body: Buffer.from(JSON.stringify(evidence, null, 2)),
      contentType: "application/json",
    });
  }, { auto: true }],
});

export { expect };
