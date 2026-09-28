package main

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/expert"
)

func TestDecodeCloudExpertAvatarAcceptsLocalImageData(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	fileName, contentType, decoded, err := decodeCloudExpertAvatar(
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(png),
		"portrait.png",
	)
	if err != nil {
		t.Fatalf("decodeCloudExpertAvatar: %v", err)
	}
	if fileName != "portrait.png" || contentType != "image/png" || string(decoded) != string(png) {
		t.Fatalf("decoded avatar = %q %q %v", fileName, contentType, decoded)
	}
	fileName, contentType, decoded, err = decodeCloudExpertAvatar("", "")
	if err != nil {
		t.Fatalf("decodeCloudExpertAvatar(default): %v", err)
	}
	if fileName != "default-expert.png" || contentType != "image/png" ||
		http.DetectContentType(decoded) != "image/png" {
		t.Fatalf("default avatar = %q %q %q", fileName, contentType, http.DetectContentType(decoded))
	}
	if _, _, _, err := decodeCloudExpertAvatar("https://assets.example.com/avatar.png", "avatar.png"); err == nil {
		t.Fatal("remote avatar URL was accepted")
	}
}

func TestCloudExpertCatalogMapsExpertsAndGroupsForRuntime(t *testing.T) {
	createdAt := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	definitions, infos := cloudExpertCatalog(cloud.SystemExpertAssets{
		Experts: []cloud.SystemExpertAsset{
			{
				ExpertID: "exp_architect", Name: "架构设计专家", Prompt: "分析系统边界。",
				Description: "聚焦系统边界、数据流和接口契约。",
				Category:    "代码交付",
				Tags:        []string{"advanced-development", "architecture", "code-quality"},
				AvatarURL:   "https://assets.example.com/architect.png", Type: "system",
				CreatedAt: createdAt, UpdatedAt: updatedAt,
			},
			{
				ExpertID: "exp_testing", Name: "测试专家", Prompt: "验证关键路径。",
				Description: "设计测试覆盖并验证关键交付路径。",
				Category:    "代码交付",
				Tags:        []string{"testing", "code-quality"},
				AvatarURL:   "https://assets.example.com/testing.png", Type: "system",
			},
		},
		ExpertGroups: []cloud.SystemExpertGroupAsset{
			{
				ExpertGroupID: "exg_engineering", Name: "研发评审专家团",
				Description: "覆盖架构、测试与发布评审。",
				Category:    "代码交付",
				ExpertIDs:   []string{"exp_testing", "exp_architect"},
				Tags:        []string{"advanced-development", "architecture", "code-quality"},
				AvatarURL:   "https://assets.example.com/engineering.png", Type: "system",
			},
		},
	})
	if len(definitions) != 3 || len(infos) != 3 {
		t.Fatalf("catalog sizes = %d definitions / %d infos", len(definitions), len(infos))
	}
	if definitions[0].ID != "exp_architect" ||
		definitions[0].Source != expert.SourceOfficial ||
		definitions[0].Description != "聚焦系统边界、数据流和接口契约。" ||
		definitions[0].SystemPrompt != "分析系统边界。" {
		t.Fatalf("expert definition = %+v", definitions[0])
	}
	if definitions[0].Role != "代码交付" ||
		len(definitions[0].Categories) != 1 ||
		definitions[0].Categories[0] != "代码交付" {
		t.Fatalf("expert classification = role %q, categories %v", definitions[0].Role, definitions[0].Categories)
	}
	if definitions[0].AvatarKey != "https://assets.example.com/architect.png" {
		t.Fatalf("runtime expert avatar = %q, want service avatar URL", definitions[0].AvatarKey)
	}
	if len(definitions[0].Tags) != 3 || definitions[0].Tags[0] != "advanced-development" || definitions[0].Tags[2] != "code-quality" {
		t.Fatalf("expert tags = %v", definitions[0].Tags)
	}
	team := definitions[2]
	if team.ID != "exg_engineering" || team.Mode != expert.ModeTeam || len(team.Members) != 2 {
		t.Fatalf("team definition = %+v", team)
	}
	if team.Description != "覆盖架构、测试与发布评审。" {
		t.Fatalf("team description = %q", team.Description)
	}
	if team.Role != "代码交付" || len(team.Categories) != 1 || team.Categories[0] != "代码交付" {
		t.Fatalf("team classification = role %q, categories %v", team.Role, team.Categories)
	}
	if len(team.Tags) != 3 || team.Tags[0] != "advanced-development" || team.Tags[2] != "code-quality" {
		t.Fatalf("team tags = %v", team.Tags)
	}
	if team.Members[0].Name != "测试专家" || team.Members[1].Name != "架构设计专家" {
		t.Fatalf("team member order = %+v", team.Members)
	}
	if team.AvatarKey != "https://assets.example.com/engineering.png" ||
		team.Members[0].AvatarKey != "https://assets.example.com/testing.png" ||
		team.Members[1].AvatarKey != "https://assets.example.com/architect.png" {
		t.Fatalf("runtime team avatars = team %q members %+v, want service avatar URLs", team.AvatarKey, team.Members)
	}
	if infos[0].AvatarURL != "https://assets.example.com/architect.png" ||
		infos[0].Description != "聚焦系统边界、数据流和接口契约。" ||
		infos[2].AvatarURL != "https://assets.example.com/engineering.png" ||
		infos[2].Description != "覆盖架构、测试与发布评审。" ||
		infos[2].Members[0].AvatarURL != "https://assets.example.com/testing.png" ||
		len(infos[2].Tags) != 3 || infos[2].Tags[1] != "architecture" {
		t.Fatalf("mapped expert infos = %+v", infos)
	}
	if definitions[0].CreatedAt != createdAt.UnixMilli() || definitions[0].UpdatedAt != updatedAt.UnixMilli() ||
		infos[0].CreatedAt != createdAt.UnixMilli() || infos[0].UpdatedAt != updatedAt.UnixMilli() {
		t.Fatalf("mapped expert timestamps = definition %+v info %+v", definitions[0], infos[0])
	}
}

func TestCloudUserExpertRecordsMapsExpertsAndGroups(t *testing.T) {
	owner := "usr_01"
	createdAt := time.Date(2026, 7, 25, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	records, err := cloudUserExpertRecords(
		[]cloud.UserExpertAsset{{
			ExpertID: "exp_user", Name: "Researcher", Description: "Research evidence.", Prompt: "Find evidence.",
			Category: "数据分析", Tags: []string{"调研", "洞察", "报告"},
			AvatarURL: "https://assets.example.com/user.png", Type: "user", OwnerUserID: &owner,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}},
		[]cloud.UserExpertGroupAsset{{
			ExpertGroupID: "exg_user", Name: "Review Team",
			Description: "Review findings.", Category: "数据分析", Tags: []string{"评审", "协作", "交付"},
			ExpertIDs: []string{"exp_system", "exp_user"},
			AvatarURL: "https://assets.example.com/team.png", Type: "user", OwnerUserID: &owner,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}},
	)
	if err != nil {
		t.Fatalf("cloudUserExpertRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %+v", records)
	}
	if records[0].ID != "exp_user" || records[0].Mode != string(expert.ModeExpert) ||
		records[0].Category != "数据分析" || records[0].Description != "Research evidence." ||
		records[0].SystemPrompt != "Find evidence." ||
		records[0].UpdatedAt != updatedAt.UnixMilli() {
		t.Fatalf("user expert = %+v", records[0])
	}
	group := records[1]
	if group.ID != "exg_user" || group.Mode != string(expert.ModeTeam) ||
		group.Category != "数据分析" || len(group.Members) != 2 {
		t.Fatalf("user group = %+v", group)
	}
	if group.Members[0].ExpertID != "exp_system" || group.Members[0].Name != "exp_system" ||
		group.Members[1].ExpertID != "exp_user" || group.Members[1].Name != "Researcher" {
		t.Fatalf("group members = %+v", group.Members)
	}
	if group.AvatarKey != "https://assets.example.com/team.png" || group.Description != "Review findings." || group.SystemPrompt == "" {
		t.Fatalf("group presentation = %+v", group)
	}
	if got := records[0].Tags; len(got) != 3 || got[0] != "调研" || got[2] != "报告" {
		t.Fatalf("expert tags = %v", got)
	}
	if got := group.Tags; len(got) != 3 || got[0] != "评审" || got[2] != "交付" {
		t.Fatalf("group tags = %v", got)
	}
}
