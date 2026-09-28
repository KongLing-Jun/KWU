package boot

import (
	"fmt"
	"path/filepath"

	"kwu/internal/config"
	"kwu/internal/skill"
)

func validateManagedSkillPaths(base skill.Options, managedPaths []string) error {
	if len(managedPaths) == 0 {
		return nil
	}
	if err := validateManagedSkillRootPaths(managedPaths); err != nil {
		return err
	}
	disabled := append([]string(nil), base.DisabledNames...)
	base.DisabledNames = nil
	baseNames := skillNames(skill.New(base).List())
	managed := skill.DiscoverManagedPaths(managedPaths, base.MaxDepth)
	if len(managed) == 0 {
		return fmt.Errorf("managed skill paths contain no skills")
	}
	return validateManagedSkillNames(managed, baseNames, disabled)
}

func validateManagedSkillRootPaths(paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		canonical := config.CanonicalSkillPath(path)
		if !filepath.IsAbs(path) || canonical == "" || seen[canonical] {
			return fmt.Errorf("managed skill path is invalid, relative, or duplicated")
		}
		seen[canonical] = true
	}
	return nil
}

func validateManagedSkillNames(managed []skill.Skill, existing map[string]bool, disabled []string) error {
	seen := map[string]bool{}
	disabledNames := make(map[string]bool, len(disabled))
	for _, name := range disabled {
		disabledNames[name] = true
	}
	for _, item := range managed {
		if existing[item.Name] || seen[item.Name] || disabledNames[item.Name] {
			return fmt.Errorf("managed skill %q conflicts with existing skill settings", item.Name)
		}
		seen[item.Name] = true
	}
	return nil
}

func skillNames(skills []skill.Skill) map[string]bool {
	out := make(map[string]bool, len(skills))
	for _, item := range skills {
		out[item.Name] = true
	}
	return out
}
