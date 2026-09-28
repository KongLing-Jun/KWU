package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kwu/internal/control"
	"kwu/internal/project"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

const projectAssetToolName = "project_assets"

var projectAssetToolSchema = json.RawMessage(`{
  "type":"object",
  "required":["action"],
  "additionalProperties":false,
  "properties":{
    "action":{"type":"string","enum":["list","get_download_url","download_to_workspace","upload","replace_content","update_asset","delete_asset","create_folder","update_folder","delete_folder"]},
    "asset_id":{"type":"string","description":"Asset ID for URL retrieval, workspace download, replace, update, or delete."},
    "folder_id":{"type":"string","description":"Folder to list, upload into, or move an asset into."},
    "parent_folder_id":{"type":"string","description":"Parent folder for folder creation or movement."},
    "move_to_root":{"type":"boolean","description":"Move an asset or folder to the project root."},
    "expected_version":{"type":"integer","minimum":1,"description":"Required optimistic version for replace, update, and folder deletion operations."},
    "name":{"type":"string","description":"New asset/folder name, or optional uploaded filename."},
    "source_path":{"type":"string","description":"Workspace-contained local file path for upload or content replacement."},
    "search":{"type":"string"},
    "type":{"type":"string","enum":["all","folder","image","video","pdf","audio","document","markdown","archive","other"]},
    "sort":{"type":"string","enum":["name","updated_at"]},
    "direction":{"type":"string","enum":["asc","desc"]},
    "cursor":{"type":"string"},
    "limit":{"type":"integer","minimum":1,"maximum":100}
  }
}`)

type projectAssetToolGateway interface {
	GetProject(context.Context, string) (project.ProjectResult, error)
	ListAssetDirectory(context.Context, string, project.DirectoryQuery) (string, project.ProjectAssetDirectoryPage, error)
	AssetDownload(context.Context, string, string) (string, project.ProjectAssetDownload, error)
	UploadAssetFileToFolder(context.Context, string, string, string, *string) (string, project.ProjectAsset, error)
	ReplaceAssetFileContent(context.Context, string, string, string, string, uint64) (string, project.ProjectAsset, error)
	PatchAsset(context.Context, string, string, project.PatchProjectAssetInput) (string, error)
	DeleteAsset(context.Context, string, string) (string, error)
	CreateAssetFolder(context.Context, string, project.CreateProjectAssetFolderInput) (string, error)
	PatchAssetFolder(context.Context, string, string, project.PatchProjectAssetFolderInput) (string, error)
	DeleteAssetFolder(context.Context, string, string, uint64) (string, error)
}

type projectAssetToolAccess struct {
	ctx     context.Context
	gateway projectAssetToolGateway
	cancel  context.CancelFunc
}

type projectAssetTool struct {
	projectID     string
	workspaceRoot string
	authorize     func(context.Context) (projectAssetToolAccess, error)
	newHTTPClient func(string) (*http.Client, error)
}

type projectAssetToolDownloadResult struct {
	AssetID      string `json:"asset_id"`
	Filename     string `json:"filename"`
	RelativePath string `json:"relative_path"`
}

type projectAssetToolArgs struct {
	Action          string  `json:"action"`
	AssetID         string  `json:"asset_id"`
	FolderID        string  `json:"folder_id"`
	ParentFolderID  string  `json:"parent_folder_id"`
	MoveToRoot      bool    `json:"move_to_root"`
	ExpectedVersion uint64  `json:"expected_version"`
	Name            *string `json:"name"`
	SourcePath      string  `json:"source_path"`
	Search          string  `json:"search"`
	Type            string  `json:"type"`
	Sort            string  `json:"sort"`
	Direction       string  `json:"direction"`
	Cursor          string  `json:"cursor"`
	Limit           int     `json:"limit"`
}

func (projectAssetTool) Name() string { return projectAssetToolName }

func (projectAssetTool) Description() string {
	return "Manage OSS-backed assets for the current Cloud project only. Use list to inspect files and folders; download_to_workspace to download an asset by ID before reading it with local file tools; get_download_url only when a URL is explicitly needed; upload or replace_content for workspace files; update_asset/delete_asset and folder actions for mutations. The project ID is pinned by the host and must not be supplied. Always list first and use the returned IDs and versions."
}

func (projectAssetTool) Schema() json.RawMessage { return projectAssetToolSchema }

// The tool includes destructive actions. Keep every invocation behind the
// writer approval path even when the selected action is a read.
func (projectAssetTool) ReadOnly() bool { return false }

func (t projectAssetTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decodeProjectAssetToolArgs(raw)
	if err != nil {
		return "", err
	}
	if t.projectID == "" || t.authorize == nil {
		return "", fmt.Errorf("project_assets: project session is unavailable")
	}
	access, err := t.authorize(ctx)
	if err != nil {
		return "", err
	}
	if access.cancel != nil {
		defer access.cancel()
	}
	if access.ctx == nil {
		access.ctx = ctx
	}
	if access.gateway == nil {
		return "", fmt.Errorf("project_assets: project asset gateway is unavailable")
	}
	result, err := t.execute(access.ctx, access.gateway, args)
	if err != nil {
		return "", fmt.Errorf("project_assets %s: %w", args.Action, err)
	}
	encoded, err := marshalProjectAssetToolResult(result)
	if err != nil {
		return "", fmt.Errorf("project_assets: encode result: %w", err)
	}
	return encoded, nil
}

func marshalProjectAssetToolResult(result any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buffer.String(), "\n"), nil
}

func decodeProjectAssetToolArgs(raw json.RawMessage) (projectAssetToolArgs, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var args projectAssetToolArgs
	if err := decoder.Decode(&args); err != nil {
		return projectAssetToolArgs{}, fmt.Errorf("project_assets: invalid arguments: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return projectAssetToolArgs{}, fmt.Errorf("project_assets: invalid trailing arguments")
	}
	args.Action = strings.TrimSpace(args.Action)
	if args.Action == "" {
		return projectAssetToolArgs{}, fmt.Errorf("project_assets: action is required")
	}
	return args, nil
}

func (t projectAssetTool) execute(ctx context.Context, gateway projectAssetToolGateway, args projectAssetToolArgs) (any, error) {
	switch args.Action {
	case "list":
		limit := args.Limit
		if limit == 0 {
			limit = 100
		}
		_, page, err := gateway.ListAssetDirectory(ctx, t.projectID, project.DirectoryQuery{
			FolderID: strings.TrimSpace(args.FolderID), Search: args.Search, Type: args.Type,
			Sort: args.Sort, Direction: args.Direction, Cursor: args.Cursor, Limit: limit,
		})
		return page, err
	case "get_download_url":
		if err := requireProjectAssetToolID("asset_id", args.AssetID); err != nil {
			return nil, err
		}
		_, item, err := gateway.AssetDownload(ctx, t.projectID, args.AssetID)
		return item, err
	case "download_to_workspace":
		if err := requireProjectAssetToolID("asset_id", args.AssetID); err != nil {
			return nil, err
		}
		return t.downloadToWorkspace(ctx, gateway, args.AssetID)
	case "upload":
		path, filename, err := projectAssetToolSource(t.workspaceRoot, args.SourcePath, args.Name)
		if err != nil {
			return nil, err
		}
		folderID := optionalProjectAssetToolID(args.FolderID)
		if err := acquireProjectAssetUpload(ctx); err != nil {
			return nil, err
		}
		defer releaseProjectAssetUpload()
		_, item, err := gateway.UploadAssetFileToFolder(ctx, t.projectID, path, filename, folderID)
		return item, err
	case "replace_content":
		if err := requireProjectAssetToolID("asset_id", args.AssetID); err != nil {
			return nil, err
		}
		if args.ExpectedVersion == 0 {
			return nil, fmt.Errorf("expected_version is required")
		}
		path, filename, err := projectAssetToolSource(t.workspaceRoot, args.SourcePath, args.Name)
		if err != nil {
			return nil, err
		}
		if err := acquireProjectAssetUpload(ctx); err != nil {
			return nil, err
		}
		defer releaseProjectAssetUpload()
		_, item, err := gateway.ReplaceAssetFileContent(ctx, t.projectID, args.AssetID, path, filename, args.ExpectedVersion)
		return item, err
	case "update_asset":
		if err := requireProjectAssetToolID("asset_id", args.AssetID); err != nil {
			return nil, err
		}
		if args.ExpectedVersion == 0 {
			return nil, fmt.Errorf("expected_version is required")
		}
		if args.MoveToRoot && strings.TrimSpace(args.FolderID) != "" {
			return nil, fmt.Errorf("folder_id and move_to_root are mutually exclusive")
		}
		folder := project.NullableStringUpdate{}
		if args.MoveToRoot {
			folder.Set = true
		} else if value := optionalProjectAssetToolID(args.FolderID); value != nil {
			folder.Set, folder.Value = true, value
		}
		if args.Name == nil && !folder.Set {
			return nil, fmt.Errorf("name, folder_id, or move_to_root is required")
		}
		_, err := gateway.PatchAsset(ctx, t.projectID, args.AssetID, project.PatchProjectAssetInput{ExpectedVersion: args.ExpectedVersion, Name: args.Name, FolderID: folder})
		return projectAssetMutationResult(args.Action, args.AssetID), err
	case "delete_asset":
		if err := requireProjectAssetToolID("asset_id", args.AssetID); err != nil {
			return nil, err
		}
		_, err := gateway.DeleteAsset(ctx, t.projectID, args.AssetID)
		return projectAssetMutationResult(args.Action, args.AssetID), err
	case "create_folder":
		if args.Name == nil || strings.TrimSpace(*args.Name) == "" {
			return nil, fmt.Errorf("name is required")
		}
		_, err := gateway.CreateAssetFolder(ctx, t.projectID, project.CreateProjectAssetFolderInput{Name: *args.Name, ParentFolderID: optionalProjectAssetToolID(args.ParentFolderID)})
		return projectAssetMutationResult(args.Action, ""), err
	case "update_folder":
		if err := requireProjectAssetToolID("folder_id", args.FolderID); err != nil {
			return nil, err
		}
		if args.ExpectedVersion == 0 {
			return nil, fmt.Errorf("expected_version is required")
		}
		if args.MoveToRoot && strings.TrimSpace(args.ParentFolderID) != "" {
			return nil, fmt.Errorf("parent_folder_id and move_to_root are mutually exclusive")
		}
		parent := project.NullableStringUpdate{}
		if args.MoveToRoot {
			parent.Set = true
		} else if value := optionalProjectAssetToolID(args.ParentFolderID); value != nil {
			parent.Set, parent.Value = true, value
		}
		if args.Name == nil && !parent.Set {
			return nil, fmt.Errorf("name, parent_folder_id, or move_to_root is required")
		}
		_, err := gateway.PatchAssetFolder(ctx, t.projectID, args.FolderID, project.PatchProjectAssetFolderInput{ExpectedVersion: args.ExpectedVersion, Name: args.Name, ParentFolderID: parent})
		return projectAssetMutationResult(args.Action, args.FolderID), err
	case "delete_folder":
		if err := requireProjectAssetToolID("folder_id", args.FolderID); err != nil {
			return nil, err
		}
		if args.ExpectedVersion == 0 {
			return nil, fmt.Errorf("expected_version is required")
		}
		_, err := gateway.DeleteAssetFolder(ctx, t.projectID, args.FolderID, args.ExpectedVersion)
		return projectAssetMutationResult(args.Action, args.FolderID), err
	default:
		return nil, fmt.Errorf("unsupported action %q", args.Action)
	}
}

func (t projectAssetTool) downloadToWorkspace(ctx context.Context, gateway projectAssetToolGateway, assetID string) (projectAssetToolDownloadResult, error) {
	workspaceRoot, err := projectAssetToolWorkspaceRoot(t.workspaceRoot)
	if err != nil {
		return projectAssetToolDownloadResult{}, err
	}
	_, item, err := gateway.AssetDownload(ctx, t.projectID, assetID)
	if err != nil {
		return projectAssetToolDownloadResult{}, err
	}
	newClient := t.newHTTPClient
	if newClient == nil {
		newClient = newPublicCloudHTTPClient
	}
	client, err := newClient(item.URL)
	if err != nil {
		return projectAssetToolDownloadResult{}, fmt.Errorf("create asset download client: %w", err)
	}
	savedPath, err := downloadProjectAssetFile(ctx, client, workspaceRoot, item.Filename, item.URL)
	if err != nil {
		return projectAssetToolDownloadResult{}, err
	}
	if !projectAssetPathWithin(workspaceRoot, savedPath) {
		_ = os.Remove(savedPath)
		return projectAssetToolDownloadResult{}, fmt.Errorf("downloaded asset path is outside the current workspace")
	}
	relativePath, err := filepath.Rel(workspaceRoot, savedPath)
	if err != nil || filepath.IsAbs(relativePath) {
		_ = os.Remove(savedPath)
		return projectAssetToolDownloadResult{}, fmt.Errorf("resolve downloaded asset path")
	}
	return projectAssetToolDownloadResult{
		AssetID: item.AssetID, Filename: filepath.Base(savedPath), RelativePath: filepath.ToSlash(relativePath),
	}, nil
}

func projectAssetMutationResult(action, id string) map[string]any {
	result := map[string]any{"ok": true, "action": action}
	if id != "" {
		result["id"] = id
	}
	return result
}

func requireProjectAssetToolID(name, value string) error {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func optionalProjectAssetToolID(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func projectAssetToolSource(workspaceRoot, sourcePath string, requestedName *string) (string, string, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	sourcePath = strings.TrimSpace(sourcePath)
	if workspaceRoot == "" || sourcePath == "" {
		return "", "", fmt.Errorf("source_path requires a bound workspace")
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace root: %w", err)
	}
	path := sourcePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve source_path: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve source_path: %w", err)
	}
	if !projectAssetPathWithin(root, path) {
		return "", "", fmt.Errorf("source_path is outside the current workspace")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("inspect source_path: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > projectAssetMaxFileBytes {
		return "", "", fmt.Errorf("source_path must be a non-empty regular file no larger than 100 MiB")
	}
	filename := filepath.Base(path)
	if requestedName != nil {
		filename = strings.TrimSpace(*requestedName)
		if filename == "" || filename == "." || filename == ".." || filepath.Base(filename) != filename {
			return "", "", fmt.Errorf("name must be a filename without directory components")
		}
	}
	return path, filename, nil
}

func projectAssetToolWorkspaceRoot(workspaceRoot string) (string, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return "", fmt.Errorf("download requires a bound workspace")
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root is not a directory")
	}
	return root, nil
}

func projectAssetPathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	if runtime.GOOS == "windows" {
		rel = strings.ToLower(rel)
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func (a *App) cloudProjectSessionTools(_ context.Context, actor userctx.ActorContext, workspaceRoot, sessionPath string) ([]tool.Tool, error) {
	meta, bound, err := loadCloudProjectSessionMeta(sessionPath)
	if err != nil {
		return nil, err
	}
	if !bound || meta.ProjectContextPermanentlyReadOnly {
		return nil, nil
	}
	services, features, err := a.projectServicesAndFeaturesForOperation()
	if err != nil {
		return nil, err
	}
	if !features.Assets {
		return nil, nil
	}
	pin := *meta.ProjectContext
	if services.Actor.Normalize().ProfileID != actor.Normalize().ProfileID || !projectContextPinMatchesScope(pin, services.Scope) {
		return nil, fmt.Errorf("project asset tool is outside the current cloud scope")
	}
	assetTool := projectAssetTool{
		projectID: pin.ProjectID, workspaceRoot: workspaceRoot,
		authorize: func(callCtx context.Context) (projectAssetToolAccess, error) {
			current, currentFeatures, currentErr := a.projectServicesAndFeaturesForOperation()
			if currentErr != nil {
				return projectAssetToolAccess{}, currentErr
			}
			if !currentFeatures.Assets || current.Actor.Normalize().ProfileID != actor.Normalize().ProfileID || !projectContextPinMatchesScope(pin, current.Scope) {
				return projectAssetToolAccess{}, fmt.Errorf("project asset tool authorization changed")
			}
			operationCtx, cancel := current.operationContext(callCtx)
			result, getErr := current.Gateway.GetProject(operationCtx, pin.ProjectID)
			if getErr != nil || result.Project.Status != project.ProjectActive {
				cancel()
				return projectAssetToolAccess{}, fmt.Errorf("active project asset access is unavailable")
			}
			return projectAssetToolAccess{ctx: operationCtx, gateway: current.Gateway, cancel: cancel}, nil
		},
	}
	return []tool.Tool{assetTool}, nil
}

func chainSessionToolProviders(first, second control.SessionToolProvider) control.SessionToolProvider {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return func(ctx context.Context, sessionPath string) ([]tool.Tool, error) {
		left, err := first(ctx, sessionPath)
		if err != nil {
			return nil, err
		}
		right, err := second(ctx, sessionPath)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}
}

var _ tool.Tool = projectAssetTool{}
var _ projectAssetToolGateway = (*project.ProjectGateway)(nil)
