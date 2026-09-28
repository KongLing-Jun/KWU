package main

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/brand"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/expert"
)

//go:embed frontend/src/assets/experts/code-review.png
var defaultUserExpertAvatar []byte

type ExpertMemberInfo struct {
	Kind      string `json:"kind,omitempty"`
	ExpertID  string `json:"expertId,omitempty"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Task      string `json:"task"`
	Output    string `json:"output"`
	Tone      string `json:"tone"`
	AvatarKey string `json:"avatarKey,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type ExpertInfo struct {
	ID          string             `json:"id"`
	Source      string             `json:"source"`
	Mode        string             `json:"mode"`
	Name        string             `json:"name"`
	Role        string             `json:"role"`
	Description string             `json:"description"`
	Category    string             `json:"category,omitempty"`
	Categories  []string           `json:"categories"`
	Tags        []string           `json:"tags"`
	IconKey     string             `json:"iconKey"`
	Tone        string             `json:"tone"`
	AvatarKey   string             `json:"avatarKey,omitempty"`
	AvatarURL   string             `json:"avatarUrl,omitempty"`
	Members     []ExpertMemberInfo `json:"members"`
	Prompts     []string           `json:"prompts"`
	Language    string             `json:"language,omitempty"`
	Enabled     bool               `json:"enabled"`
	CreatedAt   int64              `json:"createdAt,omitempty"`
	UpdatedAt   int64              `json:"updatedAt,omitempty"`
}

type ExpertListFilter struct {
	Mode     string `json:"mode,omitempty"`
	Category string `json:"category,omitempty"`
	Query    string `json:"query,omitempty"`
}

type ExpertResetResult struct {
	Experts      []ExpertInfo              `json:"experts"`
	UserExperts  []expert.UserExpertInfo   `json:"userExperts"`
	LibraryState expert.ExpertLibraryState `json:"libraryState"`
}

func (a *App) ListExperts() ([]ExpertInfo, error) {
	if a.usesCloudExpertAssets() {
		return a.listCloudExperts()
	}
	store, err := a.expertStore()
	if err != nil {
		return nil, err
	}
	defs, err := store.List(a.bootContext())
	if err != nil {
		return nil, err
	}
	return expertInfos(defs), nil
}

func (a *App) ListExpertsByFilter(input ExpertListFilter) ([]ExpertInfo, error) {
	filter, err := expertListFilter(input)
	if err != nil {
		return nil, err
	}
	if a.usesCloudExpertAssets() {
		items, err := a.listCloudExperts()
		if err != nil {
			return nil, err
		}
		return filterExpertInfos(items, filter), nil
	}
	store, err := a.expertStore()
	if err != nil {
		return nil, err
	}
	defs, err := store.ListFiltered(a.bootContext(), filter)
	if err != nil {
		return nil, err
	}
	return expertInfos(defs), nil
}

func (a *App) CreateExpert(input ExpertInfo) (ExpertInfo, error) {
	if a.usesCloudExpertAssets() {
		return ExpertInfo{}, fmt.Errorf("cloud expert catalog is managed by the service")
	}
	if strings.TrimSpace(input.ID) == "" {
		input.ID = fmt.Sprintf("custom-%d", time.Now().UnixMilli())
	}
	if expert.IsBuiltinID(strings.TrimSpace(input.ID)) {
		return ExpertInfo{}, fmt.Errorf("official expert %q is read-only", strings.TrimSpace(input.ID))
	}
	runtimes := a.currentExpertRuntimes()
	input.Source = expert.SourceUser
	store, err := a.expertStore()
	if err != nil {
		return ExpertInfo{}, err
	}
	def, err := expertDefinition(input)
	if err != nil {
		return ExpertInfo{}, err
	}
	existing, err := store.List(a.bootContext())
	if err != nil {
		return ExpertInfo{}, err
	}
	for _, item := range existing {
		if item.ID == def.ID {
			return ExpertInfo{}, fmt.Errorf("expert %q already exists", def.ID)
		}
	}
	if err := store.Save(a.bootContext(), def); err != nil {
		return ExpertInfo{}, err
	}
	reloadExpertRuntimes(runtimes, nil)
	return a.findExpert(def.ID)
}

func (a *App) UpdateExpert(id string, input ExpertInfo) (ExpertInfo, error) {
	if a.usesCloudExpertAssets() {
		return ExpertInfo{}, fmt.Errorf("cloud expert catalog is managed by the service")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ExpertInfo{}, fmt.Errorf("expert id is required")
	}
	if expert.IsBuiltinID(id) {
		return ExpertInfo{}, fmt.Errorf("official expert %q is read-only", id)
	}
	runtimes := a.currentExpertRuntimes()
	if err := ensureExpertRuntimesIdle(runtimes, id, false); err != nil {
		return ExpertInfo{}, err
	}
	input.ID = id
	input.Source = expert.SourceUser
	store, err := a.expertStore()
	if err != nil {
		return ExpertInfo{}, err
	}
	existing, err := store.List(a.bootContext())
	if err != nil {
		return ExpertInfo{}, err
	}
	found := false
	for _, item := range existing {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		return ExpertInfo{}, fmt.Errorf("expert %q was not found", id)
	}
	def, err := expertDefinition(input)
	if err != nil {
		return ExpertInfo{}, err
	}
	if err := store.Save(a.bootContext(), def); err != nil {
		return ExpertInfo{}, err
	}
	if err := refreshExpertRuntimes(runtimes, id, nil); err != nil {
		return ExpertInfo{}, err
	}
	return a.findExpert(id)
}

func (a *App) DeleteExpert(id string) error {
	if a.usesCloudExpertAssets() {
		return fmt.Errorf("cloud expert catalog is managed by the service")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("expert id is required")
	}
	if expert.IsBuiltinID(id) {
		return fmt.Errorf("official expert %q is read-only", id)
	}
	runtimes := a.currentExpertRuntimes()
	if err := ensureExpertRuntimesIdle(runtimes, id, false); err != nil {
		return err
	}
	if err := clearExpertRuntimes(runtimes, id, false); err != nil {
		return err
	}
	store, err := a.expertStore()
	if err != nil {
		return err
	}
	if err := store.Delete(a.bootContext(), id); err != nil {
		return err
	}
	reloadExpertRuntimes(runtimes, nil)
	return expert.RemoveExpertLibraryID(config.UserExpertsPath(), id)
}

func (a *App) ResetExperts() (ExpertResetResult, error) {
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	runtimes := a.expertRuntimesFor(ctrl)
	if err := ensureExpertRuntimesIdle(runtimes, "", true); err != nil {
		return ExpertResetResult{}, err
	}
	var infos []ExpertInfo
	if a.usesCloudExpertAssets() {
		infos = expertInfos(a.cloudExpertDefinitionsSnapshot())
	} else {
		store, err := a.expertStore()
		if err != nil {
			return ExpertResetResult{}, err
		}
		if err := store.DeleteUserDefinitions(a.bootContext()); err != nil {
			return ExpertResetResult{}, err
		}
		defs, listErr := store.List(a.bootContext())
		if listErr != nil {
			return ExpertResetResult{}, listErr
		}
		infos = expertInfos(defs)
	}
	userExpertPath := config.UserExpertsPath()
	if err := expert.ResetUserExpertStore(userExpertPath); err != nil {
		return ExpertResetResult{}, err
	}

	for _, runtime := range runtimes {
		runtime.ReloadExpertCatalog()
	}
	if err := clearExpertRuntimes(runtimes, "", true); err != nil {
		return ExpertResetResult{}, err
	}
	users, err := expert.LoadUserExperts(userExpertPath)
	if err != nil {
		return ExpertResetResult{}, err
	}
	libraryState, err := expert.LoadExpertLibraryState(userExpertPath)
	if err != nil {
		return ExpertResetResult{}, err
	}
	return ExpertResetResult{
		Experts:      infos,
		UserExperts:  userExpertInfos(users),
		LibraryState: libraryState,
	}, nil
}

func (a *App) expertStore() (*expert.Store, error) {
	dir := config.MemoryUserDir()
	if dir == "" {
		return nil, fmt.Errorf("%s user state directory is unavailable", brand.EnglishName)
	}
	path := filepath.Join(dir, "experts.sqlite")
	a.expertStoreMu.Lock()
	defer a.expertStoreMu.Unlock()
	if a.expertStoreInstance != nil && a.expertStorePath == path {
		return a.expertStoreInstance, nil
	}
	if a.expertStoreInstance != nil {
		_ = a.expertStoreInstance.Close()
	}
	a.expertStoreInstance = expert.NewPersistentStore(path, expert.BuiltinDefinitions())
	a.expertStorePath = path
	return a.expertStoreInstance, nil
}

func (a *App) closeExpertStore() {
	a.expertStoreMu.Lock()
	store := a.expertStoreInstance
	a.expertStoreInstance = nil
	a.expertStorePath = ""
	a.expertStoreMu.Unlock()
	if store != nil {
		_ = store.Close()
	}
}

func (a *App) usesCloudExpertAssets() bool {
	if workBuddyRCTestMode() {
		return false
	}
	return a != nil && a.cloudRuntime != nil && a.cloudRuntime.expertAssets != nil
}

// listCloudExperts always serves the system expert catalog from the remote
// response and updates the process-local runtime snapshot. Cloud definitions
// are never persisted to the local SQLite expert store.
func (a *App) listCloudExperts() ([]ExpertInfo, error) {
	a.expertCatalogSyncMu.Lock()
	defer a.expertCatalogSyncMu.Unlock()

	assets, err := a.cloudRuntime.expertAssets.ListSystem(a.bootContext())
	if err != nil {
		return nil, cloudWailsError("cloud.listExperts", err)
	}
	definitions, infos := cloudExpertCatalog(assets)
	a.replaceCloudExpertDefinitions(definitions)
	reloadExpertRuntimes(a.currentExpertRuntimes(), nil)
	return infos, nil
}

func (a *App) refreshExpertCatalog(ctx context.Context) error {
	a.expertCatalogSyncMu.Lock()
	defer a.expertCatalogSyncMu.Unlock()

	definitions := expert.BuiltinDefinitions()
	if a.usesCloudExpertAssets() {
		assets, requestErr := a.cloudRuntime.expertAssets.ListSystem(ctx)
		if requestErr != nil {
			return cloudWailsError("cloud.refreshExpertCatalog", requestErr)
		}
		definitions, _ = cloudExpertCatalog(assets)
		a.replaceCloudExpertDefinitions(definitions)
		reloadExpertRuntimes(a.currentExpertRuntimes(), nil)
		return nil
	}
	store, err := a.expertStore()
	if err != nil {
		return err
	}
	if err := store.ReplaceOfficial(ctx, definitions); err != nil {
		return err
	}
	reloadExpertRuntimes(a.currentExpertRuntimes(), nil)
	return nil
}

func (a *App) listCloudUserExperts(ctrl control.SessionAPI) ([]expert.UserExpertInfo, error) {
	runtime, scope, end, err := a.cloudExpertAssetsOperation()
	if err != nil {
		return nil, err
	}
	defer end()
	userAssets, ok := runtime.expertAssets.(cloud.UserExpertAssetsClient)
	if !ok {
		return nil, fmt.Errorf("cloud user expert assets are unavailable")
	}
	userExperts, err := userAssets.ListUserExperts(a.bootContext(), scope)
	if err != nil {
		return nil, cloudWailsError("cloud.listUserExperts", err)
	}
	userGroups, err := userAssets.ListUserExpertGroups(a.bootContext(), scope)
	if err != nil {
		return nil, cloudWailsError("cloud.listUserExperts", err)
	}
	records, err := cloudUserExpertRecords(userExperts, userGroups)
	if err != nil {
		return nil, err
	}
	if ctrl == nil {
		return userExpertInfos(records), nil
	}
	store, err := desktopUserExpertStoreFor(ctrl)
	if err != nil {
		return nil, err
	}
	if err := expert.SaveUserExperts(store.UserExpertStorePath(), records); err != nil {
		return nil, fmt.Errorf("save cloud user expert mirror: %w", err)
	}
	store.ReloadExpertCatalog()
	reloadExpertRuntimes(a.expertRuntimesFor(ctrl), ctrl)
	return userExpertInfos(records), nil
}

func (a *App) cloudExpertLibraryState(ctrl control.SessionAPI) (expert.ExpertLibraryState, error) {
	runtime, scope, end, err := a.cloudExpertAssetsOperation()
	if err != nil {
		return expert.ExpertLibraryState{}, err
	}
	defer end()
	favorites, ok := runtime.expertAssets.(cloud.ExpertFavoritesClient)
	if !ok {
		return expert.ExpertLibraryState{}, fmt.Errorf("cloud expert favorites are unavailable")
	}
	return loadCloudExpertLibraryState(a.bootContext(), favorites, scope, localExpertLibraryState(ctrl))
}

func (a *App) setCloudExpertFavorite(ctrl control.SessionAPI, id string, favorite bool) (expert.ExpertLibraryState, error) {
	id = strings.TrimSpace(id)
	runtime, scope, end, err := a.cloudExpertAssetsOperation()
	if err != nil {
		return expert.ExpertLibraryState{}, err
	}
	defer end()
	favorites, ok := runtime.expertAssets.(cloud.ExpertFavoritesClient)
	if !ok {
		return expert.ExpertLibraryState{}, fmt.Errorf("cloud expert favorites are unavailable")
	}

	switch {
	case strings.HasPrefix(id, "exg_"):
		if favorite {
			if _, err := favorites.CreateExpertGroupFavorite(a.bootContext(), scope, id); err != nil {
				return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
			}
		} else {
			items, err := favorites.ListExpertGroupFavorites(a.bootContext(), scope)
			if err != nil {
				return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
			}
			for _, item := range items {
				if item.ExpertGroupID == id {
					if err := favorites.DeleteExpertGroupFavorite(a.bootContext(), scope, item.FavoriteID); err != nil {
						return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
					}
					break
				}
			}
		}
	case strings.HasPrefix(id, "exp_"):
		if favorite {
			if _, err := favorites.CreateExpertFavorite(a.bootContext(), scope, id); err != nil {
				return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
			}
		} else {
			items, err := favorites.ListExpertFavorites(a.bootContext(), scope)
			if err != nil {
				return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
			}
			for _, item := range items {
				if item.ExpertID == id {
					if err := favorites.DeleteExpertFavorite(a.bootContext(), scope, item.FavoriteID); err != nil {
						return expert.ExpertLibraryState{}, cloudWailsError("cloud.setExpertFavorite", err)
					}
					break
				}
			}
		}
	default:
		return expert.ExpertLibraryState{}, fmt.Errorf("cloud expert id %q is invalid", id)
	}

	return loadCloudExpertLibraryState(a.bootContext(), favorites, scope, localExpertLibraryState(ctrl))
}

func loadCloudExpertLibraryState(
	ctx context.Context,
	client cloud.ExpertFavoritesClient,
	scope cloud.BindingScope,
	localState expert.ExpertLibraryState,
) (expert.ExpertLibraryState, error) {
	expertFavorites, err := client.ListExpertFavorites(ctx, scope)
	if err != nil {
		return expert.ExpertLibraryState{}, cloudWailsError("cloud.loadExpertLibrary", err)
	}
	groupFavorites, err := client.ListExpertGroupFavorites(ctx, scope)
	if err != nil {
		return expert.ExpertLibraryState{}, cloudWailsError("cloud.loadExpertLibrary", err)
	}
	favoriteIDs := make([]string, 0, len(expertFavorites)+len(groupFavorites))
	for _, item := range expertFavorites {
		favoriteIDs = append(favoriteIDs, item.ExpertID)
	}
	for _, item := range groupFavorites {
		favoriteIDs = append(favoriteIDs, item.ExpertGroupID)
	}
	return expert.ExpertLibraryState{
		FavoriteIDs: favoriteIDs,
		RecentIDs:   append([]string{}, localState.RecentIDs...),
		UsageCounts: cloneExpertUsageCounts(localState.UsageCounts),
	}, nil
}

func localExpertLibraryState(ctrl control.SessionAPI) expert.ExpertLibraryState {
	if ctrl == nil {
		return expert.ExpertLibraryState{}
	}
	return ctrl.ExpertLibraryState()
}

func cloneExpertUsageCounts(counts map[string]int64) map[string]int64 {
	if len(counts) == 0 {
		return nil
	}
	out := make(map[string]int64, len(counts))
	for id, count := range counts {
		out[id] = count
	}
	return out
}

func cloudUserExpertRecords(
	userExperts []cloud.UserExpertAsset,
	userGroups []cloud.UserExpertGroupAsset,
) ([]expert.UserExpert, error) {
	type memberDetails struct {
		name      string
		prompt    string
		avatarURL string
	}
	membersByID := make(map[string]memberDetails, len(userExperts))
	records := make([]expert.UserExpert, 0, len(userExperts)+len(userGroups))
	for _, asset := range userExperts {
		if _, duplicate := membersByID[asset.ExpertID]; duplicate {
			return nil, fmt.Errorf("cloud user experts contain duplicate id %q", asset.ExpertID)
		}
		membersByID[asset.ExpertID] = memberDetails{name: asset.Name, prompt: asset.Prompt, avatarURL: asset.AvatarURL}
		record, err := expert.NewUserExpert(asset.ExpertID, expert.UserExpertInput{
			Name: asset.Name, Mode: string(expert.ModeExpert), Description: asset.Description,
			Category: asset.Category, Tags: append([]string(nil), asset.Tags...), AvatarKey: asset.AvatarURL,
			SystemPrompt: asset.Prompt, Prompts: cloudUserExpertPrompts(asset.Name),
			Language: "auto", Enabled: true,
		}, asset.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("map cloud user expert %q: %w", asset.ExpertID, err)
		}
		if !asset.UpdatedAt.IsZero() {
			record.UpdatedAt = asset.UpdatedAt.UnixMilli()
		}
		records = append(records, record)
	}
	for _, asset := range userGroups {
		members := make([]expert.Member, 0, len(asset.ExpertIDs))
		for index, expertID := range asset.ExpertIDs {
			details := membersByID[expertID]
			name := strings.TrimSpace(details.name)
			if name == "" {
				name = expertID
			}
			kind := expert.MemberKindSub
			if index == 0 {
				kind = expert.MemberKindLead
			}
			task := strings.TrimSpace(details.prompt)
			if task == "" {
				task = "协作分析分配的任务并提交结论。"
			}
			members = append(members, expert.Member{
				Kind: kind, ExpertID: expertID, Name: name, Role: "专家团成员",
				Task: task, Output: "分析结论与可执行建议", Tone: "stone",
				AvatarKey: details.avatarURL,
			})
		}
		systemPrompt := fmt.Sprintf("你是%s。请协调专家团成员完成任务，汇总各成员结论、风险和下一步行动。", asset.Name)
		description := strings.TrimSpace(asset.Description)
		if description == "" {
			description = fmt.Sprintf("由 %d 位专家组成的用户专家团。", len(members))
		}
		record, err := expert.NewUserExpert(asset.ExpertGroupID, expert.UserExpertInput{
			Name: asset.Name, Mode: string(expert.ModeTeam), Description: description,
			Category: asset.Category, Tags: append([]string(nil), asset.Tags...), AvatarKey: asset.AvatarURL,
			Members: members, SystemPrompt: systemPrompt, Prompts: cloudUserExpertPrompts(asset.Name),
			Language: "auto", Enabled: true,
		}, asset.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("map cloud user expert group %q: %w", asset.ExpertGroupID, err)
		}
		if !asset.UpdatedAt.IsZero() {
			record.UpdatedAt = asset.UpdatedAt.UnixMilli()
		}
		records = append(records, record)
	}
	return records, nil
}

func cloudUserExpertPrompts(name string) []string {
	return []string{
		fmt.Sprintf("请以%s的身份处理当前任务。", name),
		fmt.Sprintf("请给出%s的关键判断与建议。", name),
		fmt.Sprintf("请用%s的方法整理下一步行动。", name),
	}
}

// cloudExpertAssetsOperation remains the authenticated boundary for user-owned
// expert POST operations. Public system catalog reads do not use it.
func (a *App) cloudExpertAssetsOperation() (*cloudRuntime, cloud.BindingScope, func(), error) {
	runtime, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return nil, cloud.BindingScope{}, nil, cloudWailsError("cloud.expertAssets", err)
	}
	auth, err := runtime.auth.State(a.bootContext(), actor)
	if err != nil {
		end()
		return nil, cloud.BindingScope{}, nil, cloudWailsError("cloud.expertAssets", err)
	}
	if auth.State != cloud.AccountAuthenticated && auth.State != cloud.AccountOffline {
		end()
		return nil, cloud.BindingScope{}, nil, cloudWailsError("cloud.expertAssets", &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "authentication is required"})
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		end()
		return nil, cloud.BindingScope{}, nil, cloudWailsError("cloud.expertAssets", &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "authentication is required"})
	}
	return runtime, scope, end, nil
}

func cloudExpertCatalog(assets cloud.SystemExpertAssets) ([]expert.Definition, []ExpertInfo) {
	definitions := make([]expert.Definition, 0, len(assets.Experts)+len(assets.ExpertGroups))
	avatars := make(map[string]string, len(assets.Experts)+len(assets.ExpertGroups))
	byID := make(map[string]cloud.SystemExpertAsset, len(assets.Experts))
	for _, item := range assets.Experts {
		category := strings.TrimSpace(item.Category)
		if category == "" {
			category = "系统专家"
		}
		byID[item.ExpertID] = item
		avatars[item.ExpertID] = item.AvatarURL
		definitions = append(definitions, expert.Definition{
			ID:           item.ExpertID,
			Source:       expert.SourceOfficial,
			Mode:         expert.ModeExpert,
			Name:         item.Name,
			Role:         category,
			Description:  item.Description,
			SystemPrompt: item.Prompt,
			Categories:   []string{category},
			Tags:         append([]string(nil), item.Tags...),
			IconKey:      "bot",
			Tone:         "stone",
			AvatarKey:    item.AvatarURL,
			Prompts:      cloudExpertPrompts(item.Name),
			CreatedAt:    cloudAssetUnixMilli(item.CreatedAt),
			UpdatedAt:    cloudAssetUnixMilli(item.UpdatedAt),
		})
	}
	for _, item := range assets.ExpertGroups {
		category := strings.TrimSpace(item.Category)
		if category == "" {
			category = "系统专家团"
		}
		members := make([]expert.Member, 0, len(item.ExpertIDs))
		for index, expertID := range item.ExpertIDs {
			member := byID[expertID]
			kind := expert.MemberKindSub
			if index == 0 {
				kind = expert.MemberKindLead
			}
			members = append(members, expert.Member{
				Kind:      kind,
				ExpertID:  expertID,
				Name:      member.Name,
				Role:      "系统专家",
				Task:      member.Prompt,
				Output:    "分析结论与可执行建议",
				Tone:      "stone",
				AvatarKey: member.AvatarURL,
			})
		}
		avatars[item.ExpertGroupID] = item.AvatarURL
		definitions = append(definitions, expert.Definition{
			ID:          item.ExpertGroupID,
			Source:      expert.SourceOfficial,
			Mode:        expert.ModeTeam,
			Name:        item.Name,
			Role:        category,
			Description: item.Description,
			Categories:  []string{category},
			Tags:        append([]string(nil), item.Tags...),
			IconKey:     "users",
			Tone:        "teal",
			AvatarKey:   item.AvatarURL,
			Members:     members,
			Prompts:     cloudExpertPrompts(item.Name),
			CreatedAt:   cloudAssetUnixMilli(item.CreatedAt),
			UpdatedAt:   cloudAssetUnixMilli(item.UpdatedAt),
		})
	}
	infos := expertInfos(definitions)
	for index := range infos {
		infos[index].AvatarURL = avatars[infos[index].ID]
		if infos[index].Mode != string(expert.ModeTeam) {
			continue
		}
		group := assets.ExpertGroups[index-len(assets.Experts)]
		for memberIndex, expertID := range group.ExpertIDs {
			infos[index].Members[memberIndex].AvatarURL = avatars[expertID]
		}
	}
	return definitions, infos
}

func cloudAssetUnixMilli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func cloudExpertPrompts(name string) []string {
	return []string{
		fmt.Sprintf("请以%s的视角分析当前任务。", name),
		fmt.Sprintf("请给出%s的关键判断与建议。", name),
		fmt.Sprintf("请用%s的方法整理下一步行动。", name),
	}
}

func (a *App) currentExpertRuntimes() []desktopExpertRuntime {
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	return a.expertRuntimesFor(ctrl)
}

func (a *App) findExpert(id string) (ExpertInfo, error) {
	list, err := a.ListExperts()
	if err != nil {
		return ExpertInfo{}, err
	}
	for _, item := range list {
		if item.ID == id {
			return item, nil
		}
	}
	return ExpertInfo{}, fmt.Errorf("expert %q was not found", id)
}

func expertDefinition(info ExpertInfo) (expert.Definition, error) {
	mode, err := expert.ParseMode(info.Mode)
	if err != nil {
		return expert.Definition{}, err
	}
	members := make([]expert.Member, 0, len(info.Members))
	for _, member := range info.Members {
		members = append(members, expert.Member{
			Kind:      member.Kind,
			ExpertID:  member.ExpertID,
			Name:      member.Name,
			Role:      member.Role,
			Task:      member.Task,
			Output:    member.Output,
			Tone:      member.Tone,
			AvatarKey: member.AvatarKey,
		})
	}
	return expert.Definition{
		ID:          info.ID,
		Source:      info.Source,
		Mode:        mode,
		Name:        info.Name,
		Role:        info.Role,
		Description: info.Description,
		Categories:  append([]string(nil), info.Categories...),
		Tags:        append([]string(nil), info.Tags...),
		IconKey:     info.IconKey,
		Tone:        info.Tone,
		AvatarKey:   info.AvatarKey,
		Members:     members,
		Prompts:     append([]string(nil), info.Prompts...),
	}, nil
}

func expertListFilter(input ExpertListFilter) (expert.ListFilter, error) {
	mode := strings.TrimSpace(input.Mode)
	filter := expert.ListFilter{
		Category: strings.TrimSpace(input.Category),
		Query:    strings.TrimSpace(input.Query),
	}
	if mode == "" || strings.EqualFold(mode, "all") {
		return filter, nil
	}
	parsed, err := expert.ParseMode(mode)
	if err != nil {
		return expert.ListFilter{}, err
	}
	filter.Mode = parsed
	return filter, nil
}

func filterExpertInfos(items []ExpertInfo, filter expert.ListFilter) []ExpertInfo {
	category := strings.TrimSpace(filter.Category)
	if category == "全部" || strings.EqualFold(category, "all") {
		category = ""
	}
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	out := make([]ExpertInfo, 0, len(items))
	for _, item := range items {
		if filter.Mode != "" && item.Mode != string(filter.Mode) {
			continue
		}
		if category != "" {
			matched := false
			for _, value := range item.Categories {
				if strings.EqualFold(strings.TrimSpace(value), category) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if query != "" {
			fields := []string{item.Name, item.Role, item.Description}
			fields = append(fields, item.Categories...)
			fields = append(fields, item.Tags...)
			fields = append(fields, item.Prompts...)
			for _, member := range item.Members {
				fields = append(fields, member.Name, member.Role, member.Task, member.Output)
			}
			matched := false
			for _, value := range fields {
				if strings.Contains(strings.ToLower(value), query) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		out = append(out, item)
	}
	return out
}

func expertInfos(defs []expert.Definition) []ExpertInfo {
	out := make([]ExpertInfo, 0, len(defs))
	for _, def := range defs {
		members := make([]ExpertMemberInfo, 0, len(def.Members))
		for _, member := range def.Members {
			members = append(members, ExpertMemberInfo{
				Kind:      member.Kind,
				ExpertID:  member.ExpertID,
				Name:      member.Name,
				Role:      member.Role,
				Task:      member.Task,
				Output:    member.Output,
				Tone:      member.Tone,
				AvatarKey: member.AvatarKey,
			})
		}
		category := def.Role
		if len(def.Categories) > 0 {
			category = def.Categories[0]
		}
		out = append(out, ExpertInfo{
			ID:          def.ID,
			Source:      def.Source,
			Mode:        string(def.Mode),
			Name:        def.Name,
			Role:        def.Role,
			Description: def.Description,
			Category:    category,
			Categories:  append([]string(nil), def.Categories...),
			Tags:        append([]string(nil), def.Tags...),
			IconKey:     def.IconKey,
			Tone:        def.Tone,
			AvatarKey:   def.AvatarKey,
			Members:     members,
			Prompts:     append([]string(nil), def.Prompts...),
			Language:    "auto",
			Enabled:     true,
			CreatedAt:   def.CreatedAt,
			UpdatedAt:   def.UpdatedAt,
		})
	}
	return out
}

func userExpertInfos(records []expert.UserExpert) []expert.UserExpertInfo {
	out := make([]expert.UserExpertInfo, 0, len(records))
	for _, record := range records {
		out = append(out, record.Info())
	}
	return out
}
