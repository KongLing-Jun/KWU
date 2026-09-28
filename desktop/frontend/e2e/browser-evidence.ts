import { test as base, expect } from "@playwright/test";

type ConsoleEntry = {
  type: string;
  text: string;
};

type NetworkEntry = {
  method: string;
  path: string;
  status?: number;
  failure?: string;
};

export const test = base.extend<{ browserEvidence: void }>({
  browserEvidence: [async ({ page }, use, testInfo) => {
    const consoleEntries: ConsoleEntry[] = [];
    const networkEntries: NetworkEntry[] = [];

    page.on("console", (message) => {
      consoleEntries.push({ type: message.type(), text: message.text() });
    });
    page.on("response", (response) => {
      const url = new URL(response.url());
      networkEntries.push({
        method: response.request().method(),
        path: url.pathname,
        status: response.status(),
      });
    });
    page.on("requestfailed", (request) => {
      const url = new URL(request.url());
      networkEntries.push({
        method: request.method(),
        path: url.pathname,
        failure: request.failure()?.errorText,
      });
    });

    await use();

    await testInfo.attach("browser-evidence.json", {
      body: Buffer.from(JSON.stringify({
        console: consoleEntries,
        network: networkEntries,
      }, null, 2)),
      contentType: "application/json",
    });
  }, { auto: true }],
});

export { expect };
