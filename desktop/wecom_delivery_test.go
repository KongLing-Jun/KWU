package main

import "testing"

func TestNormalizeBotInstallTargetDistinguishesWeComAndWeixinILink(t *testing.T) {
	provider, domain := normalizeBotInstallTarget("wecom", "")
	if provider != "wecom" || domain != "wecom" {
		t.Fatalf("wecom normalized to %q/%q", provider, domain)
	}
	provider, domain = normalizeBotInstallTarget("weixin", "")
	if provider != "weixin" || domain != "weixin" {
		t.Fatalf("weixin normalized to %q/%q", provider, domain)
	}
}
