package expert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"kwu/internal/fileutil"
)

const userStoreVersion = 2

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

type UserExpert struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Mode          string   `json:"mode,omitempty"`
	Description   string   `json:"description"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	AvatarKey     string   `json:"avatarKey,omitempty"`
	Members       []Member `json:"members,omitempty"`
	SystemPrompt  string   `json:"systemPrompt"`
	DisplayPrompt string   `json:"displayPrompt,omitempty"`
	Prompts       []string `json:"prompts"`
	Language      string   `json:"language,omitempty"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     int64    `json:"createdAt"`
	UpdatedAt     int64    `json:"updatedAt"`
	enabledSet    bool
}

type UserExpertInput struct {
	Name           string   `json:"name"`
	Mode           string   `json:"mode,omitempty"`
	Description    string   `json:"description"`
	Category       string   `json:"category,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	AvatarKey      string   `json:"avatarKey,omitempty"`
	AvatarFileName string   `json:"avatarFileName,omitempty"`
	Members        []Member `json:"members,omitempty"`
	SystemPrompt   string   `json:"systemPrompt"`
	DisplayPrompt  string   `json:"displayPrompt,omitempty"`
	Prompts        []string `json:"prompts"`
	Language       string   `json:"language,omitempty"`
	Enabled        bool     `json:"enabled"`
	enabledSet     bool
}

type UserExpertInfo struct {
	ID            string   `json:"id"`
	Source        string   `json:"source"`
	Name          string   `json:"name"`
	Mode          string   `json:"mode"`
	Description   string   `json:"description"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	AvatarKey     string   `json:"avatarKey,omitempty"`
	Members       []Member `json:"members,omitempty"`
	SystemPrompt  string   `json:"systemPrompt"`
	DisplayPrompt string   `json:"displayPrompt,omitempty"`
	Prompts       []string `json:"prompts"`
	Language      string   `json:"language,omitempty"`
	Enabled       bool     `json:"enabled"`
	CreatedAt     int64    `json:"createdAt"`
	UpdatedAt     int64    `json:"updatedAt"`
}

type ExpertLibraryState struct {
	FavoriteIDs []string         `json:"favoriteIds"`
	RecentIDs   []string         `json:"recentIds"`
	UsageCounts map[string]int64 `json:"usageCounts,omitempty"`
}

type userStoreFile struct {
	Version      int                `json:"version"`
	Experts      []UserExpert       `json:"experts"`
	LibraryState ExpertLibraryState `json:"libraryState,omitempty"`
}

func (e *UserExpert) UnmarshalJSON(data []byte) error {
	type userExpertJSON UserExpert
	var decoded userExpertJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = UserExpert(decoded)
	e.enabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

func (input *UserExpertInput) UnmarshalJSON(data []byte) error {
	type userExpertInputJSON UserExpertInput
	var decoded userExpertInputJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*input = UserExpertInput(decoded)
	input.enabledSet = jsonFieldPresent(data, "enabled")
	return nil
}

func LoadCatalog(userPath string) (*Catalog, error) {
	builtins, err := Builtin()
	if err != nil {
		return nil, err
	}
	users, err := LoadUserExperts(userPath)
	if err != nil {
		return nil, err
	}
	experts := make([]Expert, 0, len(builtins)+len(users))
	experts = append(experts, builtins...)
	for _, user := range users {
		experts = append(experts, user.Expert())
	}
	return NewCatalog(experts)
}

func LoadUserExperts(path string) ([]UserExpert, error) {
	file, err := loadUserStoreFile(path)
	if err != nil {
		return nil, err
	}
	return file.Experts, nil
}

func LoadExpertLibraryState(path string) (ExpertLibraryState, error) {
	file, err := loadUserStoreFile(path)
	if err != nil {
		return ExpertLibraryState{}, err
	}
	return file.LibraryState, nil
}

func SaveExpertLibraryState(path string, state ExpertLibraryState) error {
	file, err := loadUserStoreFile(path)
	if err != nil {
		return err
	}
	file.LibraryState = normalizeLibraryState(state)
	return saveUserStoreFile(path, file)
}

func ResetUserExpertStore(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("user expert path is empty")
	}
	return saveUserStoreFile(path, userStoreFile{
		Version:      userStoreVersion,
		Experts:      []UserExpert{},
		LibraryState: ExpertLibraryState{},
	})
}

func SetExpertFavorite(path, id string, favorite bool) (ExpertLibraryState, error) {
	state, err := LoadExpertLibraryState(path)
	if err != nil {
		return ExpertLibraryState{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return state, fmt.Errorf("expert id is required")
	}
	state.FavoriteIDs = removeID(state.FavoriteIDs, id)
	if favorite {
		state.FavoriteIDs = append([]string{id}, state.FavoriteIDs...)
	}
	state = normalizeLibraryState(state)
	if err := SaveExpertLibraryState(path, state); err != nil {
		return ExpertLibraryState{}, err
	}
	return state, nil
}

func MarkExpertUsed(path, id string) (ExpertLibraryState, error) {
	state, err := LoadExpertLibraryState(path)
	if err != nil {
		return ExpertLibraryState{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return state, nil
	}
	if state.UsageCounts == nil {
		state.UsageCounts = make(map[string]int64)
	}
	if state.UsageCounts[id] < math.MaxInt64 {
		state.UsageCounts[id]++
	}
	state.RecentIDs = append([]string{id}, removeID(state.RecentIDs, id)...)
	state = normalizeLibraryState(state)
	if err := SaveExpertLibraryState(path, state); err != nil {
		return ExpertLibraryState{}, err
	}
	return state, nil
}

func RemoveExpertLibraryID(path, id string) error {
	file, err := loadUserStoreFile(path)
	if err != nil {
		return err
	}
	file.LibraryState.FavoriteIDs = removeID(file.LibraryState.FavoriteIDs, id)
	file.LibraryState.RecentIDs = removeID(file.LibraryState.RecentIDs, id)
	delete(file.LibraryState.UsageCounts, strings.TrimSpace(id))
	return saveUserStoreFile(path, file)
}

func loadUserStoreFile(path string) (userStoreFile, error) {
	file, storedVersion, err := readUserStoreFile(path)
	if err != nil {
		return file, err
	}
	file.Experts, err = normalizeLoadedUserExperts(file.Experts, storedVersion)
	if err != nil {
		return file, err
	}
	file.Version = userStoreVersion
	file.LibraryState = normalizeLibraryState(file.LibraryState)
	if err := saveMigratedUserStore(path, file, storedVersion); err != nil {
		return file, err
	}
	return file, nil
}

func readUserStoreFile(path string) (userStoreFile, int, error) {
	file := userStoreFile{Version: userStoreVersion}
	if strings.TrimSpace(path) == "" {
		return file, file.Version, nil
	}
	body, err := readUserStoreBytes(path)
	if errors.Is(err, os.ErrNotExist) {
		return file, file.Version, nil
	}
	if err != nil {
		return file, file.Version, err
	}
	if err := json.Unmarshal(body, &file); err != nil {
		return file, file.Version, fmt.Errorf("parse user experts: %w", err)
	}
	return file, file.Version, nil
}

func normalizeLoadedUserExperts(records []UserExpert, storedVersion int) ([]UserExpert, error) {
	out := make([]UserExpert, 0, len(records))
	seen := map[string]bool{}
	for _, ex := range records {
		ex = migrateLegacyUserExpert(ex, storedVersion)
		ex = normalizeUserExpert(ex)
		if err := ex.Validate(); err != nil {
			return nil, err
		}
		if seen[ex.ID] {
			return nil, fmt.Errorf("duplicate user expert id %q", ex.ID)
		}
		seen[ex.ID] = true
		out = append(out, ex)
	}
	return out, nil
}

func migrateLegacyUserExpert(ex UserExpert, storedVersion int) UserExpert {
	if storedVersion >= userStoreVersion {
		return ex
	}
	ex.Prompts = migrateLegacyUserPrompts(ex)
	if strings.TrimSpace(ex.Mode) == string(ModeTeam) {
		ex.Members = completeLegacyTeamMembers(ex.Name, ex.Members)
	}
	return ex
}

func migrateLegacyUserPrompts(ex UserExpert) []string {
	prompts := trimStrings(ex.Prompts)
	switch {
	case len(prompts) > 5:
		return append([]string(nil), prompts[:5]...)
	case len(prompts) >= 3:
		return prompts
	}
	if len(prompts) == 0 {
		prompts = []string{ex.DisplayPrompt}
	}
	return completeLegacyPrompts(ex.Mode, ex.Name, prompts)
}

func saveMigratedUserStore(path string, file userStoreFile, storedVersion int) error {
	if storedVersion >= userStoreVersion {
		return nil
	}
	if err := saveUserStoreFile(path, file); err != nil {
		return fmt.Errorf("migrate user experts: %w", err)
	}
	return nil
}

func SaveUserExperts(path string, experts []UserExpert) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("user expert path is empty")
	}
	file, err := loadUserStoreFile(path)
	if err != nil {
		return err
	}
	file.Version = userStoreVersion
	file.Experts = make([]UserExpert, 0, len(experts))
	seen := map[string]bool{}
	for _, ex := range experts {
		ex = normalizeUserExpert(ex)
		if err := ex.Validate(); err != nil {
			return err
		}
		if seen[ex.ID] {
			return fmt.Errorf("duplicate user expert id %q", ex.ID)
		}
		seen[ex.ID] = true
		file.Experts = append(file.Experts, ex)
	}
	return saveUserStoreFile(path, file)
}

func saveUserStoreFile(path string, file userStoreFile) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("user expert path is empty")
	}
	file.Version = userStoreVersion
	file.LibraryState = normalizeLibraryState(file.LibraryState)
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return fileutil.AtomicWriteFileStrict(path, body, 0o600)
}

func NewUserExpert(id string, input UserExpertInput, now time.Time) (UserExpert, error) {
	ex := UserExpert{
		ID:            strings.TrimSpace(id),
		Name:          strings.TrimSpace(input.Name),
		Mode:          strings.TrimSpace(input.Mode),
		Description:   strings.TrimSpace(input.Description),
		Category:      strings.TrimSpace(input.Category),
		Tags:          cleanTags(input.Tags),
		AvatarKey:     strings.TrimSpace(input.AvatarKey),
		Members:       append([]Member(nil), input.Members...),
		SystemPrompt:  strings.TrimSpace(input.SystemPrompt),
		DisplayPrompt: strings.TrimSpace(input.DisplayPrompt),
		Prompts:       append([]string(nil), input.Prompts...),
		Language:      strings.TrimSpace(input.Language),
		Enabled:       enabledOrDefault(input.Enabled, input.enabledSet),
		CreatedAt:     now.UnixMilli(),
		UpdatedAt:     now.UnixMilli(),
		enabledSet:    true,
	}
	ex = normalizeUserExpert(ex)
	return ex, ex.Validate()
}

func UpdateUserExpert(existing UserExpert, input UserExpertInput, now time.Time) (UserExpert, error) {
	next, err := NewUserExpert(existing.ID, input, now)
	if err != nil {
		return UserExpert{}, err
	}
	next.CreatedAt = existing.CreatedAt
	next.UpdatedAt = now.UnixMilli()
	return next, nil
}

func ImportUserExpertConfig(payload string) (UserExpertInput, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return UserExpertInput{}, fmt.Errorf("import payload is required")
	}
	type userExpertInputWire UserExpertInput
	var wire userExpertInputWire
	dec := json.NewDecoder(bytes.NewBufferString(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return UserExpertInput{}, fmt.Errorf("parse expert import config: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return UserExpertInput{}, fmt.Errorf("parse expert import config: multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return UserExpertInput{}, fmt.Errorf("parse expert import config: %w", err)
	}
	input := UserExpertInput(wire)
	input.enabledSet = jsonFieldPresent([]byte(payload), "enabled")
	input.Name = strings.TrimSpace(input.Name)
	input.Mode = strings.TrimSpace(input.Mode)
	input.Description = strings.TrimSpace(input.Description)
	input.Category = strings.TrimSpace(input.Category)
	input.Tags = cleanTags(input.Tags)
	input.AvatarKey = strings.TrimSpace(input.AvatarKey)
	input.Members = normalizeUserExpertMembers(input.Mode, input.Members)
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	input.DisplayPrompt = strings.TrimSpace(input.DisplayPrompt)
	input.Prompts = trimStrings(input.Prompts)
	input.Language = strings.TrimSpace(input.Language)
	if input.Language == "" {
		input.Language = "auto"
	}
	input.Enabled = enabledOrDefault(input.Enabled, input.enabledSet)
	input.enabledSet = true
	preview, err := NewUserExpert("import-preview", input, time.Now())
	if err != nil {
		return UserExpertInput{}, err
	}
	return UserExpertInput{
		Name:          preview.Name,
		Mode:          preview.Mode,
		Description:   preview.Description,
		Category:      preview.Category,
		Tags:          append([]string(nil), preview.Tags...),
		AvatarKey:     preview.AvatarKey,
		Members:       append([]Member(nil), preview.Members...),
		SystemPrompt:  preview.SystemPrompt,
		DisplayPrompt: preview.DisplayPrompt,
		Prompts:       append([]string(nil), preview.Prompts...),
		Language:      preview.Language,
		Enabled:       preview.Enabled,
		enabledSet:    true,
	}, nil
}

func (e UserExpert) Validate() error {
	if IsBuiltinID(strings.TrimSpace(e.ID)) {
		return fmt.Errorf("official expert %q is read-only", strings.TrimSpace(e.ID))
	}
	if err := e.Expert().Validate(); err != nil {
		return err
	}
	if _, err := normalizeAndValidateMembers(e.ID, Mode(e.Mode), e.Members); err != nil {
		return err
	}
	return validatePrompts(e.ID, e.Prompts)
}

func (e UserExpert) Expert() Expert {
	e = normalizeUserExpert(e)
	return Expert{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Category:    e.Category,
		Prompt:      e.SystemPrompt,
		Language:    e.Language,
		Enabled:     e.Enabled,
		Source:      SourceUser,
		Mode:        Mode(e.Mode),
		Members:     append([]Member(nil), e.Members...),
		Prompts:     append([]string(nil), e.Prompts...),
	}
}

func (e UserExpert) Info() UserExpertInfo {
	e = normalizeUserExpert(e)
	return UserExpertInfo{
		ID:            e.ID,
		Source:        SourceUser,
		Name:          e.Name,
		Mode:          e.Mode,
		Description:   e.Description,
		Category:      e.Category,
		Tags:          append([]string(nil), e.Tags...),
		AvatarKey:     e.AvatarKey,
		Members:       append([]Member(nil), e.Members...),
		SystemPrompt:  e.SystemPrompt,
		DisplayPrompt: e.DisplayPrompt,
		Prompts:       append([]string(nil), e.Prompts...),
		Language:      e.Language,
		Enabled:       e.Enabled,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
}

func UniqueUserExpertID(name string, exists func(string) bool, now time.Time) string {
	base := "custom-" + slugify(name)
	if base == "custom-" {
		base = "custom-expert"
	}
	if exists == nil || !exists(base) {
		return base
	}
	stamp := now.UnixMilli()
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("%s-%d", base, stamp+int64(i))
		if !exists(id) {
			return id
		}
	}
	return fmt.Sprintf("%s-%d", base, time.Now().UnixNano())
}

func normalizeUserExpert(e UserExpert) UserExpert {
	e.ID = strings.TrimSpace(e.ID)
	e.Name = strings.TrimSpace(e.Name)
	e.Mode = strings.TrimSpace(e.Mode)
	if e.Mode != "team" {
		e.Mode = "expert"
	}
	e.Description = strings.TrimSpace(e.Description)
	if e.Description == "" {
		e.Description = "User-defined expert."
	}
	e.Category = strings.TrimSpace(e.Category)
	if e.Category == "" {
		if e.Mode == "team" {
			e.Category = "team"
		} else {
			e.Category = "custom"
		}
	}
	e.Tags = cleanTags(e.Tags)
	e.AvatarKey = strings.TrimSpace(e.AvatarKey)
	e.Members = normalizeUserExpertMembers(e.Mode, e.Members)
	e.SystemPrompt = strings.TrimSpace(e.SystemPrompt)
	e.DisplayPrompt = strings.TrimSpace(e.DisplayPrompt)
	e.Prompts = trimStrings(e.Prompts)
	if len(e.Prompts) == 0 {
		e.Prompts = completeLegacyPrompts(e.Mode, e.Name, []string{e.DisplayPrompt})
	}
	if len(e.Prompts) > 0 {
		e.DisplayPrompt = e.Prompts[0]
	}
	e.Language = strings.TrimSpace(e.Language)
	if e.Language == "" {
		e.Language = "auto"
	}
	e.Enabled = enabledOrDefault(e.Enabled, e.enabledSet)
	e.enabledSet = true
	now := time.Now().UnixMilli()
	if e.CreatedAt == 0 {
		e.CreatedAt = now
	}
	if e.UpdatedAt == 0 {
		e.UpdatedAt = e.CreatedAt
	}
	return e
}

func migratedUserPrompts(mode, name, first string) []string {
	first = strings.TrimSpace(first)
	if first == "" {
		first = fmt.Sprintf("请以%s的身份处理当前任务。", strings.TrimSpace(name))
	}
	if mode == "team" {
		return []string{
			first,
			"请先拆解当前任务，并说明专家团成员的分工和交付物。",
			"请协调专家团分析当前问题，汇总风险、方案和下一步行动。",
		}
	}
	return []string{
		first,
		"请分析当前问题的关键原因，并给出可执行的解决建议。",
		"请检查当前方案的主要风险，并给出优先级明确的改进清单。",
	}
}

func completeLegacyPrompts(mode, name string, prompts []string) []string {
	out := trimStrings(prompts)
	first := ""
	if len(out) > 0 {
		first = out[0]
	}
	for _, prompt := range migratedUserPrompts(mode, name, first) {
		if len(out) >= 3 {
			break
		}
		if !containsPrompt(out, prompt) {
			out = append(out, prompt)
		}
	}
	for index := 1; len(out) < 3; index++ {
		prompt := fmt.Sprintf("请围绕%s补充第%d个关键分析问题。", strings.TrimSpace(name), index)
		if !containsPrompt(out, prompt) {
			out = append(out, prompt)
		}
	}
	return out
}

func containsPrompt(prompts []string, target string) bool {
	for _, prompt := range prompts {
		if prompt == target {
			return true
		}
	}
	return false
}

func enabledOrDefault(enabled bool, explicitlySet bool) bool {
	if explicitlySet || enabled {
		return enabled
	}
	return true
}

func jsonFieldPresent(data []byte, field string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return false
	}
	_, ok := raw[field]
	return ok
}

func normalizeUserExpertMembers(mode string, members []Member) []Member {
	return normalizeMembers(Mode(mode), members)
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
		if len(out) == 8 {
			break
		}
	}
	return out
}

func normalizeLibraryState(state ExpertLibraryState) ExpertLibraryState {
	return ExpertLibraryState{
		FavoriteIDs: cleanIDs(state.FavoriteIDs, 200),
		RecentIDs:   cleanIDs(state.RecentIDs, 20),
		UsageCounts: cleanUsageCounts(state.UsageCounts),
	}
}

func cleanUsageCounts(counts map[string]int64) map[string]int64 {
	if len(counts) == 0 {
		return nil
	}
	out := make(map[string]int64, len(counts))
	for id, count := range counts {
		id = strings.TrimSpace(id)
		if id == "" || count <= 0 {
			continue
		}
		if out[id] > math.MaxInt64-count {
			out[id] = math.MaxInt64
		} else {
			out[id] += count
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cleanIDs(ids []string, limit int) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func removeID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	out := ids[:0]
	for _, current := range ids {
		if strings.TrimSpace(current) == id {
			continue
		}
		out = append(out, current)
	}
	return out
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "expert"
	}
	return s
}
