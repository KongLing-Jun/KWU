package boot

import (
	"context"
	"testing"

	"kwu/internal/cloud"
)

func TestBuildExposesMediaUnderstandingAsOptionalTool(t *testing.T) {
	isolateConfigHome(t)
	dir := robustTempDir(t)
	t.Chdir(dir)
	writeFile(t, dir, "kwu.toml", `
default_model = "test-model"

[agent]
system_prompt = "BASE"

[[providers]]
name = "test-model"
kind = "boot-token-profile-test"
model = "x"
`)

	req, _, _ := firstAgentModeRequest(t, Options{
		ModelBillingOptions: ModelBillingOptions{
			MediaUnderstandingClient: bootFakeMediaUnderstandingClient{},
			MediaVideoScope: func(context.Context) (cloud.BindingScope, error) {
				return cloud.BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}, nil
			},
		},
	})
	if !requestHasTool(req, "understand_media") {
		t.Fatalf("provider request missing understand_media; tools=%v", toolSchemaNames(req.Tools))
	}
	if requestToolSchemaContains(req, "understand_media", `"model"`) {
		t.Fatal("understand_media schema unexpectedly exposes model")
	}
	if requestHasTool(req, "generate_image") || requestHasTool(req, "generate_video") || requestHasTool(req, "generate_media") {
		t.Fatal("understanding-only configuration unexpectedly exposed a generation tool")
	}
}

type bootFakeMediaUnderstandingClient struct{}

func (bootFakeMediaUnderstandingClient) UnderstandURL(
	context.Context,
	cloud.BindingScope,
	string,
	cloud.MediaUnderstandingRequest,
) (cloud.MediaUnderstandingResult, error) {
	return cloud.MediaUnderstandingResult{}, nil
}

func (bootFakeMediaUnderstandingClient) UnderstandFile(
	context.Context,
	cloud.BindingScope,
	string,
	string,
	string,
	string,
	[]byte,
) (cloud.MediaUnderstandingResult, error) {
	return cloud.MediaUnderstandingResult{}, nil
}

var _ cloud.MediaUnderstandingClient = bootFakeMediaUnderstandingClient{}
