package boot

import (
	"net/http"

	"kwu/internal/airclaude"
	"kwu/internal/cloud"
	"kwu/internal/tool"
)

func addCurrentDatetimeAndMediaTool(
	registry *tool.Registry,
	currentDatetime tool.Tool,
	credential airclaude.CredentialSource,
	imageQuotaPricing airclaude.ImageQuotaPricingSource,
	videoClient cloud.VideoGenerationClient,
	understandingClient cloud.MediaUnderstandingClient,
	videoScope airclaude.VideoScopeSource,
	client *http.Client,
	outputDir string,
	workspaceRoot string,
) {
	registry.Add(currentDatetime)
	if credential != nil {
		registry.Add(airclaude.NewDefaultImageTool(credential, imageQuotaPricing, client, outputDir))
	}
	if videoClient != nil && videoScope != nil {
		registry.Add(airclaude.NewDefaultVideoTool(videoClient, videoScope, client, outputDir))
	}
	if understandingClient != nil && videoScope != nil {
		if mediaUnderstanding, err := airclaude.NewMediaUnderstandingTool(understandingClient, videoScope, workspaceRoot); err == nil {
			registry.Add(mediaUnderstanding)
		}
	}
}
