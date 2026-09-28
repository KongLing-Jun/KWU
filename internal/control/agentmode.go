package control

import "kwu/internal/agentmode"

func defaultAgentMode(mode agentmode.Mode) agentmode.Mode {
	if mode == "" {
		return agentmode.ModeNormal
	}
	return mode
}

func defaultAgentModeSource(source agentmode.ModeSourceKind) agentmode.ModeSourceKind {
	if source == "" {
		return agentmode.ModeSourceDefault
	}
	return source
}

// AgentMode returns the effective high-level handling mode for this session.
func (c *Controller) AgentMode() agentmode.Mode {
	if c == nil {
		return agentmode.ModeNormal
	}
	return defaultAgentMode(c.agentMode)
}

// AgentModeSource returns which mode source won during boot resolution.
func (c *Controller) AgentModeSource() agentmode.ModeSourceKind {
	if c == nil {
		return agentmode.ModeSourceDefault
	}
	return defaultAgentModeSource(c.agentModeSrc)
}
