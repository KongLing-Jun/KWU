package config

import "testing"

func TestDirectProxyHostsFromNoProxyProviders(t *testing.T) {
	c := Default()
	c.Providers = append(c.Providers, ProviderEntry{Name: "domestic", Kind: "openai", BaseURL: "https://domestic.example/v1", Model: "chat", NoProxy: true})
	spec := c.OutboundProxySpec()
	hasDirectHost := false
	for _, h := range spec.DirectHosts {
		if h == "domestic.example" {
			hasDirectHost = true
		}
		if h == "api.deepseek.com" {
			t.Errorf("DeepSeek works through the proxy and must not be forced direct: %v", spec.DirectHosts)
		}
	}
	if !hasDirectHost {
		t.Errorf("a no_proxy provider's host should land in DirectHosts, got %v", spec.DirectHosts)
	}
}
