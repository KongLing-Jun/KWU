package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kwu/internal/frontmatter"
	"kwu/internal/skill"
)

const (
	publicSkillArchiveLimit            = 64 << 20
	publicSkillInstallMetadataFilename = ".clouvia-marketplace.json"
	publicSkillErrAlreadyInstalled     = "SKILL_ALREADY_INSTALLED"
	publicSkillErrCatalogUnavailable   = "SKILL_CATALOG_UNAVAILABLE"
	publicSkillErrDownloadFailed       = "SKILL_DOWNLOAD_FAILED"
	publicSkillErrInstallPermission    = "SKILL_INSTALL_PERMISSION"
	publicSkillErrInstallBusy          = "SKILL_INSTALL_BUSY"
	publicSkillErrInstallFailed        = "SKILL_INSTALL_FAILED"
	publicSkillErrUninstallFailed      = "SKILL_UNINSTALL_FAILED"
	publicSkillUninstallTrashDirname   = "uninstalled-skills"
)

type publicSkillInstallMetadata struct {
	Namespace string `json:"namespace"`
	Slug      string `json:"slug"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version"`
	IconURL   string `json:"iconUrl,omitempty"`
}

// PublicSkillView is the public, unauthenticated skill catalog contract exposed
// by the cloud service. The object key is intentionally not represented here.
type PublicSkillView struct {
	Category          string   `json:"category"`
	Categories        []string `json:"categories"`
	Type              string   `json:"type"`
	ClaimState        string   `json:"claim_state"`
	Claimable         bool     `json:"claimable"`
	ClaimedUserHandle *string  `json:"claimed_user_handle,omitempty"`
	CreatedAt         string   `json:"created_at"`
	Description       string   `json:"description"`
	Downloads         int64    `json:"downloads"`
	Homepage          string   `json:"homepage"`
	IconURL           *string  `json:"iconUrl,omitempty"`
	Installs          int64    `json:"installs"`
	InstallCount      int64    `json:"install_count"`
	Labels            []string `json:"labels"`
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace"`
	OwnerName         string   `json:"ownerName"`
	Publisher         string   `json:"publisher"`
	Score             float64  `json:"score"`
	Slug              string   `json:"slug"`
	Source            string   `json:"source"`
	Stars             int64    `json:"stars"`
	SubCategories     []string `json:"subCategories"`
	Tags              []string `json:"tags"`
	UpdatedAt         string   `json:"updated_at"`
	Verified          bool     `json:"verified"`
	Version           string   `json:"version"`
	DownloadURL       string   `json:"download_url"`
	DownloadFilename  string   `json:"download_filename"`
	DownloadSizeBytes int64    `json:"download_size_bytes"`
	DownloadSHA256    string   `json:"download_sha256"`
}

// UnmarshalJSON tolerates the lossless third-party metadata shapes stored by
// the cloud catalog. Older imports used objects for namespace, publisher,
// labels, and sub-categories while the public client contract projects those
// values to strings and string lists.
func (s *PublicSkillView) UnmarshalJSON(data []byte) error {
	type publicSkillWire struct {
		Category          string          `json:"category"`
		Categories        json.RawMessage `json:"categories"`
		Type              string          `json:"type"`
		ClaimState        string          `json:"claim_state"`
		Claimable         bool            `json:"claimable"`
		ClaimedUserHandle *string         `json:"claimed_user_handle"`
		CreatedAt         json.RawMessage `json:"created_at"`
		Description       string          `json:"description"`
		DescriptionZH     string          `json:"description_zh"`
		Downloads         int64           `json:"downloads"`
		Homepage          string          `json:"homepage"`
		IconURL           *string         `json:"iconUrl"`
		Installs          int64           `json:"installs"`
		InstallCount      int64           `json:"install_count"`
		Labels            json.RawMessage `json:"labels"`
		Name              string          `json:"name"`
		Namespace         json.RawMessage `json:"namespace"`
		OwnerName         string          `json:"ownerName"`
		Publisher         json.RawMessage `json:"publisher"`
		Score             float64         `json:"score"`
		Slug              string          `json:"slug"`
		Source            string          `json:"source"`
		Stars             int64           `json:"stars"`
		SubCategories     json.RawMessage `json:"subCategories"`
		Tags              json.RawMessage `json:"tags"`
		UpdatedAt         json.RawMessage `json:"updated_at"`
		Verified          bool            `json:"verified"`
		Version           string          `json:"version"`
		DownloadURL       string          `json:"download_url"`
		DownloadFilename  string          `json:"download_filename"`
		DownloadSizeBytes int64           `json:"download_size_bytes"`
		DownloadSHA256    string          `json:"download_sha256"`
	}
	var wire publicSkillWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	description := wire.Description
	if strings.TrimSpace(wire.DescriptionZH) != "" {
		description = wire.DescriptionZH
	}
	*s = PublicSkillView{
		Category: wire.Category, Categories: publicSkillStringList(wire.Categories),
		Type:       wire.Type,
		ClaimState: wire.ClaimState, Claimable: wire.Claimable,
		ClaimedUserHandle: wire.ClaimedUserHandle, CreatedAt: publicSkillTimestamp(wire.CreatedAt),
		Description: description, Downloads: wire.Downloads, Homepage: wire.Homepage, IconURL: wire.IconURL,
		Installs: wire.Installs, InstallCount: wire.InstallCount,
		Labels: publicSkillStringList(wire.Labels), Name: wire.Name,
		Namespace: publicSkillString(wire.Namespace, "handle", "displayName", "canonicalName"), OwnerName: wire.OwnerName,
		Publisher: publicSkillString(wire.Publisher, "name", "displayName", "handle"), Score: wire.Score,
		Slug: wire.Slug, Source: wire.Source, Stars: wire.Stars,
		SubCategories: publicSkillStringList(wire.SubCategories), Tags: publicSkillStringList(wire.Tags),
		UpdatedAt: publicSkillTimestamp(wire.UpdatedAt), Verified: wire.Verified, Version: wire.Version,
		DownloadURL: wire.DownloadURL, DownloadFilename: wire.DownloadFilename,
		DownloadSizeBytes: wire.DownloadSizeBytes, DownloadSHA256: wire.DownloadSHA256,
	}
	s.Namespace = strings.TrimPrefix(strings.TrimSpace(s.Namespace), "@")
	return nil
}

func publicSkillString(raw json.RawMessage, objectFields ...string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return ""
	}
	for _, field := range objectFields {
		if candidate, ok := object[field]; ok && json.Unmarshal(candidate, &value) == nil && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func publicSkillStringList(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}
	}
	var stringsOnly []string
	if json.Unmarshal(raw, &stringsOnly) == nil {
		return stringsOnly
	}
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) == nil {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text := publicSkillString(value, "name", "key", "label"); strings.TrimSpace(text) != "" {
				out = append(out, text)
			}
		}
		return out
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		out := make([]string, 0, len(object))
		for key := range object {
			out = append(out, key)
		}
		sort.Strings(out)
		return out
	}
	return []string{}
}

func publicSkillTimestamp(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var milliseconds int64
	if json.Unmarshal(raw, &milliseconds) == nil && milliseconds > 0 {
		return time.UnixMilli(milliseconds).UTC().Format(time.RFC3339)
	}
	return ""
}

type PublicSkillsPageView struct {
	Items      []PublicSkillView `json:"items"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type PublicSkillDetailView struct {
	Skill PublicSkillView `json:"skill"`
}

type PublicSkillInstallResult struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	Slug           string `json:"slug"`
	Version        string `json:"version"`
	Path           string `json:"path"`
	ReloadRequired bool   `json:"reload_required"`
}

type publicSkillInstallationView struct {
	CanonicalName string `json:"canonical_name"`
	InstallCount  int64  `json:"install_count"`
}

type publicSkillInstallationData struct {
	Installation publicSkillInstallationView `json:"installation"`
}

type publicSkillAPIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type publicSkillEnvelope[T any] struct {
	Data      *T                   `json:"data"`
	Error     *publicSkillAPIError `json:"error"`
	RequestID string               `json:"request_id"`
}

func (a *App) PublicSkills(limit int, cursor string) (PublicSkillsPageView, error) {
	baseURL, client, err := publicSkillCloudClient()
	if err != nil {
		return PublicSkillsPageView{}, err
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 30*time.Second)
	defer cancel()
	return fetchPublicSkills(ctx, client, baseURL, limit, cursor)
}

func (a *App) SearchPublicSkills(keyword string, limit int, cursor string) (PublicSkillsPageView, error) {
	baseURL, client, err := publicSkillCloudClient()
	if err != nil {
		return PublicSkillsPageView{}, err
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 30*time.Second)
	defer cancel()
	return searchPublicSkills(ctx, client, baseURL, keyword, limit, cursor)
}

func (a *App) PublicSkill(namespace, slug string) (PublicSkillDetailView, error) {
	baseURL, client, err := publicSkillCloudClient()
	if err != nil {
		return PublicSkillDetailView{}, err
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 30*time.Second)
	defer cancel()
	return fetchPublicSkill(ctx, client, baseURL, namespace, slug)
}

func (a *App) InstallPublicSkill(namespace, slug string) (PublicSkillInstallResult, error) {
	baseURL, client, err := publicSkillCloudClient()
	if err != nil {
		return PublicSkillInstallResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 2*time.Minute)
	defer cancel()
	detail, err := fetchPublicSkill(ctx, client, baseURL, namespace, slug)
	if err != nil {
		return PublicSkillInstallResult{}, publicSkillInstallError(publicSkillErrCatalogUnavailable, "获取技能详情失败", err)
	}
	paths := a.activeProfilePathResolver()
	installedPath, installedName, err := installPublicSkillArchive(ctx, client, detail.Skill, filepath.Join(paths.KWUHomeDir(), skill.SkillsDirname))
	if err != nil {
		return PublicSkillInstallResult{}, err
	}
	// The installation endpoint intentionally has no idempotency key. Send one
	// request after the archive has been committed locally and never retry it:
	// an ambiguous retry would increment the public counter a second time.
	if _, err := recordPublicSkillInstallation(ctx, client, baseURL, detail.Skill.Namespace, detail.Skill.Slug); err != nil {
		slog.Warn("record public skill installation failed", "namespace", detail.Skill.Namespace, "slug", detail.Skill.Slug, "err", err)
	}
	result := PublicSkillInstallResult{
		Name: installedName, Namespace: detail.Skill.Namespace, Slug: detail.Skill.Slug,
		Version: detail.Skill.Version, Path: installedPath,
	}
	if err := a.RefreshSkills(); err != nil {
		result.ReloadRequired = true
	}
	return result, nil
}

// UninstallPublicSkill removes an app-managed marketplace skill from discovery.
// The directory is moved to a profile-local recovery folder instead of being
// deleted, so an accidental uninstall remains recoverable.
func (a *App) UninstallPublicSkill(slug string) error {
	paths := a.activeProfilePathResolver()
	kwuHome := paths.KWUHomeDir()
	if err := uninstallPublicSkill(
		filepath.Join(kwuHome, skill.SkillsDirname),
		filepath.Join(kwuHome, publicSkillUninstallTrashDirname),
		slug,
	); err != nil {
		return err
	}
	return a.RefreshSkills()
}

func uninstallPublicSkill(skillsRoot, trashRoot, slug string) error {
	slug = strings.TrimSpace(slug)
	if !skill.IsValidName(slug) {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "技能标识无效", nil)
	}
	skillsRoot = filepath.Clean(skillsRoot)
	target := filepath.Join(skillsRoot, slug)
	if !pathWithin(skillsRoot, target) {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "技能路径不安全", nil)
	}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return publicSkillInstallError(publicSkillErrUninstallFailed, fmt.Sprintf("技能 %q 未安装", slug), nil)
		}
		return publicSkillInstallError(publicSkillErrUninstallFailed, "检查技能目录失败", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "技能目录类型不安全", nil)
	}
	metadata, ok := installedPublicSkillMetadataInDir(target)
	if !ok || !strings.EqualFold(strings.TrimSpace(metadata.Slug), slug) {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "只能卸载由技能库安装的技能", nil)
	}
	if err := os.MkdirAll(trashRoot, 0o755); err != nil {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "创建技能恢复目录失败", err)
	}
	destination := filepath.Join(trashRoot, slug+"-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err := os.Rename(target, destination); err != nil {
		return publicSkillInstallError(publicSkillErrUninstallFailed, "移动技能到恢复目录失败", err)
	}
	return nil
}

func publicSkillCloudClient() (string, *http.Client, error) {
	baseURL, err := resolveCloudBaseURL(version, os.Getenv(cloudBaseURLEnv), cloudDevelopmentBaseURL, cloudProductionBaseURL)
	if err != nil {
		return "", nil, err
	}
	client, err := newPublicCloudHTTPClient(baseURL)
	if err != nil {
		return "", nil, err
	}
	return strings.TrimRight(baseURL, "/"), client, nil
}

func fetchPublicSkills(ctx context.Context, client *http.Client, baseURL string, limit int, cursor string) (PublicSkillsPageView, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return PublicSkillsPageView{}, fmt.Errorf("技能列表数量必须在 1 到 100 之间")
	}
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/skills")
	if err != nil {
		return PublicSkillsPageView{}, fmt.Errorf("构造技能列表地址失败: %w", err)
	}
	query := endpoint.Query()
	query.Set("limit", strconv.Itoa(limit))
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		query.Set("cursor", cursor)
	}
	endpoint.RawQuery = query.Encode()
	return getPublicSkillEnvelope[PublicSkillsPageView](ctx, client, endpoint.String())
}

func searchPublicSkills(ctx context.Context, client *http.Client, baseURL, keyword string, limit int, cursor string) (PublicSkillsPageView, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len([]rune(keyword)) > 128 {
		return PublicSkillsPageView{}, fmt.Errorf("技能搜索关键词必须包含 1 到 128 个字符")
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return PublicSkillsPageView{}, fmt.Errorf("技能搜索数量必须在 1 到 100 之间")
	}
	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/skills/search")
	if err != nil {
		return PublicSkillsPageView{}, fmt.Errorf("构造技能搜索地址失败: %w", err)
	}
	query := endpoint.Query()
	query.Set("keyword", keyword)
	query.Set("limit", strconv.Itoa(limit))
	if cursor = strings.TrimSpace(cursor); cursor != "" {
		query.Set("cursor", cursor)
	}
	endpoint.RawQuery = query.Encode()
	return getPublicSkillEnvelope[PublicSkillsPageView](ctx, client, endpoint.String())
}

func fetchPublicSkill(ctx context.Context, client *http.Client, baseURL, namespace, slug string) (PublicSkillDetailView, error) {
	namespace = strings.TrimSpace(strings.TrimPrefix(namespace, "@"))
	slug = strings.TrimSpace(slug)
	if namespace == "" || slug == "" || strings.ContainsAny(namespace+slug, "/\\") {
		return PublicSkillDetailView{}, fmt.Errorf("技能命名空间和标识不正确")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/skills/" + url.PathEscape(namespace) + "/" + url.PathEscape(slug)
	return getPublicSkillEnvelope[PublicSkillDetailView](ctx, client, endpoint)
}

func recordPublicSkillInstallation(ctx context.Context, client *http.Client, baseURL, namespace, slug string) (publicSkillInstallationView, error) {
	var zero publicSkillInstallationView
	namespace = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(namespace), "@"))
	slug = strings.TrimSpace(slug)
	if namespace == "" || slug == "" || strings.ContainsAny(namespace+slug, "/\\") {
		return zero, fmt.Errorf("技能命名空间和标识不正确")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/skills/" + url.PathEscape(namespace) + "/" + url.PathEscape(slug) + "/installations"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return zero, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return zero, fmt.Errorf("连接技能库失败: %w", err)
	}
	defer response.Body.Close()

	var envelope publicSkillEnvelope[publicSkillInstallationData]
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return zero, fmt.Errorf("技能库返回了无效响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Error != nil {
		message := strings.TrimSpace(response.Status)
		if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
			message = envelope.Error.Message
		}
		if envelope.RequestID != "" {
			message += "（请求 ID：" + envelope.RequestID + "）"
		}
		return zero, errors.New(message)
	}
	if envelope.Data == nil {
		return zero, errors.New("技能库响应缺少数据")
	}
	return envelope.Data.Installation, nil
}

func getPublicSkillEnvelope[T any](ctx context.Context, client *http.Client, endpoint string) (T, error) {
	var zero T
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return zero, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return zero, fmt.Errorf("连接技能库失败: %w", err)
	}
	defer response.Body.Close()
	var envelope publicSkillEnvelope[T]
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return zero, fmt.Errorf("技能库返回了无效响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Error != nil {
		message := strings.TrimSpace(response.Status)
		if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
			message = envelope.Error.Message
		}
		if envelope.RequestID != "" {
			message += "（请求 ID：" + envelope.RequestID + "）"
		}
		return zero, errors.New(message)
	}
	if envelope.Data == nil {
		return zero, errors.New("技能库响应缺少数据")
	}
	return *envelope.Data, nil
}

func installPublicSkillArchive(ctx context.Context, client *http.Client, meta PublicSkillView, skillsRoot string) (string, string, error) {
	slug := strings.TrimSpace(meta.Slug)
	if !skill.IsValidName(slug) {
		return "", "", publicSkillInstallError(publicSkillErrInstallFailed, fmt.Sprintf("技能标识 %q 不可用于本地安装", slug), nil)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(meta.DownloadURL), nil)
	if err != nil {
		return "", "", publicSkillInstallError(publicSkillErrDownloadFailed, "创建技能包下载请求失败", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return "", "", publicSkillInstallError(publicSkillErrDownloadFailed, "下载技能包失败", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", publicSkillInstallError(publicSkillErrDownloadFailed, "下载技能包失败", errors.New(response.Status))
	}
	archive, err := io.ReadAll(io.LimitReader(response.Body, publicSkillArchiveLimit+1))
	if err != nil {
		return "", "", publicSkillInstallError(publicSkillErrDownloadFailed, "读取技能包失败", err)
	}
	if int64(len(archive)) > publicSkillArchiveLimit {
		return "", "", publicSkillInstallError(publicSkillErrInstallFailed, "技能包超过大小限制", nil)
	}
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return "", "", publicSkillInstallError(publicSkillErrInstallPermission, "创建技能目录失败", err)
	}
	target := filepath.Join(skillsRoot, slug)
	if _, err := os.Lstat(target); err == nil {
		return "", "", publicSkillInstallError(publicSkillErrAlreadyInstalled, fmt.Sprintf("技能 %q 已安装", slug), nil)
	} else if !os.IsNotExist(err) {
		return "", "", publicSkillInstallError(publicSkillErrInstallPermission, "检查技能目录失败", err)
	}
	tempRoot, err := os.MkdirTemp(skillsRoot, ".skill-install-*")
	if err != nil {
		return "", "", publicSkillInstallError(publicSkillErrInstallPermission, "创建技能安装临时目录失败", err)
	}
	defer os.RemoveAll(tempRoot)
	// Use the catalog slug for the staging directory. When an imported package
	// uses a human-readable (for example Chinese) frontmatter name, the skill
	// loader correctly falls back to this directory name instead of leaking the
	// implementation-only name "payload" into duplicate checks and results.
	payload := filepath.Join(tempRoot, slug)
	if err := extractPublicSkillArchive(archive, payload); err != nil {
		return "", "", publicSkillInstallError(publicSkillErrInstallFailed, "解压技能包失败", err)
	}
	store := skill.New(skill.Options{CustomPaths: []string{tempRoot}, DisableBuiltins: true, Stderr: io.Discard})
	installed := store.List()
	installedName := slug
	if len(installed) == 1 && pathWithin(payload, installed[0].Path) {
		installedName = installed[0].Name
	}
	for _, existing := range skill.New(skill.Options{CustomPaths: []string{skillsRoot}, DisableBuiltins: true, Stderr: io.Discard}).List() {
		if strings.EqualFold(existing.Name, installedName) {
			return "", "", publicSkillInstallError(publicSkillErrAlreadyInstalled, fmt.Sprintf("技能 %q 已安装", installedName), nil)
		}
	}
	if err := writePublicSkillInstallMetadata(payload, meta); err != nil {
		return "", "", publicSkillInstallError(publicSkillErrInstallPermission, "保存技能元数据失败", err)
	}
	if err := commitPublicSkillDirectory(ctx, payload, target, os.Rename, []time.Duration{
		25 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}); err != nil {
		if errors.Is(err, fs.ErrExist) || os.IsExist(err) {
			return "", "", publicSkillInstallError(publicSkillErrAlreadyInstalled, fmt.Sprintf("技能 %q 已安装", slug), err)
		}
		if publicSkillCommitPermissionError(err) {
			return "", "", publicSkillInstallError(publicSkillErrInstallPermission, "无法写入技能目录", err)
		}
		if publicSkillCommitBusyError(err) {
			return "", "", publicSkillInstallError(publicSkillErrInstallBusy, "技能文件正被其他程序占用", err)
		}
		return "", "", publicSkillInstallError(publicSkillErrInstallFailed, "保存技能失败", err)
	}
	return filepath.Join(target, skill.SkillFile), installedName, nil
}

func publicSkillInstallError(code, message string, cause error) error {
	message = strings.TrimSpace(message)
	if cause == nil {
		return errors.New(code + ": " + message)
	}
	return fmt.Errorf("%s: %s: %w", code, message, cause)
}

func commitPublicSkillDirectory(
	ctx context.Context,
	source, target string,
	rename func(string, string) error,
	retryDelays []time.Duration,
) error {
	for attempt := 0; ; attempt++ {
		err := rename(source, target)
		if err == nil {
			return nil
		}
		if !retryablePublicSkillCommitError(err) || attempt >= len(retryDelays) {
			return err
		}
		delay := retryDelays[attempt]
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func retryablePublicSkillCommitError(err error) bool {
	return publicSkillCommitPermissionError(err) || publicSkillCommitBusyError(err)
}

func publicSkillCommitPermissionError(err error) bool {
	if errors.Is(err, fs.ErrPermission) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == 5
}

func publicSkillCommitBusyError(err error) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	// Defender, an indexer, or another scanner can briefly retain an extracted
	// file and report sharing/lock violations while the directory is committed.
	return errno == 32 || errno == 33
}

func writePublicSkillInstallMetadata(skillDir string, meta PublicSkillView) error {
	iconURL := ""
	if meta.IconURL != nil {
		iconURL = strings.TrimSpace(*meta.IconURL)
	}
	metadata := publicSkillInstallMetadata{
		Namespace: strings.TrimSpace(meta.Namespace),
		Slug:      strings.TrimSpace(meta.Slug),
		Name:      strings.TrimSpace(meta.Name),
		Version:   strings.TrimSpace(meta.Version),
		IconURL:   iconURL,
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, publicSkillInstallMetadataFilename), data, 0o644)
}

func installedPublicSkillIconURL(skillPath string) string {
	metadata, ok := installedPublicSkillMetadata(skillPath)
	if !ok {
		return ""
	}
	return strings.TrimSpace(metadata.IconURL)
}

func installedPublicSkillMetadata(skillPath string) (publicSkillInstallMetadata, bool) {
	skillPath = strings.TrimSpace(skillPath)
	if skillPath == "" || skillPath == "(builtin)" {
		return publicSkillInstallMetadata{}, false
	}
	return installedPublicSkillMetadataInDir(filepath.Dir(skillPath))
}

func installedPublicSkillMetadataInDir(skillDir string) (publicSkillInstallMetadata, bool) {
	data, err := os.ReadFile(filepath.Join(skillDir, publicSkillInstallMetadataFilename))
	if err != nil {
		return publicSkillInstallMetadata{}, false
	}
	var metadata publicSkillInstallMetadata
	if json.Unmarshal(data, &metadata) != nil {
		return publicSkillInstallMetadata{}, false
	}
	if !skill.IsValidName(strings.TrimSpace(metadata.Slug)) {
		return publicSkillInstallMetadata{}, false
	}
	return metadata, true
}

func installedSkillDisplayName(skillPath, canonicalName string, metadata publicSkillInstallMetadata) string {
	if name := strings.TrimSpace(metadata.Name); name != "" {
		return name
	}
	if strings.TrimSpace(skillPath) == "" || skillPath == "(builtin)" {
		return ""
	}
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return ""
	}
	fm, _ := frontmatter.Split(strings.TrimPrefix(string(data), "\uFEFF"))
	name := strings.TrimSpace(fm["name"])
	if name == "" || strings.EqualFold(name, strings.TrimSpace(canonicalName)) {
		return ""
	}
	return name
}

func extractPublicSkillArchive(archive []byte, destination string) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("技能包不是有效的 ZIP 文件: %w", err)
	}
	prefix, err := publicSkillArchivePrefix(reader.File)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	var extractedBytes uint64
	for _, entry := range reader.File {
		name := filepath.ToSlash(entry.Name)
		rel, ok := trimArchivePrefix(name, prefix)
		if !ok || rel == "" {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("技能包包含不安全路径 %q", entry.Name)
		}
		target := filepath.Join(destination, clean)
		if !pathWithin(destination, target) {
			return fmt.Errorf("技能包包含越界路径 %q", entry.Name)
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || (!mode.IsRegular() && !entry.FileInfo().IsDir()) {
			return fmt.Errorf("技能包包含不支持的文件 %q", entry.Name)
		}
		if entry.UncompressedSize64 > publicSkillArchiveLimit || extractedBytes+entry.UncompressedSize64 > publicSkillArchiveLimit {
			return fmt.Errorf("技能包解压后超过大小限制")
		}
		extractedBytes += entry.UncompressedSize64
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(file, io.LimitReader(source, publicSkillArchiveLimit+1))
		closeErr := errors.Join(source.Close(), file.Close())
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
	}
	return nil
}

func publicSkillArchivePrefix(entries []*zip.File) (string, error) {
	candidates := make([]string, 0, 1)
	minDepth := int(^uint(0) >> 1)
	for _, entry := range entries {
		rawName := filepath.ToSlash(entry.Name)
		if !safeArchivePath(rawName) {
			return "", fmt.Errorf("技能包包含不安全路径 %q", entry.Name)
		}
		name := strings.Trim(rawName, "/")
		if !strings.EqualFold(filepath.Base(name), skill.SkillFile) {
			continue
		}
		depth := strings.Count(name, "/")
		if depth < minDepth {
			minDepth = depth
			candidates = candidates[:0]
		}
		if depth == minDepth {
			dir := ""
			if index := strings.LastIndex(name, "/"); index >= 0 {
				dir = name[:index]
			}
			candidates = append(candidates, dir)
		}
	}
	if len(candidates) != 1 {
		return "", fmt.Errorf("技能包必须包含一个明确的 SKILL.md")
	}
	return strings.Trim(candidates[0], "/"), nil
}

func safeArchivePath(name string) bool {
	if name == "" || filepath.IsAbs(filepath.FromSlash(name)) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func trimArchivePrefix(name, prefix string) (string, bool) {
	name = strings.TrimLeft(filepath.ToSlash(name), "/")
	if prefix == "" {
		return name, true
	}
	prefix += "/"
	if !strings.HasPrefix(name, prefix) {
		return "", false
	}
	return strings.TrimPrefix(name, prefix), true
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
