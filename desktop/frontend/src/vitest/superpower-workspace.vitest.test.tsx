import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SuperpowerWorkspace, type SuperpowerSessionAdapter } from "../features/superpowers";

const sessionAdapter: SuperpowerSessionAdapter = {
  ready: true,
  running: false,
  messages: [],
  assistantMessages: [],
  submit: vi.fn(() => "submitted" as const),
};

function renderEcommerceWorkspace() {
  const renderSessionTranscript = vi.fn(() => <div data-testid="ordinary-session-transcript">普通会话记录</div>);
  const result = render(
    <SuperpowerWorkspace
      initialScreen="ecommerce"
      sessionAdapter={sessionAdapter}
      renderSessionTranscript={renderSessionTranscript}
      renderOperationalPrompt={() => <div data-testid="ordinary-session-prompt">等待审批</div>}
      renderComposer={() => <textarea aria-label="普通会话输入框" />}
      renderSessionStatusBar={() => <div data-testid="ordinary-session-status">普通会话状态</div>}
    />,
  );
  return { result, renderSessionTranscript };
}

describe("SuperpowerWorkspace", () => {
  it("keeps the catalog as the first level and opens the ecommerce workspace", () => {
    render(<SuperpowerWorkspace />);

    expect(screen.getByRole("heading", { name: "选择你的超能力" })).toBeVisible();
    expect(screen.getByRole("button", { name: "打开跨境电商超能力" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "经营分析超能力，即将上线" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "打开跨境电商超能力" }));
    expect(screen.getByRole("region", { name: "跨境电商超能力工作台" })).toBeVisible();
  });

  it("reuses the ordinary session surface and keeps only the dashboard sidebar specialized", () => {
    const { renderSessionTranscript } = renderEcommerceWorkspace();

    expect(screen.getByTestId("ordinary-session-transcript")).toBeVisible();
    expect(screen.getByTestId("ordinary-session-prompt")).toBeVisible();
    expect(screen.getByRole("textbox", { name: "普通会话输入框" })).toBeVisible();
    expect(screen.getByTestId("ordinary-session-status")).toBeVisible();
    expect(screen.getByRole("complementary", { name: "AI 研究组件画布" })).toBeVisible();
    expect(screen.queryByText("便携式榨汁机市场进入研究")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("会话历史")).not.toBeInTheDocument();
    expect(renderSessionTranscript).toHaveBeenCalledWith(expect.objectContaining({
      welcomePresentation: expect.objectContaining({
        label: "跨境电商超能力",
        title: "从市场线索到可验证的经营判断",
      }),
      onWelcomePrompt: expect.any(Function),
    }));
  });

  it("keeps the specialized dashboard sidebar permanently visible", () => {
    renderEcommerceWorkspace();

    expect(screen.getByRole("complementary", { name: "AI 研究组件画布" })).toBeVisible();
    expect(screen.queryByRole("button", { name: "收起研究画布" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "展开研究画布" })).not.toBeInTheDocument();
  });
});
