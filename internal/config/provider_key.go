package config

import (
	"net/netip"
	"net/url"
	"os"
	"strings"
)

func processEnvCredentialValue(key string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	for _, alias := range credentialKeyAliases(key) {
		if alias == key {
			continue
		}
		if value := strings.TrimSpace(os.Getenv(alias)); value != "" {
			return value
		}
	}
	return ""
}

func providerBaseURLRequiresAPIKey(raw string) bool {
	switch officialProviderHost(raw) {
	case "api.deepseek.com", "api.xiaomimimo.com", "token-plan-cn.xiaomimimo.com", "api.minimaxi.com", "api.openai.com", "api.airclaudeai.com", "192.168.10.231":
		return true
	default:
		return false
	}
}

func providerBaseURLAllowsMissingAPIKey(raw string) bool {
	if providerBaseURLRequiresAPIKey(raw) {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.Trim(strings.ToLower(u.Hostname()), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
}
