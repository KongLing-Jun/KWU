package bot

import (
	"context"
	"time"

	"kwu/internal/boot"
	"kwu/internal/control"
)

// ControllerBuilder constructs one controller for a remote bot session.
type ControllerBuilder func(context.Context, boot.Options) (*control.Controller, error)

func (gw *BotGateway) buildController(ctx context.Context, options boot.Options) (*control.Controller, error) {
	buildController := gw.cfg.ControllerBuilder
	if buildController == nil {
		buildController = boot.Build
	}
	return buildController(ctx, options)
}

// defaultBotApprovalTimeout caps how long a bot session waits for a remote
// user's approval/ask reply before treating it as denied, so an abandoned
// prompt (or a dropped IM event) can't leave the session wedged forever
// (#4626, #4402). 30 minutes is generous for a human reply yet bounded.
const defaultBotApprovalTimeout = 30 * time.Minute

// approvalTimeout resolves the configured bot approval wait: zero uses the
// bounded default; a negative value opts out (wait indefinitely).
func (gw *BotGateway) approvalTimeout() time.Duration {
	switch {
	case gw.cfg.ApprovalTimeout < 0:
		return 0
	case gw.cfg.ApprovalTimeout == 0:
		return defaultBotApprovalTimeout
	default:
		return gw.cfg.ApprovalTimeout
	}
}
