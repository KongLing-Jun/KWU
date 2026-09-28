import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { HistoryLoadErrorBanner } from "../components/HistoryLoadErrorBanner";
import { LocaleProvider } from "../lib/i18n";

describe("HistoryLoadErrorBanner", () => {
  it("surfaces a fail-closed alert and keeps diagnostics behind disclosure", async () => {
    const user = userEvent.setup();
    render(
      <LocaleProvider>
        <HistoryLoadErrorBanner error="synthetic disk detail" />
      </LocaleProvider>,
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("未切换为空白会话");

    const details = alert.querySelector("details");
    expect(details).not.toHaveAttribute("open");
    await user.click(screen.getByText("错误详情"));
    expect(details).toHaveAttribute("open");
    expect(screen.getByText("synthetic disk detail")).toBeVisible();
  });
});
