package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const currentUserProjectCapabilityMaxBytes = 16 * 1024
const currentUserProjectsV1CapabilityMaxBytes = currentUserProjectCapabilityMaxBytes

type CurrentUserCapabilities struct {
	ProjectsV1 json.RawMessage `json:"projects_v1,omitempty"`
	ProjectsV2 json.RawMessage `json:"projects_v2,omitempty"`
}

type CurrentSubscription struct {
	Plan      string
	Status    SubscriptionStatus
	PeriodEnd time.Time
	Version   uint64
}

type CurrentUserSnapshot struct {
	User         CloudUser
	Balance      BalanceSnapshot
	Subscription CurrentSubscription
	Capabilities CurrentUserCapabilities
	ServerTime   time.Time
}

func validateCurrentUserCapabilities(capabilities CurrentUserCapabilities) error {
	if err := validateCurrentUserProjectCapability(capabilities.ProjectsV1, "projects_v1"); err != nil {
		return err
	}
	return validateCurrentUserProjectCapability(capabilities.ProjectsV2, "projects_v2")
}

func validateCurrentUserProjectCapability(raw json.RawMessage, name string) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > currentUserProjectCapabilityMaxBytes {
		return fmt.Errorf("cloud: %s capability exceeds %d bytes", name, currentUserProjectCapabilityMaxBytes)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("cloud: %s capability is invalid JSON", name)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := requireCapabilityObject(decoder, name); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return capabilityTrailingError(err, name)
	}
	return nil
}

func requireCapabilityObject(decoder *json.Decoder, name string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("cloud: decode %s capability: %w", name, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return fmt.Errorf("cloud: %s capability must be an object", name)
	}
	return scanCurrentUserCapabilityObject(decoder, name)
}

func capabilityTrailingError(err error, name string) error {
	if err == nil {
		return fmt.Errorf("cloud: %s capability has multiple JSON values", name)
	}
	return fmt.Errorf("cloud: decode %s capability trailing JSON: %w", name, err)
}

func scanCurrentUserCapabilityObject(decoder *json.Decoder, name string) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		key, err := currentUserCapabilityKey(decoder, seen, name)
		if err != nil {
			return err
		}
		seen[key] = struct{}{}
		if err := scanCurrentUserCapabilityValue(decoder, name); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("cloud: decode %s capability: %w", name, err)
	}
	return nil
}

func currentUserCapabilityKey(decoder *json.Decoder, seen map[string]struct{}, name string) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", fmt.Errorf("cloud: decode %s capability: %w", name, err)
	}
	key, ok := token.(string)
	if !ok {
		return "", fmt.Errorf("cloud: %s capability key is not a string", name)
	}
	if _, duplicate := seen[key]; duplicate {
		return "", fmt.Errorf("cloud: %s capability contains duplicate key %q", name, key)
	}
	return key, nil
}

func scanCurrentUserCapabilityValue(decoder *json.Decoder, name string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("cloud: decode %s capability: %w", name, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter == '{' {
		return scanCurrentUserCapabilityObject(decoder, name)
	}
	if delimiter != '[' {
		return fmt.Errorf("cloud: invalid %s capability delimiter", name)
	}
	for decoder.More() {
		if err := scanCurrentUserCapabilityValue(decoder, name); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("cloud: decode %s capability: %w", name, err)
	}
	return nil
}

func normalizeCreateUserExpertAvatar(fileName, contentType *string, avatar []byte) error {
	if len(avatar) == 0 {
		return fmt.Errorf("expert avatar file is required")
	}
	if len(avatar) > userExpertAvatarMaxBytes {
		return fmt.Errorf("expert avatar file exceeds 5 MiB")
	}
	detectedType, extension, err := detectExpertAvatarType(avatar)
	if err != nil {
		return err
	}
	declaredType := strings.TrimSpace(*contentType)
	if declaredType != "" && declaredType != detectedType {
		return fmt.Errorf("expert avatar content type does not match file contents")
	}
	baseName := path.Base(strings.ReplaceAll(strings.TrimSpace(*fileName), "\\", "/"))
	stem := strings.TrimSuffix(baseName, path.Ext(baseName))
	if stem == "" || stem == "." || len(stem) > 200 {
		stem = "avatar"
	}
	*fileName = stem + extension
	*contentType = detectedType
	return nil
}

func detectExpertAvatarType(avatar []byte) (string, string, error) {
	switch detectedType := http.DetectContentType(avatar); detectedType {
	case "image/png":
		return detectedType, ".png", nil
	case "image/jpeg":
		return detectedType, ".jpg", nil
	case "image/webp":
		return detectedType, ".webp", nil
	default:
		return "", "", fmt.Errorf("expert avatar must be PNG, JPEG, or WebP")
	}
}

func normalizeCreateUserExpertAssetInput(input *CreateUserExpertAssetInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Category = strings.TrimSpace(input.Category)
	if err := normalizeCreateUserExpertAvatar(&input.AvatarFileName, &input.AvatarContentType, input.Avatar); err != nil {
		return err
	}
	if input.Name == "" || len(input.Name) > 256 {
		return fmt.Errorf("expert name must contain between 1 and 256 bytes")
	}
	if input.Prompt == "" || len(input.Prompt) > 256*1024 {
		return fmt.Errorf("expert prompt must contain between 1 and 262144 bytes")
	}
	if input.Category == "" {
		return fmt.Errorf("expert category is required")
	}
	if len([]byte(input.Description)) > userExpertDescriptionMaxBytes {
		return fmt.Errorf("expert description exceeds %d bytes", userExpertDescriptionMaxBytes)
	}
	tags, err := normalizeUserExpertTags(input.Tags, true)
	if err != nil {
		return err
	}
	input.Tags = tags
	return nil
}

func normalizeCreateUserExpertGroupAssetInput(input *CreateUserExpertGroupAssetInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Category = strings.TrimSpace(input.Category)
	if err := normalizeCreateUserExpertAvatar(&input.AvatarFileName, &input.AvatarContentType, input.Avatar); err != nil {
		return err
	}
	if input.Name == "" || len(input.Name) > 256 {
		return fmt.Errorf("expert group name must contain between 1 and 256 bytes")
	}
	if input.Category == "" {
		return fmt.Errorf("expert group category is required")
	}
	if len([]byte(input.Description)) > userExpertDescriptionMaxBytes {
		return fmt.Errorf("expert group description exceeds %d bytes", userExpertDescriptionMaxBytes)
	}
	tags, err := normalizeUserExpertTags(input.Tags, true)
	if err != nil {
		return err
	}
	input.Tags = tags
	expertIDs, err := normalizeCreateExpertGroupIDs(input.ExpertIDs)
	if err != nil {
		return err
	}
	input.ExpertIDs = expertIDs
	return nil
}

func normalizeCreateExpertGroupIDs(ids []string) ([]string, error) {
	if ids == nil {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(ids))
	for index, rawID := range ids {
		expertID := strings.TrimSpace(rawID)
		if expertID == "" {
			return nil, fmt.Errorf("expert_ids[%d] must not be empty", index)
		}
		if _, exists := seen[expertID]; exists {
			return nil, fmt.Errorf("expert_ids must not contain duplicates")
		}
		seen[expertID] = struct{}{}
		ids[index] = expertID
	}
	return ids, nil
}

func normalizeUserExpertTags(tags []string, allowEmpty bool) ([]string, error) {
	if len(tags) == 0 && allowEmpty {
		return nil, nil
	}
	if len(tags) != userExpertTagCount {
		return nil, fmt.Errorf("expert tags must contain exactly %d values", userExpertTagCount)
	}
	normalized := make([]string, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for index, rawTag := range tags {
		tag := strings.TrimSpace(rawTag)
		if tag == "" || len([]byte(tag)) > userExpertTagMaxBytes {
			return nil, fmt.Errorf("expert tag %d is required and must not exceed %d bytes", index+1, userExpertTagMaxBytes)
		}
		if _, duplicate := seen[tag]; duplicate {
			return nil, fmt.Errorf("expert tags must not contain duplicates")
		}
		seen[tag] = struct{}{}
		normalized[index] = tag
	}
	return normalized, nil
}

func normalizeCreatedUserExpertAsset(asset *UserExpertAsset) error {
	asset.ExpertID = strings.TrimSpace(asset.ExpertID)
	asset.AvatarURL = strings.TrimSpace(asset.AvatarURL)
	asset.Name = strings.TrimSpace(asset.Name)
	asset.Description = strings.TrimSpace(asset.Description)
	asset.Prompt = strings.TrimSpace(asset.Prompt)
	asset.Category = strings.TrimSpace(asset.Category)
	if asset.ExpertID == "" || asset.Name == "" || asset.Prompt == "" || asset.Category == "" ||
		asset.Type != "user" || asset.OwnerUserID == nil || strings.TrimSpace(*asset.OwnerUserID) == "" {
		return fmt.Errorf("create user expert response is incomplete")
	}
	tags, err := normalizeUserExpertTags(asset.Tags, false)
	if err != nil {
		return fmt.Errorf("create user expert response has invalid tags: %w", err)
	}
	asset.Tags = tags
	return validateSystemExpertAvatarURL(asset.AvatarURL)
}

func normalizeCreatedUserExpertGroupAsset(asset *UserExpertGroupAsset) error {
	asset.ExpertGroupID = strings.TrimSpace(asset.ExpertGroupID)
	asset.AvatarURL = strings.TrimSpace(asset.AvatarURL)
	asset.Name = strings.TrimSpace(asset.Name)
	asset.Description = strings.TrimSpace(asset.Description)
	asset.Category = strings.TrimSpace(asset.Category)
	if asset.ExpertGroupID == "" || asset.Name == "" || asset.Category == "" ||
		asset.Type != "user" || asset.OwnerUserID == nil || strings.TrimSpace(*asset.OwnerUserID) == "" {
		return fmt.Errorf("create user expert group response is incomplete")
	}
	if err := validateSystemExpertAvatarURL(asset.AvatarURL); err != nil {
		return err
	}
	tags, err := normalizeUserExpertTags(asset.Tags, false)
	if err != nil {
		return fmt.Errorf("create user expert group response has invalid tags: %w", err)
	}
	asset.Tags = tags
	expertIDs, err := normalizeCreatedExpertGroupIDs(asset.ExpertIDs)
	if err != nil {
		return err
	}
	asset.ExpertIDs = expertIDs
	return nil
}

func normalizeCreatedExpertGroupIDs(ids []string) ([]string, error) {
	if ids == nil {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(ids))
	for index, rawID := range ids {
		expertID := strings.TrimSpace(rawID)
		if expertID == "" {
			return nil, fmt.Errorf("created expert group contains an empty expert id")
		}
		if _, exists := seen[expertID]; exists {
			return nil, fmt.Errorf("created expert group contains duplicate expert id %q", expertID)
		}
		seen[expertID] = struct{}{}
		ids[index] = expertID
	}
	return ids, nil
}
