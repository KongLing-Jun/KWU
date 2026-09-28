package bot

func (gw *BotGateway) checkAllowlist(plat Platform, msg InboundMessage) bool {
	if gw.cfg.Allowlist.AllowAll {
		return true
	}
	if !gw.cfg.Allowlist.Enabled {
		return false
	}
	actors := operatorAllowlistCandidates(msg)
	if len(actors) == 0 {
		actors = []string{msg.UserID}
	}
	allowed := false
	for _, actor := range actors {
		if gw.allowlist[plat][actor] {
			allowed = true
			break
		}
	}
	if !allowed {
		return false
	}
	groups := gw.groupAllowlist[plat]
	if chatUsesGroupAllowlist(msg.ChatType) && len(groups) > 0 && !groups[msg.ChatID] {
		return false
	}
	return true
}

func operatorAllowlistCandidates(msg InboundMessage) []string {
	if msg.OperatorID == "" && len(msg.OperatorIDs) == 0 {
		return nil
	}
	return uniqueNonEmptyStrings(append([]string{msg.OperatorID}, msg.OperatorIDs...)...)
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func chatUsesGroupAllowlist(chatType ChatType) bool {
	switch chatType {
	case ChatGroup, ChatGuild, ChatThread:
		return true
	default:
		return false
	}
}
