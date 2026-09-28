package boot

import (
	"time"

	"kwu/internal/agent"
	"kwu/internal/airclaude"
	"kwu/internal/cloud"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
)

// ModelBillingOptions groups the optional strict B2B composition inputs.
type ModelBillingOptions struct {
	TaskContextInitializer control.TaskContextInitializer
	RequireTaskContext     bool
	BillingRouteSelection  modelbilling.BillingRouteSelection
	// SubscriptionModelCatalog is the authenticated AirClaude catalog captured
	// by Desktop for this build. It is applied only to the in-memory
	// subscription configuration and is never persisted or used by BYOK routes.
	SubscriptionModelCatalog []string
	MainStreamDispatcher     agent.StreamDispatcher
	// MediaToolCredential resolves the currently authorized backend MaaS key
	// for AirClaude image tool calls. It is deliberately dynamic so a
	// controller cannot keep using a credential after logout or account switch.
	MediaToolCredential    airclaude.CredentialSource
	MediaImageQuotaPricing airclaude.ImageQuotaPricingSource
	// MediaVideoClient uses the authenticated Reasonix Cloud session transport;
	// MediaVideoScope resolves the currently authorized account binding.
	MediaVideoClient cloud.VideoGenerationClient
	MediaVideoScope  airclaude.VideoScopeSource
	// MediaUnderstandingClient exposes settled image/video/audio-to-text calls
	// through the authenticated Reasonix Cloud session. It shares the dynamic
	// account scope used by cloud-backed media generation.
	MediaUnderstandingClient  cloud.MediaUnderstandingClient
	ModelBillingTaskStore     modelbilling.TaskStore
	TaskLifecycleRecovery     modelbilling.UsagePendingRecovery
	TaskLifecycleNow          func() time.Time
	ModelBillingCleanup       func()
	RejectBackgroundSubagents bool
}
