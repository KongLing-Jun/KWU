package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestWriteFileAuditsUserConfigWrites(t *testing.T) {
	isolateUserConfigHome(t)
	path := UserConfigPath()

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	before := Default()
	before.Providers = append(before.Providers, ProviderEntry{
		Name:    "custom-audit-provider",
		Kind:    "openai",
		BaseURL: "http://127.0.0.1:9999/v1",
		Model:   "custom-audit-model",
	})
	if err := before.SaveToScope(path, RenderScopeUser); err != nil {
		t.Fatalf("write baseline config: %v", err)
	}
	logs.Reset()

	after := Default()
	if err := after.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := logs.String()
	for _, want := range []string{
		"config: write audit",
		"config: possible custom provider loss",
		"custom-audit-provider",
		"TestWriteFileAuditsUserConfigWrites",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("audit log missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "(*Config).WriteFile") {
		t.Fatalf("audit operation should skip WriteFile frame:\n%s", got)
	}
}
