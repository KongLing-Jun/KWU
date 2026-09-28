// Package brand centralizes product naming and stable runtime identifiers.
package brand

const (
	DisplayName = "云智企办"
	EnglishName = "Clouvia"
	Slug        = "clouvia"

	LegacyDisplayName = "快务AI"
	LegacyEnglishName = "KWU"
	LegacySlug        = "kwu"

	// Runtime environment variables and provider IDs stay on KWU so existing
	// credentials, automation and cloud contracts continue to work unchanged.
	EnvPrefix       = "KWU"
	LegacyEnvPrefix = EnvPrefix
	PathEnvPrefix   = "CLOUVIA"

	OfficialAPIKeyEnv       = EnvPrefix + "_API_KEY"
	LegacyOfficialAPIKeyEnv = LegacyEnvPrefix + "_API_KEY"

	ProjectConfigName         = "clouvia.toml"
	LegacyProjectConfigName   = "kwu.toml"
	SuiYuanProjectConfigName  = "suiyuan.toml"
	ReasonixProjectConfigName = "reasonix.toml"

	ProviderID       = "kwu"
	LegacyProviderID = ProviderID

	DesktopBinaryName       = "kwu-desktop"
	LegacyDesktopBinaryName = "kwu-desktop"

	MemoryFileName         = "CLOUVIA.md"
	LegacyMemoryFileName   = "KWU.md"
	SuiYuanMemoryFileName  = "SUIYUAN.md"
	ReasonixMemoryFileName = "REASONIX.md"

	ProjectDirName         = ".clouvia"
	LegacyProjectDirName   = ".kwu"
	SuiYuanProjectDirName  = ".suiyuan"
	ReasonixProjectDirName = ".reasonix"
)
