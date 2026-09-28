package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strings"
	"time"
)

const systemExpertAssetsPath = "/api/v1/expert-assets/system"
const userExpertsPath = "/api/v1/experts"
const userExpertGroupsPath = "/api/v1/expert-groups"
const userExpertAssetsPageLimit = 20
const userExpertAssetsMaxPages = 1000
const userExpertAvatarMaxBytes = 5 * 1024 * 1024
const userExpertDescriptionMaxBytes = 1024
const userExpertTagCount = 3
const userExpertTagMaxBytes = 48

type SystemExpertAsset struct {
	ExpertID    string    `json:"expert_id"`
	AvatarURL   string    `json:"avatar_url"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	Type        string    `json:"type"`
	OwnerUserID *string   `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SystemExpertGroupAsset struct {
	ExpertGroupID string    `json:"expert_group_id"`
	AvatarURL     string    `json:"avatar_url"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags"`
	ExpertIDs     []string  `json:"expert_ids"`
	Type          string    `json:"type"`
	OwnerUserID   *string   `json:"owner_user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SystemExpertAssets struct {
	Experts      []SystemExpertAsset      `json:"experts"`
	ExpertGroups []SystemExpertGroupAsset `json:"expert_groups"`
}

type CreateUserExpertAssetInput struct {
	AvatarFileName    string
	AvatarContentType string
	Avatar            []byte
	Name              string
	Description       string
	Prompt            string
	Category          string
	Tags              []string
}

type CreateUserExpertGroupAssetInput struct {
	AvatarFileName    string
	AvatarContentType string
	Avatar            []byte
	Name              string
	Description       string
	Category          string
	Tags              []string
	ExpertIDs         []string
}

type UserExpertAsset struct {
	ExpertID    string    `json:"expert_id"`
	AvatarURL   string    `json:"avatar_url"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	UsageCount  int64     `json:"usage_count"`
	Type        string    `json:"type"`
	OwnerUserID *string   `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserExpertGroupAsset struct {
	ExpertGroupID string    `json:"expert_group_id"`
	AvatarURL     string    `json:"avatar_url"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags"`
	UsageCount    int64     `json:"usage_count"`
	ExpertIDs     []string  `json:"expert_ids"`
	Type          string    `json:"type"`
	OwnerUserID   *string   `json:"owner_user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type userExpertAssetPage[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

type ExpertAssetsClient interface {
	ListSystem(context.Context) (SystemExpertAssets, error)
	CreateUserExpert(context.Context, BindingScope, CreateUserExpertAssetInput) (UserExpertAsset, error)
	CreateUserExpertGroup(context.Context, BindingScope, CreateUserExpertGroupAssetInput) (UserExpertGroupAsset, error)
}

type UserExpertAssetsClient interface {
	ListUserExperts(context.Context, BindingScope) ([]UserExpertAsset, error)
	ListUserExpertGroups(context.Context, BindingScope) ([]UserExpertGroupAsset, error)
}

type FullExpertAssetsClient interface {
	ExpertAssetsClient
	UserExpertAssetsClient
}

type httpExpertAssetsClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPExpertAssetsClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (FullExpertAssetsClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: expert assets client is not configured")
	}
	return &httpExpertAssetsClient{executor: executor, transport: transport}, nil
}

func (c *httpExpertAssetsClient) ListSystem(ctx context.Context) (SystemExpertAssets, error) {
	assets := SystemExpertAssets{}
	if err := c.transport.DoAuthenticatedJSON(ctx, http.MethodGet, systemExpertAssetsPath, nil, "", nil, &assets); err != nil {
		return SystemExpertAssets{}, err
	}
	if err := normalizeSystemExpertAssets(&assets); err != nil {
		return SystemExpertAssets{}, protocolError(err)
	}
	return assets, nil
}

func (c *httpExpertAssetsClient) ListUserExperts(ctx context.Context, scope BindingScope) ([]UserExpertAsset, error) {
	seen := make(map[string]struct{})
	return listUserExpertAssets(ctx, c, scope, userExpertsPath, nil, func(asset *UserExpertAsset, scope BindingScope) error {
		if err := normalizeListedUserExpertAsset(asset, scope); err != nil {
			return err
		}
		if _, duplicate := seen[asset.ExpertID]; duplicate {
			return fmt.Errorf("listed user experts contain duplicate id %q", asset.ExpertID)
		}
		seen[asset.ExpertID] = struct{}{}
		return nil
	})
}

func (c *httpExpertAssetsClient) ListUserExpertGroups(ctx context.Context, scope BindingScope) ([]UserExpertGroupAsset, error) {
	seen := make(map[string]struct{})
	return listUserExpertAssets(ctx, c, scope, userExpertGroupsPath, nil, func(asset *UserExpertGroupAsset, scope BindingScope) error {
		if err := normalizeListedUserExpertGroupAsset(asset, scope); err != nil {
			return err
		}
		if _, duplicate := seen[asset.ExpertGroupID]; duplicate {
			return fmt.Errorf("listed user expert groups contain duplicate id %q", asset.ExpertGroupID)
		}
		seen[asset.ExpertGroupID] = struct{}{}
		return nil
	})
}

func listUserExpertAssets[T any](
	ctx context.Context,
	client *httpExpertAssetsClient,
	scope BindingScope,
	path string,
	headers http.Header,
	normalize func(*T, BindingScope) error,
) ([]T, error) {
	items := make([]T, 0)
	cursor := ""
	seenCursors := map[string]struct{}{}
	for pageIndex := 0; pageIndex < userExpertAssetsMaxPages; pageIndex++ {
		query := url.Values{"limit": {fmt.Sprintf("%d", userExpertAssetsPageLimit)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var page userExpertAssetPage[T]
		err := client.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
			return client.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, path, query, nil, token, headers, &page)
		})
		if err != nil {
			return nil, err
		}
		for index := range page.Items {
			if err := normalize(&page.Items[index], scope); err != nil {
				return nil, protocolError(err)
			}
		}
		items = append(items, page.Items...)
		nextCursor, hasMore, err := nextUserExpertAssetCursor(page.HasMore, page.NextCursor)
		if err != nil {
			return nil, protocolError(err)
		}
		if !hasMore {
			return items, nil
		}
		if _, duplicate := seenCursors[nextCursor]; duplicate {
			return nil, protocolError(fmt.Errorf("expert asset pagination repeated cursor"))
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}
	return nil, protocolError(fmt.Errorf("expert asset pagination exceeds %d pages", userExpertAssetsMaxPages))
}

func nextUserExpertAssetCursor(hasMore bool, nextCursor *string) (string, bool, error) {
	if !hasMore {
		if nextCursor != nil {
			return "", false, fmt.Errorf("expert asset page returned next_cursor without has_more")
		}
		return "", false, nil
	}
	if nextCursor == nil {
		return "", false, fmt.Errorf("expert asset page omitted next_cursor while has_more is true")
	}
	if strings.TrimSpace(*nextCursor) == "" {
		return "", false, fmt.Errorf("expert asset page returned an empty next_cursor while has_more is true")
	}
	return *nextCursor, true, nil
}

func (c *httpExpertAssetsClient) CreateUserExpert(ctx context.Context, scope BindingScope, input CreateUserExpertAssetInput) (UserExpertAsset, error) {
	if err := normalizeCreateUserExpertAssetInput(&input); err != nil {
		return UserExpertAsset{}, err
	}
	fields := []multipartField{
		{name: "name", value: input.Name},
		{name: "prompt", value: input.Prompt},
		{name: "category", value: input.Category},
	}
	if input.Description != "" {
		fields = append(fields, multipartField{name: "description", value: input.Description})
	}
	if len(input.Tags) > 0 {
		encodedTags, encodeErr := json.Marshal(input.Tags)
		if encodeErr != nil {
			return UserExpertAsset{}, fmt.Errorf("encode expert tags: %w", encodeErr)
		}
		fields = append(fields, multipartField{name: "tags", value: string(encodedTags)})
	}
	payload, contentType, err := createExpertAssetMultipart(input.AvatarFileName, input.AvatarContentType, input.Avatar, fields)
	if err != nil {
		return UserExpertAsset{}, err
	}
	var response struct {
		Expert UserExpertAsset `json:"expert"`
	}
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedMultipartJSON(callCtx, http.MethodPost, userExpertsPath, payload, contentType, token, nil, &response)
	})
	if err != nil {
		return UserExpertAsset{}, err
	}
	if err := normalizeCreatedUserExpertAsset(&response.Expert); err != nil {
		return UserExpertAsset{}, protocolError(err)
	}
	return response.Expert, nil
}

func normalizeListedUserExpertAsset(asset *UserExpertAsset, scope BindingScope) error {
	if err := normalizeCreatedUserExpertAsset(asset); err != nil {
		return fmt.Errorf("listed user expert is invalid: %w", err)
	}
	if strings.TrimSpace(*asset.OwnerUserID) != strings.TrimSpace(scope.CloudUserID) {
		return fmt.Errorf("listed user expert %q belongs to another user", asset.ExpertID)
	}
	if asset.CreatedAt.IsZero() || asset.UpdatedAt.IsZero() {
		return fmt.Errorf("listed user expert %q has incomplete timestamps", asset.ExpertID)
	}
	return nil
}

func normalizeListedUserExpertGroupAsset(asset *UserExpertGroupAsset, scope BindingScope) error {
	if err := normalizeCreatedUserExpertGroupAsset(asset); err != nil {
		return fmt.Errorf("listed user expert group is invalid: %w", err)
	}
	if strings.TrimSpace(*asset.OwnerUserID) != strings.TrimSpace(scope.CloudUserID) {
		return fmt.Errorf("listed user expert group %q belongs to another user", asset.ExpertGroupID)
	}
	if asset.CreatedAt.IsZero() || asset.UpdatedAt.IsZero() {
		return fmt.Errorf("listed user expert group %q has incomplete timestamps", asset.ExpertGroupID)
	}
	return nil
}

func (c *httpExpertAssetsClient) CreateUserExpertGroup(ctx context.Context, scope BindingScope, input CreateUserExpertGroupAssetInput) (UserExpertGroupAsset, error) {
	if err := normalizeCreateUserExpertGroupAssetInput(&input); err != nil {
		return UserExpertGroupAsset{}, err
	}
	expertIDs, err := json.Marshal(input.ExpertIDs)
	if err != nil {
		return UserExpertGroupAsset{}, fmt.Errorf("encode expert_ids: %w", err)
	}
	fields := []multipartField{
		{name: "name", value: input.Name},
		{name: "category", value: input.Category},
		{name: "expert_ids", value: string(expertIDs)},
	}
	if input.Description != "" {
		fields = append(fields, multipartField{name: "description", value: input.Description})
	}
	if len(input.Tags) > 0 {
		encodedTags, encodeErr := json.Marshal(input.Tags)
		if encodeErr != nil {
			return UserExpertGroupAsset{}, fmt.Errorf("encode expert group tags: %w", encodeErr)
		}
		fields = append(fields, multipartField{name: "tags", value: string(encodedTags)})
	}
	payload, contentType, err := createExpertAssetMultipart(input.AvatarFileName, input.AvatarContentType, input.Avatar, fields)
	if err != nil {
		return UserExpertGroupAsset{}, err
	}
	var response struct {
		ExpertGroup UserExpertGroupAsset `json:"expert_group"`
	}
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedMultipartJSON(callCtx, http.MethodPost, userExpertGroupsPath, payload, contentType, token, nil, &response)
	})
	if err != nil {
		return UserExpertGroupAsset{}, err
	}
	if err := normalizeCreatedUserExpertGroupAsset(&response.ExpertGroup); err != nil {
		return UserExpertGroupAsset{}, protocolError(err)
	}
	return response.ExpertGroup, nil
}

type multipartField struct {
	name  string
	value string
}

func createExpertAssetMultipart(fileName, contentType string, avatar []byte, fields []multipartField) ([]byte, string, error) {
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name": "avatar", "filename": fileName,
	}))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", fmt.Errorf("create avatar multipart part: %w", err)
	}
	if _, err := part.Write(avatar); err != nil {
		return nil, "", fmt.Errorf("write avatar multipart part: %w", err)
	}
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return nil, "", fmt.Errorf("write multipart field %q: %w", field.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finish expert asset multipart body: %w", err)
	}
	return payload.Bytes(), writer.FormDataContentType(), nil
}

func normalizeSystemExpertAssets(assets *SystemExpertAssets) error {
	assets.Experts = nonNilSystemAssets(assets.Experts)
	assets.ExpertGroups = nonNilSystemAssets(assets.ExpertGroups)
	expertIDs, err := normalizeSystemExperts(assets.Experts)
	if err != nil {
		return err
	}
	if err := normalizeSystemExpertGroups(assets.ExpertGroups, expertIDs); err != nil {
		return err
	}
	sort.SliceStable(assets.Experts, func(i, j int) bool {
		return assets.Experts[i].ExpertID < assets.Experts[j].ExpertID
	})
	sort.SliceStable(assets.ExpertGroups, func(i, j int) bool {
		return assets.ExpertGroups[i].ExpertGroupID < assets.ExpertGroups[j].ExpertGroupID
	})
	return nil
}

func nonNilSystemAssets[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func normalizeSystemExperts(items []SystemExpertAsset) (map[string]struct{}, error) {
	expertIDs := make(map[string]struct{}, len(items))
	for index := range items {
		item := &items[index]
		normalizeSystemExpertFields(item)
		if err := validateSystemExpertAsset(*item, expertIDs); err != nil {
			return nil, err
		}
		expertIDs[item.ExpertID] = struct{}{}
	}
	return expertIDs, nil
}

func normalizeSystemExpertFields(item *SystemExpertAsset) {
	item.ExpertID = strings.TrimSpace(item.ExpertID)
	item.AvatarURL = strings.TrimSpace(item.AvatarURL)
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.Prompt = strings.TrimSpace(item.Prompt)
	item.Category = strings.TrimSpace(item.Category)
}

func validateSystemExpertAsset(item SystemExpertAsset, expertIDs map[string]struct{}) error {
	if item.ExpertID == "" || item.Name == "" || item.Prompt == "" {
		return fmt.Errorf("system expert response contains an incomplete expert")
	}
	if item.Type != "system" || item.OwnerUserID != nil {
		return fmt.Errorf("system expert %q has invalid ownership", item.ExpertID)
	}
	if err := validateSystemExpertAvatarURL(item.AvatarURL); err != nil {
		return fmt.Errorf("system expert %q: %w", item.ExpertID, err)
	}
	if _, exists := expertIDs[item.ExpertID]; exists {
		return fmt.Errorf("system expert response contains duplicate id %q", item.ExpertID)
	}
	return nil
}

func normalizeSystemExpertGroups(items []SystemExpertGroupAsset, expertIDs map[string]struct{}) error {
	groupIDs := make(map[string]struct{}, len(items))
	for index := range items {
		item := &items[index]
		normalizeSystemExpertGroupFields(item)
		if err := validateSystemExpertGroupAsset(*item, groupIDs); err != nil {
			return err
		}
		if err := normalizeSystemExpertGroupMembers(item, expertIDs); err != nil {
			return err
		}
		groupIDs[item.ExpertGroupID] = struct{}{}
	}
	return nil
}

func normalizeSystemExpertGroupFields(item *SystemExpertGroupAsset) {
	item.ExpertGroupID = strings.TrimSpace(item.ExpertGroupID)
	item.AvatarURL = strings.TrimSpace(item.AvatarURL)
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.Category = strings.TrimSpace(item.Category)
	if item.ExpertIDs == nil {
		item.ExpertIDs = []string{}
	}
}

func validateSystemExpertGroupAsset(item SystemExpertGroupAsset, groupIDs map[string]struct{}) error {
	if item.ExpertGroupID == "" || item.Name == "" {
		return fmt.Errorf("system expert response contains an incomplete expert group")
	}
	if item.Type != "system" || item.OwnerUserID != nil {
		return fmt.Errorf("system expert group %q has invalid ownership", item.ExpertGroupID)
	}
	if err := validateSystemExpertAvatarURL(item.AvatarURL); err != nil {
		return fmt.Errorf("system expert group %q: %w", item.ExpertGroupID, err)
	}
	if _, exists := groupIDs[item.ExpertGroupID]; exists {
		return fmt.Errorf("system expert response contains duplicate group id %q", item.ExpertGroupID)
	}
	return nil
}

func normalizeSystemExpertGroupMembers(item *SystemExpertGroupAsset, expertIDs map[string]struct{}) error {
	memberIDs := make(map[string]struct{}, len(item.ExpertIDs))
	for index, rawID := range item.ExpertIDs {
		expertID := strings.TrimSpace(rawID)
		if _, exists := expertIDs[expertID]; !exists {
			return fmt.Errorf("system expert group %q references unknown expert %q", item.ExpertGroupID, expertID)
		}
		if _, exists := memberIDs[expertID]; exists {
			return fmt.Errorf("system expert group %q contains duplicate expert %q", item.ExpertGroupID, expertID)
		}
		memberIDs[expertID] = struct{}{}
		item.ExpertIDs[index] = expertID
	}
	return nil
}

func validateSystemExpertAvatarURL(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 2048 {
		return fmt.Errorf("avatar_url exceeds 2048 bytes")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("avatar_url must be an absolute credential-free HTTPS URL")
	}
	return nil
}

const (
	errorReportsPath               = "/api/v1/error-reports"
	ErrorReportDescriptionMaxBytes = 10_000
	ErrorReportAttachmentMaxBytes  = 20 * 1024 * 1024
	ErrorReportStatusSubmitted     = "submitted"
	errorReportIdempotencyHeader   = "X-Idempotency-Key"
	errorReportAttachmentField     = "file"
	errorReportDescriptionField    = "description"
	errorReportContentTypePNG      = "image/png"
	errorReportContentTypeJPEG     = "image/jpeg"
	errorReportContentTypeZIP      = "application/zip"
)

// ErrorReportAttachmentInput holds one sanitized in-memory desktop attachment.
type ErrorReportAttachmentInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

type SubmitErrorReportInput struct {
	Description    string
	IdempotencyKey string
	Attachment     *ErrorReportAttachmentInput
}

type ErrorReportSubmission struct {
	ReportID string
	Status   string
	Replayed bool
}

type ErrorReportConfirmation struct {
	ReportID string
	Status   string
}

type ErrorReportClient interface {
	SubmitErrorReport(context.Context, BindingScope, SubmitErrorReportInput) (ErrorReportSubmission, error)
	GetErrorReport(context.Context, BindingScope, string) (ErrorReportConfirmation, error)
}

type httpErrorReportClient struct {
	executor  AuthenticatedRequestExecutor
	transport *HTTPSessionClient
}

func NewHTTPErrorReportClient(executor AuthenticatedRequestExecutor, transport *HTTPSessionClient) (ErrorReportClient, error) {
	if isNilDependency(executor) || transport == nil {
		return nil, fmt.Errorf("cloud: error report client is not configured")
	}
	return &httpErrorReportClient{executor: executor, transport: transport}, nil
}

func (c *httpErrorReportClient) SubmitErrorReport(ctx context.Context, scope BindingScope, input SubmitErrorReportInput) (ErrorReportSubmission, error) {
	if err := validateSubmitErrorReportInput(scope, input); err != nil {
		return ErrorReportSubmission{}, err
	}
	payload, contentType, err := createErrorReportMultipart(input)
	if err != nil {
		return ErrorReportSubmission{}, err
	}
	var response struct {
		Report struct {
			ReportID string `json:"report_id"`
			Status   string `json:"status"`
		} `json:"report"`
		Replayed bool `json:"replayed"`
	}
	headers := http.Header{errorReportIdempotencyHeader: []string{input.IdempotencyKey}}
	err = c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedMultipartJSON(
			callCtx,
			http.MethodPost,
			errorReportsPath,
			payload,
			contentType,
			token,
			headers,
			&response,
		)
	})
	if err != nil {
		return ErrorReportSubmission{}, err
	}
	if err := requireTrimmed("report_id", response.Report.ReportID); err != nil {
		return ErrorReportSubmission{}, protocolError(fmt.Errorf("error report response has invalid report_id: %w", err))
	}
	if err := requireTrimmed("status", response.Report.Status); err != nil {
		return ErrorReportSubmission{}, protocolError(fmt.Errorf("error report response has invalid status: %w", err))
	}
	if response.Report.Status != ErrorReportStatusSubmitted {
		return ErrorReportSubmission{}, protocolError(fmt.Errorf("error report response returned unsupported status %q", response.Report.Status))
	}
	return ErrorReportSubmission{ReportID: response.Report.ReportID, Status: response.Report.Status, Replayed: response.Replayed}, nil
}

func (c *httpErrorReportClient) GetErrorReport(ctx context.Context, scope BindingScope, reportID string) (ErrorReportConfirmation, error) {
	if err := scope.Validate(); err != nil {
		return ErrorReportConfirmation{}, err
	}
	if err := validateErrorReportID(reportID); err != nil {
		return ErrorReportConfirmation{}, err
	}
	var response struct {
		Report struct {
			ReportID string `json:"report_id"`
			Status   string `json:"status"`
		} `json:"report"`
	}
	err := c.executor.Do(ctx, scope, func(callCtx context.Context, token string) error {
		return c.transport.DoAuthenticatedJSON(
			callCtx,
			http.MethodGet,
			errorReportsPath+"/"+url.PathEscape(reportID),
			nil,
			token,
			nil,
			&response,
		)
	})
	if err != nil {
		return ErrorReportConfirmation{}, err
	}
	if err := validateErrorReportID(response.Report.ReportID); err != nil {
		return ErrorReportConfirmation{}, protocolError(fmt.Errorf("error report query response has invalid report_id: %w", err))
	}
	if response.Report.ReportID != reportID {
		return ErrorReportConfirmation{}, protocolError(fmt.Errorf("error report query returned mismatched report_id"))
	}
	if err := requireTrimmed("status", response.Report.Status); err != nil {
		return ErrorReportConfirmation{}, protocolError(fmt.Errorf("error report query response has invalid status: %w", err))
	}
	if response.Report.Status != ErrorReportStatusSubmitted {
		return ErrorReportConfirmation{}, protocolError(fmt.Errorf("error report query returned unsupported status %q", response.Report.Status))
	}
	return ErrorReportConfirmation{ReportID: response.Report.ReportID, Status: response.Report.Status}, nil
}

func validateErrorReportID(reportID string) error {
	if err := requireTrimmed("report_id", reportID); err != nil {
		return err
	}
	if len(reportID) > 128 {
		return fmt.Errorf("report_id exceeds 128 bytes")
	}
	for _, char := range reportID {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.:", char) {
			return fmt.Errorf("report_id contains unsupported characters")
		}
	}
	return nil
}

func validateSubmitErrorReportInput(scope BindingScope, input SubmitErrorReportInput) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := requireTrimmed(errorReportDescriptionField, input.Description); err != nil {
		return err
	}
	if len([]byte(input.Description)) > ErrorReportDescriptionMaxBytes {
		return fmt.Errorf("cloud: description exceeds %d bytes", ErrorReportDescriptionMaxBytes)
	}
	if err := requireTrimmed("idempotency_key", input.IdempotencyKey); err != nil {
		return err
	}
	if input.Attachment == nil {
		return nil
	}
	return validateErrorReportAttachment(input.Attachment)
}

func validateErrorReportAttachment(attachment *ErrorReportAttachmentInput) error {
	if err := requireTrimmed("attachment filename", attachment.Filename); err != nil {
		return err
	}
	if strings.ContainsAny(attachment.Filename, `/\`) || strings.ContainsAny(attachment.Filename, "\r\n\x00") {
		return fmt.Errorf("cloud: attachment filename is invalid")
	}
	switch attachment.ContentType {
	case errorReportContentTypePNG, errorReportContentTypeJPEG, errorReportContentTypeZIP:
	default:
		return fmt.Errorf("cloud: attachment content type %q is unsupported", attachment.ContentType)
	}
	if len(attachment.Data) == 0 {
		return fmt.Errorf("cloud: attachment data is required")
	}
	if len(attachment.Data) > ErrorReportAttachmentMaxBytes {
		return fmt.Errorf("cloud: attachment exceeds %d bytes", ErrorReportAttachmentMaxBytes)
	}
	return nil
}

func createErrorReportMultipart(input SubmitErrorReportInput) ([]byte, string, error) {
	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	if err := writer.WriteField(errorReportDescriptionField, input.Description); err != nil {
		return nil, "", fmt.Errorf("cloud: write error report description: %w", err)
	}
	if input.Attachment != nil {
		disposition := mime.FormatMediaType("form-data", map[string]string{
			"name":     errorReportAttachmentField,
			"filename": input.Attachment.Filename,
		})
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Type", input.Attachment.ContentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, "", fmt.Errorf("cloud: create error report attachment: %w", err)
		}
		if _, err := part.Write(input.Attachment.Data); err != nil {
			return nil, "", fmt.Errorf("cloud: write error report attachment: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("cloud: finalize error report multipart: %w", err)
	}
	return payload.Bytes(), writer.FormDataContentType(), nil
}
