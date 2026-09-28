package config

func legacyDeepSeekProviderEntries() []ProviderEntry {
	return []ProviderEntry{
		{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY", BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4FlashPrice()},
		{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: "DEEPSEEK_API_KEY", BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: deepSeekV4ProPrice()},
	}
}

func configWithLegacyDeepSeekProviders() *Config {
	c := Default()
	c.DefaultModel = "deepseek-flash"
	c.Providers = legacyDeepSeekProviderEntries()
	return c
}
