import { chromium } from "@playwright/test";

const endpointIndex = process.argv.indexOf("--endpoint");
if (endpointIndex < 0 || !process.argv[endpointIndex + 1]) {
  console.error("usage: node e2e/webview2-cdp-probe.mjs --endpoint http://127.0.0.1:<port>");
  process.exit(2);
}

const endpoint = process.argv[endpointIndex + 1];
const browser = await chromium.connectOverCDP(endpoint);
try {
  const deadline = Date.now() + 15_000;
  let pages = browser.contexts().flatMap((context) => context.pages());
  while (pages.length === 0 && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    pages = browser.contexts().flatMap((context) => context.pages());
  }
  if (pages.length === 0) {
    throw new Error("WebView2 CDP endpoint exposed no pages");
  }

  const page = pages[0];
  await page.waitForLoadState("domcontentloaded", { timeout: 10_000 }).catch(() => {});
  const result = {
    contextCount: browser.contexts().length,
    pageCount: pages.length,
    page: {
      url: page.url(),
      title: await page.title(),
      language: await page.locator("html").getAttribute("lang"),
      hasWailsRuntime: await page.evaluate(() => typeof window.runtime !== "undefined"),
    },
  };
  process.stdout.write(`${JSON.stringify(result)}\n`);
} finally {
  await browser.close();
}
