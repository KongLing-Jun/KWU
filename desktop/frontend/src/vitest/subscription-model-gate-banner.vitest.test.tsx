import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SubscriptionModelGateBanner } from "../components/SubscriptionModelGateBanner";
import { LocaleProvider } from "../lib/i18n";

describe("SubscriptionModelGateBanner", () => {
  it("stays absent while subscription model access is loading", () => {
    const { container } = render(
      <LocaleProvider>
        <SubscriptionModelGateBanner access={{ state: "loading" }} modelCatalog={{ state: "loading" }} onSwitchModel={vi.fn()} />
      </LocaleProvider>,
    );

    expect(container.querySelector("[data-subscription-model-gate]")).not.toBeInTheDocument();
    expect(screen.queryByText(/Checking whether this subscription model is available/)).not.toBeInTheDocument();
  });

  it("keeps MCP available and offers model management when no verified model exists", async () => {
    const user = userEvent.setup();
    const onOpenModelPicker = vi.fn();
    const { container } = render(
      <LocaleProvider>
        <SubscriptionModelGateBanner access={{ state: "blocked" }} modelCatalog={{ state: "unavailable" }} onSwitchModel={vi.fn()} onOpenModelPicker={onOpenModelPicker} />
      </LocaleProvider>,
    );

    expect(container.querySelector('[data-subscription-model-gate="blocked"]')).toBeInTheDocument();
    expect(screen.getByText(/MCP 配置不受影响/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "管理模型" }));
    expect(onOpenModelPicker).toHaveBeenCalledOnce();
  });

  it("offers only models authorized by the shared catalog for the active Free account", () => {
    render(
      <LocaleProvider>
        <SubscriptionModelGateBanner
          access={{ state: "blocked" }}
          modelCatalog={{
            state: "ready",
            routes: [
              { ref: "kwu/gpt-5.4", provider: "airclaude_subscription", model: "gpt-5.4", billingSource: "subscription_pool", current: true },
              { ref: "kwu/claude-fable-5", provider: "airclaude_subscription", model: "claude-fable-5", billingSource: "subscription_pool", current: false },
              { ref: "kwu/qwen3.7-plus", provider: "airclaude_subscription", model: "qwen3.7-plus", billingSource: "subscription_pool", current: false },
            ],
            marketplace: {
              sources: [{
                id: "international",
                label: "International",
                sourceUrl: "https://pricing.example.invalid/api/pricing",
                pricingUrl: "https://pricing.example.invalid/models",
                pricingState: "fresh",
                metadataStatus: "ready",
                models: [
                  { id: "gpt-5.4", name: "gpt-5.4", tags: ["Subscribe_Pro"] },
                  { id: "claude-fable-5", name: "claude-fable-5", tags: ["Subscribe_Pro"] },
                  { id: "qwen3.7-plus", name: "qwen3.7-plus", tags: ["Subscribe_Free"] },
                ],
              }],
            },
          }}
          subscriptionAccess={{ state: "ready", plan: "free", requiredTag: "Subscribe_Free" }}
          onSwitchModel={vi.fn()}
        />
      </LocaleProvider>,
    );

    expect(screen.getByRole("button", { name: /qwen3\.7-plus/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /gpt-5\.4/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /claude-fable-5/ })).not.toBeInTheDocument();
  });
});
