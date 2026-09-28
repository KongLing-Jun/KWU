package project

import (
	"fmt"
	"strings"
)

func validateProjectMembers(items []ProjectMember, projectID, currentUserID string) ([]ProjectMember, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("projects-v2: active member response is empty")
	}
	seen := make(map[string]struct{}, len(items))
	ownerCount := 0
	currentMemberFound := false
	for _, item := range items {
		if err := validateActiveProjectMember(item, projectID); err != nil {
			return nil, err
		}
		if _, exists := seen[item.UserID]; exists {
			return nil, fmt.Errorf("projects-v2: duplicate member response")
		}
		seen[item.UserID] = struct{}{}
		if item.Role == "owner" {
			ownerCount++
		}
		if item.UserID == currentUserID {
			currentMemberFound = true
		}
	}
	if ownerCount != 1 || !currentMemberFound {
		return nil, fmt.Errorf("projects-v2: member response is missing required relationships")
	}
	return items, nil
}

func validateActiveProjectMember(item ProjectMember, projectID string) error {
	if !validProjectMemberIdentity(item, projectID) {
		return fmt.Errorf("projects-v2: invalid member identity")
	}
	if !validActiveMemberRelationship(item) {
		return fmt.Errorf("projects-v2: invalid active member relationship")
	}
	if !validActiveMemberFields(item) {
		return fmt.Errorf("projects-v2: invalid active member fields")
	}
	if err := validateProjectTimestamps(item.CreatedAt, item.UpdatedAt); err != nil {
		return fmt.Errorf("projects-v2: invalid active member timestamps: %w", err)
	}
	return nil
}

func validProjectMemberIdentity(item ProjectMember, projectID string) bool {
	return item.ProjectID == projectID && validateOpaqueProjectID(item.UserID) == nil
}

func validActiveMemberRelationship(item ProjectMember) bool {
	return (item.Role == "owner" || item.Role == "member") && item.Status == "active"
}

func validActiveMemberFields(item ProjectMember) bool {
	return item.Version > 0 && item.Version <= MaxSafeJSONInteger &&
		strings.TrimSpace(item.DisplayName) != "" && validProjectText(item.DisplayName, 255)
}
