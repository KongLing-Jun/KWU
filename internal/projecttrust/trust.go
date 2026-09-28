package projecttrust

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store keeps explicit trust decisions outside project-controlled files. The
// persisted JSON shape and legacy paths remain compatible with earlier releases
// so existing local MCP trust decisions continue to work after hook removal.
type Store struct {
	Path      string
	Fallbacks []string
}

type trustFile struct {
	Projects map[string]bool `json:"projects"`
}

func (s Store) IsTrusted(projectRoot string) bool {
	if strings.TrimSpace(projectRoot) == "" || strings.TrimSpace(s.Path) == "" {
		return false
	}
	return s.read().Projects[absRoot(projectRoot)]
}

func (s Store) Trust(projectRoot string) error {
	if strings.TrimSpace(projectRoot) == "" {
		return nil
	}
	if strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("empty project trust path")
	}
	tf := s.read()
	if tf.Projects == nil {
		tf.Projects = map[string]bool{}
	}
	tf.Projects[absRoot(projectRoot)] = true
	return writeTrustFile(s.Path, tf)
}

func (s Store) read() trustFile {
	var tf trustFile
	body, err := os.ReadFile(s.Path)
	if err == nil {
		_ = json.Unmarshal(body, &tf)
		return tf
	}
	if !os.IsNotExist(err) {
		return tf
	}
	for _, fallback := range s.Fallbacks {
		if strings.TrimSpace(fallback) == "" {
			continue
		}
		body, err = os.ReadFile(fallback)
		if err == nil {
			_ = json.Unmarshal(body, &tf)
			return tf
		}
	}
	return tf
}

func absRoot(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

func writeTrustFile(path string, tf trustFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
