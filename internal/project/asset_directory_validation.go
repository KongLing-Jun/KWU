package project

import (
	"fmt"
	"strings"
)

func decodeProjectAssetDirectoryPage(payload []byte, expectedProjectID, expectedFolderID string) (ProjectAssetDirectoryPage, error) {
	var page ProjectAssetDirectoryPage
	if err := decodeV2Payload(payload, &page, "project_id", "folder_id", "breadcrumbs", "items", "total_count", "next_cursor", "usage_bytes", "quota_bytes", "quota_plan"); err != nil {
		return ProjectAssetDirectoryPage{}, err
	}
	if page.Breadcrumbs == nil {
		page.Breadcrumbs = []ProjectAssetDirectoryBreadcrumb{}
	}
	if page.Items == nil {
		page.Items = []ProjectAssetDirectoryItem{}
	}
	if err := validateProjectAssetDirectoryPage(page, expectedProjectID, expectedFolderID); err != nil {
		return ProjectAssetDirectoryPage{}, err
	}
	if err := validateProjectAssetDirectoryBreadcrumbs(page.Breadcrumbs); err != nil {
		return ProjectAssetDirectoryPage{}, err
	}
	for index := range page.Items {
		if err := validateProjectAssetDirectoryItem(page.Items[index]); err != nil {
			return ProjectAssetDirectoryPage{}, err
		}
		page.Items[index].UpdatedAt = page.Items[index].UpdatedAt.UTC()
	}
	return page, nil
}

func validateProjectAssetDirectoryPage(page ProjectAssetDirectoryPage, expectedProjectID, expectedFolderID string) error {
	if invalidProjectAssetDirectoryIdentity(page, expectedProjectID, expectedFolderID) ||
		invalidProjectAssetDirectoryBounds(page) ||
		(page.NextCursor != nil && validateWorkItemListCursor(*page.NextCursor) != nil) {
		return fmt.Errorf("projects-v2: invalid asset directory response")
	}
	return nil
}

func invalidProjectAssetDirectoryIdentity(page ProjectAssetDirectoryPage, expectedProjectID, expectedFolderID string) bool {
	if page.ProjectID != expectedProjectID {
		return true
	}
	if expectedFolderID == "" {
		return page.FolderID != nil
	}
	return page.FolderID == nil || *page.FolderID != expectedFolderID || validateOpaqueProjectID(*page.FolderID) != nil
}

func invalidProjectAssetDirectoryBounds(page ProjectAssetDirectoryPage) bool {
	return page.TotalCount > MaxSafeJSONInteger || uint64(len(page.Items)) > page.TotalCount ||
		page.UsageBytes < 0 || page.QuotaBytes < 0 || strings.TrimSpace(page.QuotaPlan) == ""
}

func validateProjectAssetDirectoryBreadcrumbs(items []ProjectAssetDirectoryBreadcrumb) error {
	for _, item := range items {
		if validateOpaqueProjectID(item.FolderID) != nil || strings.TrimSpace(item.Name) == "" || !validProjectText(item.Name, 255) {
			return fmt.Errorf("projects-v2: invalid asset directory breadcrumb")
		}
	}
	return nil
}

func validateProjectAssetDirectoryItem(item ProjectAssetDirectoryItem) error {
	if err := validateProjectAssetDirectoryItemBase(item); err != nil {
		return err
	}
	if err := validateProjectAssetDirectoryUpdater(item.UpdatedBy); err != nil {
		return err
	}
	if err := validateProjectAssetDirectoryParents(item); err != nil {
		return err
	}
	return validateProjectAssetDirectoryItemShape(item)
}

func validateProjectAssetDirectoryItemBase(item ProjectAssetDirectoryItem) error {
	if !oneOf(item.Kind, "folder", "asset") || validateOpaqueProjectID(item.ID) != nil || strings.TrimSpace(item.Name) == "" || !validProjectText(item.Name, 255) || item.Version == 0 || item.Version > MaxSafeJSONInteger || item.UpdatedAt.IsZero() {
		return fmt.Errorf("projects-v2: invalid asset directory item")
	}
	return nil
}

func validateProjectAssetDirectoryUpdater(item ProjectAssetDirectoryUpdatedBy) error {
	if validateOpaqueProjectID(item.UserID) != nil || strings.TrimSpace(item.DisplayName) == "" || !validProjectText(item.DisplayName, 255) {
		return fmt.Errorf("projects-v2: invalid asset directory updater")
	}
	return nil
}

func validateProjectAssetDirectoryParents(item ProjectAssetDirectoryItem) error {
	for _, value := range []*string{item.FolderID, item.ParentFolderID} {
		if value != nil && validateOpaqueProjectID(*value) != nil {
			return fmt.Errorf("projects-v2: invalid asset directory parent")
		}
	}
	return nil
}

func validateProjectAssetDirectoryItemShape(item ProjectAssetDirectoryItem) error {
	if item.Kind == "folder" && (item.ContentType != nil || item.SizeBytes != nil || item.FolderID != nil) {
		return fmt.Errorf("projects-v2: invalid asset folder response")
	}
	if item.Kind == "asset" && (item.ContentType == nil || item.SizeBytes == nil || *item.SizeBytes < 0 || item.ParentFolderID != nil) {
		return fmt.Errorf("projects-v2: invalid asset item response")
	}
	return nil
}

func validateProjectAssetName(value string) error {
	if strings.TrimSpace(value) == "" || !validProjectText(value, 255) {
		return fmt.Errorf("projects-v2: invalid asset name")
	}
	return nil
}

func validateAssetDirectoryIDs(projectID, resourceID string) error {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	return validateOpaqueProjectID(resourceID)
}

func validateAssetDirectoryMutation(projectID, resourceID string, version uint64) error {
	if err := validateAssetDirectoryIDs(projectID, resourceID); err != nil {
		return err
	}
	if version == 0 || version > MaxExpectedVersion {
		return fmt.Errorf("projects-v2: asset expected_version must be between 1 and %d", MaxExpectedVersion)
	}
	return nil
}
