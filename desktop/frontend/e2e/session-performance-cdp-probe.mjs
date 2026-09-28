import { chromium } from "@playwright/test";
import path from "node:path";

function arg(name, required = true) {
  const index = process.argv.indexOf(name);
  const value = index >= 0 ? process.argv[index + 1] : "";
  if (required && !value) {
    console.error(`missing ${name}`);
    process.exit(2);
  }
  return value;
}

function percentile(samples, p) {
  const sorted = [...samples].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.ceil((p / 100) * sorted.length) - 1);
  return Number(sorted[Math.max(0, index)].toFixed(2));
}

function summary(samples) {
  if (samples.length === 0) return { samples: [], p50Ms: null, p95Ms: null };
  return {
    samples: samples.map((value) => Number(value.toFixed(2))),
    p50Ms: percentile(samples, 50),
    p95Ms: percentile(samples, 95),
  };
}

async function measure(action) {
  const started = performance.now();
  await action();
  return performance.now() - started;
}

const endpoint = arg("--endpoint");
const outputDirectory = path.resolve(arg("--output"));
const browser = await chromium.connectOverCDP(endpoint);
try {
  const deadline = Date.now() + 20_000;
  let pages = browser.contexts().flatMap((context) => context.pages());
  while (pages.length === 0 && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 100));
    pages = browser.contexts().flatMap((context) => context.pages());
  }
  if (pages.length === 0) throw new Error("WebView2 CDP endpoint exposed no pages");

  const page = pages[0];
  await page.waitForLoadState("domcontentloaded", { timeout: 15_000 }).catch(() => {});
  await page.locator("#main-workspace").waitFor({ state: "visible", timeout: 30_000 });
  await page.waitForFunction(() => Boolean(window.go?.main?.App?.ListAllSessions), null, { timeout: 30_000 });
  await page.evaluate(() => {
    window.__kwuPerfLongTasks = [];
    window.__kwuPerfCalls = { history: [], subagent: [] };
    if (Array.isArray(window.__KWU_REACT_PROFILER_COMMITS__)) {
      window.__KWU_REACT_PROFILER_COMMITS__.length = 0;
    }
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        window.__kwuPerfLongTasks.push({
          startTime: entry.startTime,
          duration: entry.duration,
        });
      }
    });
    observer.observe({ type: "longtask", buffered: true });
    window.__kwuPerfObserver = observer;
    const app = window.go.main.App;
    const historyPage = app.HistoryPageForTab.bind(app);
    app.HistoryPageForTab = async (...args) => {
      const started = performance.now();
      try {
        return await historyPage(...args);
      } finally {
        window.__kwuPerfCalls.history.push(performance.now() - started);
      }
    };
    const subagentPage = app.SubagentHistoryPageForTab.bind(app);
    app.SubagentHistoryPageForTab = async (...args) => {
      const started = performance.now();
      try {
        return await subagentPage(...args);
      } finally {
        window.__kwuPerfCalls.subagent.push(performance.now() - started);
      }
    };
  });

  const listSamples = [];
  let sessions = [];
  for (let i = 0; i < 5; i += 1) {
    listSamples.push(await measure(async () => {
      sessions = await page.evaluate(() => window.go.main.App.ListAllSessions());
    }));
  }
  if (sessions.length < 500) {
    throw new Error(`expected at least 500 sessions, got ${sessions.length}`);
  }

  const openSamples = [];
  for (let i = 0; i < 5; i += 1) {
    openSamples.push(await measure(async () => {
      await page.getByRole("button", { name: "对话中心" }).click();
      await page.locator('section[aria-label="对话中心"]').waitFor({ state: "visible" });
      await page.locator(".hist-item__main").first().waitFor({ state: "visible" });
    }));
    const modal = page.locator('section[aria-label="对话中心"]');
    await modal.getByRole("button", { name: "关闭" }).click();
    await modal.waitFor({ state: "detached" });
  }

  const modal = page.locator('section[aria-label="对话中心"]');
  const activateDuration = await measure(async () => {
    await page.getByRole("button", { name: "对话中心" }).click();
    await modal.waitFor({ state: "visible" });
    await modal.getByPlaceholder("搜索会话…").fill("PERF-LONG-SESSION");
    const target = modal.locator(".hist-item__main").filter({ hasText: "PERF-LONG-SESSION" });
    await target.waitFor({ state: "visible" });
    await target.dblclick();
    await modal.waitFor({ state: "detached" });
    await page.getByText("PERF-LONG-LAST", { exact: true }).waitFor({ state: "visible", timeout: 180_000 });
  });

  const olderSamples = [];
  for (let i = 0; i < 5; i += 1) {
    const button = page.getByRole("button", { name: "加载更早的对话" }).first();
    await button.waitFor({ state: "visible", timeout: 20_000 });
    const completed = await page.evaluate(() => window.__kwuPerfCalls.history.length);
    olderSamples.push(await measure(async () => {
      await button.click();
      await page.waitForFunction(
        (count) => window.__kwuPerfCalls.history.length > count,
        completed,
        { timeout: 20_000 },
      );
      await page.getByRole("button", { name: "加载更早的对话" }).first().waitFor({ state: "visible", timeout: 20_000 });
    }));
  }

  await page.getByRole("button", { name: "对话中心" }).click();
  await modal.waitFor({ state: "visible" });
  await modal.getByPlaceholder("搜索会话…").fill("PERF-SUBAGENT-SESSION");
  const subagentParent = modal.locator(".hist-item__main").filter({ hasText: "PERF-SUBAGENT-SESSION" });
  await subagentParent.waitFor({ state: "visible" });
  await subagentParent.dblclick();
  await modal.waitFor({ state: "detached" });
  await page.getByText("PERF-SUBAGENT-PARENT-LAST", { exact: true }).waitFor({ state: "visible", timeout: 30_000 });

  const taskCard = page.locator(".tool").filter({ has: page.locator(".tool__name", { hasText: "task" }) }).last();
  await taskCard.waitFor({ state: "visible", timeout: 5_000 }).catch(async (error) => {
    const diagnostic = await page.evaluate(async () => {
      const tabs = await window.go.main.App.ListTabs();
      const active = tabs.find((tab) => tab.active);
      const history = active ? await window.go.main.App.HistoryPageForTab(active.id, 0, 20, "") : null;
      return { active, history };
    });
    const toolNames = await page.locator(".tool__name").allTextContents();
    throw new Error(`task card not visible; toolNames=${JSON.stringify(toolNames)} diagnostic=${JSON.stringify(diagnostic)} cause=${error.message}`);
  });
  const completedSubagentCalls = await page.evaluate(() => window.__kwuPerfCalls.subagent.length);
  const subagentDuration = await measure(async () => {
    await taskCard.locator(".tool__head").click();
    await page.waitForFunction(
      (count) => window.__kwuPerfCalls.subagent.length > count,
      completedSubagentCalls,
      { timeout: 60_000 },
    );
    await page.getByText("PERF-SUBAGENT-LAST", { exact: true }).waitFor({ state: "visible", timeout: 60_000 });
  });

  await page.screenshot({ path: path.join(outputDirectory, "session-performance.png"), fullPage: false });
  const runtimeMetrics = await page.evaluate(() => ({
    navigation: performance.getEntriesByType("navigation").map((entry) => ({
      domContentLoadedMs: entry.domContentLoadedEventEnd,
      loadEventMs: entry.loadEventEnd,
      responseEndMs: entry.responseEnd,
    })),
    paints: performance.getEntriesByType("paint").map((entry) => ({
      name: entry.name,
      startTime: entry.startTime,
    })),
    longTasks: window.__kwuPerfLongTasks ?? [],
    bridgeCalls: window.__kwuPerfCalls,
    heap: performance.memory ? {
      usedJSHeapSize: performance.memory.usedJSHeapSize,
      totalJSHeapSize: performance.memory.totalJSHeapSize,
      jsHeapSizeLimit: performance.memory.jsHeapSizeLimit,
    } : null,
    reactProfilerCommits: window.__KWU_REACT_PROFILER_COMMITS__ ?? null,
  }));
  const reactCommits = runtimeMetrics.reactProfilerCommits;
  delete runtimeMetrics.reactProfilerCommits;

  process.stdout.write(`${JSON.stringify({
    schemaVersion: 1,
    evidenceScope: "local-production-mode-webview2-wails-ipc-disk-react",
    contextCount: browser.contexts().length,
    pageCount: pages.length,
    page: {
      url: page.url(),
      title: await page.title(),
      language: await page.locator("html").getAttribute("lang"),
      hasWailsRuntime: await page.evaluate(() => typeof window.runtime !== "undefined"),
    },
    catalogCount: sessions.length,
    listAllSessions: summary(listSamples),
    conversationCenterOpen: summary(openSamples),
    activateLongSession: { durationMs: Number(activateDuration.toFixed(2)) },
    loadOlderHistory: summary(olderSamples),
    lazySubagentFirstOpen: { durationMs: Number(subagentDuration.toFixed(2)) },
    runtimeMetrics,
    reactProfilerCommits: Array.isArray(reactCommits)
      ? {
          available: true,
          count: reactCommits.length,
          actualDuration: summary(reactCommits.map((commit) => commit.actualDuration)),
          baseDuration: summary(reactCommits.map((commit) => commit.baseDuration)),
        }
      : {
          available: false,
          reason: "this binary was not built with KWU_REACT_PROFILING=1",
        },
  })}\n`);
} finally {
  await browser.close();
}
