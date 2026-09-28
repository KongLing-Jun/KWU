package expert

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserStoreReadPreservesErrorPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		missing    bool
	}{
		{name: "missing", missing: true},
		{name: "empty"},
		{name: "malformed", body: "{"},
		{name: "wrong-schema", body: `{"version":2,"experts":"invalid"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "experts.json")
			if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			records, err := LoadUserExperts(path)
			if tc.missing {
				if err != nil || len(records) != 0 {
					t.Fatalf("missing store = %v, %v", records, err)
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("read created missing store: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "parse user experts:") {
				t.Fatalf("parse error = %v", err)
			}
			if err := SaveUserExperts(path, nil); err == nil {
				t.Fatal("save ignored a failed initial read")
			}
			body, err := os.ReadFile(path)
			if err != nil || string(body) != tc.body {
				t.Fatalf("invalid store overwritten: %q, %v", body, err)
			}
		})
	}
}
