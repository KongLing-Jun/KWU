package config

import (
	"strings"
	"testing"
)

func TestWeComConnectionPersistsOnlyProfileCredentialReference(t *testing.T) {
	cfg := Default()
	cfg.Bot.Connections = []BotConnectionConfig{{
		ID: "wecom-team", Provider: "wecom", Domain: "wecom", Enabled: true, Status: "connected",
		Credential: BotConnectionCredential{TokenEnv: "WECOM_TEAM_WEBHOOK_KEY"},
	}}
	rendered := RenderTOML(cfg)
	for _, want := range []string{`provider = "wecom"`, `domain = "wecom"`, `token_env = "WECOM_TEAM_WEBHOOK_KEY"`} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered config missing %s", want)
		}
	}
	for _, forbidden := range []string{"qyapi.weixin.qq.com", "cgi-bin/webhook/send", "private-webhook-key"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("rendered config contains webhook secret material %q", forbidden)
		}
	}
}
