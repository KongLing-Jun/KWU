import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SendAvailabilityBanner } from "../components/SendAvailabilityBanner";
import { LocaleProvider } from "../lib/i18n";

describe("SendAvailabilityBanner", () => {
  it("announces the login recovery state and exposes deterministic actions", async () => {
    const user = userEvent.setup();
    const onLogin = vi.fn();
    const onRetry = vi.fn();
    const onSwitchModel = vi.fn();

    render(
      <LocaleProvider>
        <SendAvailabilityBanner
          availability={{
            state: "subscription_auth_required",
            retryable: true,
            action: "login",
            detail: "auth_required",
          }}
          hasHistory
          onLogin={onLogin}
          onOpenAccount={vi.fn()}
          onOpenPayments={vi.fn()}
          onConfigureBYOK={vi.fn()}
          onRetry={onRetry}
          onSwitchModel={onSwitchModel}
        />
      </LocaleProvider>,
    );

    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(status).toHaveTextContent("登录后使用官方订阅");
    expect(status).toHaveTextContent("继续当前会话");

    await user.click(screen.getByRole("button", { name: "登录" }));
    await user.click(screen.getByRole("button", { name: "切换模型" }));

    expect(onLogin).toHaveBeenCalledOnce();
    expect(screen.queryByRole("button", { name: "重试" })).not.toBeInTheDocument();
    expect(onRetry).not.toHaveBeenCalled();
    expect(onSwitchModel).toHaveBeenCalledOnce();
  });
});
