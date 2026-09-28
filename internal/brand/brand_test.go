package brand

import "testing"

func TestBrandConstants(t *testing.T) {
	if DisplayName != "云智企办" || EnglishName != "Clouvia" || Slug != "clouvia" {
		t.Fatalf("visible brand = %q/%q/%q", DisplayName, EnglishName, Slug)
	}
	if LegacyDisplayName != "快务AI" || LegacyEnglishName != "KWU" || LegacySlug != "kwu" {
		t.Fatalf("legacy brand = %q/%q/%q", LegacyDisplayName, LegacyEnglishName, LegacySlug)
	}
	if EnvPrefix != "KWU" || LegacyEnvPrefix != EnvPrefix || PathEnvPrefix != "CLOUVIA" {
		t.Fatalf("env prefixes = %q/%q/%q", EnvPrefix, LegacyEnvPrefix, PathEnvPrefix)
	}
	if ProjectConfigName != "clouvia.toml" || LegacyProjectConfigName != "kwu.toml" ||
		SuiYuanProjectConfigName != "suiyuan.toml" || ReasonixProjectConfigName != "reasonix.toml" {
		t.Fatalf("config names = %q/%q/%q/%q", ProjectConfigName, LegacyProjectConfigName, SuiYuanProjectConfigName, ReasonixProjectConfigName)
	}
	if ProviderID != "kwu" || LegacyProviderID != ProviderID || DesktopBinaryName != "kwu-desktop" {
		t.Fatalf("stable runtime IDs changed: provider=%q/%q binary=%q", ProviderID, LegacyProviderID, DesktopBinaryName)
	}
	if MemoryFileName != "CLOUVIA.md" || LegacyMemoryFileName != "KWU.md" ||
		SuiYuanMemoryFileName != "SUIYUAN.md" || ReasonixMemoryFileName != "REASONIX.md" {
		t.Fatalf("memory file names = %q/%q/%q/%q", MemoryFileName, LegacyMemoryFileName, SuiYuanMemoryFileName, ReasonixMemoryFileName)
	}
	if ProjectDirName != ".clouvia" || LegacyProjectDirName != ".kwu" ||
		SuiYuanProjectDirName != ".suiyuan" || ReasonixProjectDirName != ".reasonix" {
		t.Fatalf("project dirs = %q/%q/%q/%q", ProjectDirName, LegacyProjectDirName, SuiYuanProjectDirName, ReasonixProjectDirName)
	}
}
