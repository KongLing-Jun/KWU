package bot

import "kwu/internal/userctx"

const (
	botServiceActorID = "bot-service"
	botAuthMethod     = "bot"
)

func ServiceActorContext(owner userctx.ActorContext, workspaceRoot string) userctx.ActorContext {
	owner = owner.Normalize()
	return userctx.ActorContext{
		ActorID:       botServiceActorID,
		ProfileID:     owner.ProfileID,
		AuthMethod:    botAuthMethod,
		WorkspaceRoot: workspaceRoot,
	}.Normalize()
}

func ActorContextForInbound(msg InboundMessage, workspaceRoot string) userctx.ActorContext {
	return userctx.ActorContext{
		ActorID:       botServiceActorID,
		ProfileID:     userctx.DefaultProfileID,
		AuthMethod:    botAuthMethod,
		WorkspaceRoot: workspaceRoot,
		RemoteIdentity: &userctx.RemoteIdentity{
			Provider: string(msg.Platform),
			Domain:   msg.Domain,
			UserID:   msg.UserID,
			ChatID:   msg.ChatID,
			ThreadID: msg.ThreadID,
		},
	}.Normalize()
}
