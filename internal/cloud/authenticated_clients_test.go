package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type expertAssetsTestExecutor struct {
	token string
	calls *int
}

func (e expertAssetsTestExecutor) Do(ctx context.Context, scope BindingScope, request func(context.Context, string) error) error {
	if e.calls != nil {
		(*e.calls)++
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	return request(ctx, e.token)
}

func TestHTTPExpertAssetsClientListsSystemAssetsWithoutQueryParameters(t *testing.T) {
	var requests int
	var authenticatedRequests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.URL.Path != systemExpertAssetsPath {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.URL.RawQuery != "" {
			t.Fatalf("query = %q, want empty", request.URL.RawQuery)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Fatalf("Authorization = %q, want absent", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data": {
				"experts": [
					{
						"expert_id": "exp_testing",
						"avatar_url": "",
						"name": "Testing",
						"description": " Verify critical paths and release quality. ",
						"prompt": "Test the change.",
						"category": " Code Delivery ",
						"tags": ["Testing", "Quality"],
						"type": "system",
						"owner_user_id": null,
						"created_at": "2026-07-25T00:00:00Z",
						"updated_at": "2026-07-25T00:00:00Z"
					},
					{
						"expert_id": "exp_architect",
						"avatar_url": "https://assets.example.com/architect.png",
						"name": "Architect",
						"description": " Design clear and evolvable system boundaries. ",
						"prompt": "Design the system.",
						"category": "Code Delivery",
						"tags": ["Architecture", "Quality"],
						"type": "system",
						"owner_user_id": null,
						"created_at": "2026-07-25T00:00:00Z",
						"updated_at": "2026-07-25T00:00:00Z"
					}
				],
				"expert_groups": [
					{
						"expert_group_id": "exg_engineering",
						"avatar_url": "https://assets.example.com/team.png",
						"name": "Engineering",
						"description": " Coordinate architecture, testing, and release review. ",
						"category": " Code Delivery ",
						"tags": ["Advanced Development", "Architecture", "Quality"],
						"expert_ids": ["exp_testing", "exp_architect"],
						"type": "system",
						"owner_user_id": null,
						"created_at": "2026-07-25T00:00:00Z",
						"updated_at": "2026-07-25T00:00:00Z"
					}
				]
			},
			"error": null,
			"request_id": "req-experts",
			"server_time": "2026-07-25T00:00:01Z"
		}`))
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-01", ClientVersion: "dev",
	}, func() string { return "req-client" })
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	client, err := NewHTTPExpertAssetsClient(expertAssetsTestExecutor{token: "access-current", calls: &authenticatedRequests}, transport)
	if err != nil {
		t.Fatalf("NewHTTPExpertAssetsClient: %v", err)
	}
	assets, err := client.ListSystem(context.Background())
	if err != nil {
		t.Fatalf("ListSystem: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	if authenticatedRequests != 0 {
		t.Fatalf("authenticated executor calls = %d, want 0", authenticatedRequests)
	}
	if len(assets.Experts) != 2 || assets.Experts[0].ExpertID != "exp_architect" || assets.Experts[1].ExpertID != "exp_testing" {
		t.Fatalf("experts = %+v", assets.Experts)
	}
	if assets.Experts[0].Category != "Code Delivery" || assets.Experts[1].Category != "Code Delivery" {
		t.Fatalf("expert categories = %+v", assets.Experts)
	}
	if assets.Experts[0].Description != "Design clear and evolvable system boundaries." ||
		assets.Experts[1].Description != "Verify critical paths and release quality." {
		t.Fatalf("expert descriptions = %+v", assets.Experts)
	}
	if got := assets.Experts[1].Tags; len(got) != 2 || got[0] != "Testing" || got[1] != "Quality" {
		t.Fatalf("expert tags = %#v", got)
	}
	if len(assets.ExpertGroups) != 1 ||
		assets.ExpertGroups[0].Category != "Code Delivery" ||
		assets.ExpertGroups[0].Description != "Coordinate architecture, testing, and release review." ||
		len(assets.ExpertGroups[0].Tags) != 3 ||
		assets.ExpertGroups[0].Tags[0] != "Advanced Development" ||
		assets.ExpertGroups[0].ExpertIDs[0] != "exp_testing" ||
		assets.ExpertGroups[0].ExpertIDs[1] != "exp_architect" {
		t.Fatalf("expert groups = %+v", assets.ExpertGroups)
	}
}

func TestNormalizeSystemExpertAssetsRejectsPrivateAndInsecureAssets(t *testing.T) {
	owner := "usr_other"
	for _, test := range []struct {
		name   string
		assets SystemExpertAssets
	}{
		{
			name: "private expert",
			assets: SystemExpertAssets{Experts: []SystemExpertAsset{{
				ExpertID: "exp_private", Name: "Private", Prompt: "No", Type: "user", OwnerUserID: &owner,
			}}},
		},
		{
			name: "insecure avatar",
			assets: SystemExpertAssets{Experts: []SystemExpertAsset{{
				ExpertID: "exp_system", Name: "System", Prompt: "Yes", Type: "system", AvatarURL: "http://assets.example.com/avatar.png",
			}}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizeSystemExpertAssets(&test.assets); err == nil {
				t.Fatal("normalizeSystemExpertAssets returned nil error")
			}
		})
	}
}

func TestHTTPExpertAssetsClientListsCurrentUserAssetsAcrossCursorPages(t *testing.T) {
	const owner = "usr_01"
	const nextCursor = "next/+?="
	requestsByPath := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestsByPath[request.URL.Path]++
		if request.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", request.Method)
		}
		if request.Header.Get("Authorization") != "Bearer access-current" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.ContentLength != 0 || request.Header.Get("Content-Type") != "" {
			t.Fatalf("request has body metadata: length=%d content-type=%q", request.ContentLength, request.Header.Get("Content-Type"))
		}
		query := request.URL.Query()
		if query.Get("limit") != "20" || query.Has("type") || query.Has("owner_user_id") {
			t.Fatalf("query = %v", query)
		}
		cursor, hasCursor := query["cursor"]
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == userExpertsPath && !hasCursor:
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"expert_id":"exp_01","avatar_url":"","name":"Research","prompt":"Find evidence.",
				"description":"Research evidence.","category":"数据分析","tags":["调研","洞察","报告"],
				"type":"user","owner_user_id":"` + owner + `",
				"created_at":"2026-07-25T08:00:00Z","updated_at":"2026-07-25T08:00:00Z"
			}],"next_cursor":"` + nextCursor + `","has_more":true},"error":null}`))
		case request.URL.Path == userExpertsPath && len(cursor) == 1 && cursor[0] == nextCursor:
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"expert_id":"exp_02","avatar_url":"","name":"Writer","prompt":"Draft the result.",
				"description":"Write the result.","category":"代码交付","tags":["写作","总结","交付"],
				"type":"user","owner_user_id":"` + owner + `",
				"created_at":"2026-07-25T09:00:00Z","updated_at":"2026-07-25T09:00:00Z"
			}],"next_cursor":null,"has_more":false},"error":null}`))
		case request.URL.Path == userExpertGroupsPath && !hasCursor:
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"expert_group_id":"exg_01","avatar_url":"","name":"Research Team",
				"description":"Review research.","category":"数据分析","tags":["评审","协作","报告"],
				"expert_ids":["exp_01","exp_02"],"type":"user","owner_user_id":"` + owner + `",
				"created_at":"2026-07-25T10:00:00Z","updated_at":"2026-07-25T10:00:00Z"
			}],"next_cursor":null,"has_more":false},"error":null}`))
		default:
			t.Fatalf("unexpected request %s?%s", request.URL.Path, request.URL.RawQuery)
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-01", ClientVersion: "dev",
	}, nil)
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	client, err := NewHTTPExpertAssetsClient(expertAssetsTestExecutor{token: "access-current"}, transport)
	if err != nil {
		t.Fatalf("NewHTTPExpertAssetsClient: %v", err)
	}
	scope := BindingScope{ProfileID: "default", CloudUserID: owner, BindingGeneration: 1}
	experts, err := client.ListUserExperts(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListUserExperts: %v", err)
	}
	groups, err := client.ListUserExpertGroups(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListUserExpertGroups: %v", err)
	}
	if len(experts) != 2 || experts[0].ExpertID != "exp_01" || experts[1].ExpertID != "exp_02" ||
		experts[0].Description != "Research evidence." || len(experts[0].Tags) != 3 || experts[0].Tags[1] != "洞察" {
		t.Fatalf("experts = %+v", experts)
	}
	if len(groups) != 1 || groups[0].ExpertGroupID != "exg_01" || len(groups[0].ExpertIDs) != 2 ||
		groups[0].Description != "Review research." || len(groups[0].Tags) != 3 || groups[0].Tags[2] != "报告" {
		t.Fatalf("groups = %+v", groups)
	}
	if requestsByPath[userExpertsPath] != 2 || requestsByPath[userExpertGroupsPath] != 1 {
		t.Fatalf("requests = %v", requestsByPath)
	}
}

func TestHTTPExpertAssetsClientCreatesUserExpertAndGroup(t *testing.T) {
	owner := "usr_01"
	avatar := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	var requestPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestPaths = append(requestPaths, request.URL.Path)
		if request.Method != http.MethodPost || request.URL.RawQuery != "" {
			t.Fatalf("request = %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "Bearer access-current" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if err := request.ParseMultipartForm(6 * 1024 * 1024); err != nil {
			t.Fatalf("parse multipart request: %v", err)
		}
		file, header, err := request.FormFile("avatar")
		if err != nil {
			t.Fatalf("avatar file: %v", err)
		}
		defer file.Close()
		uploaded, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read avatar: %v", err)
		}
		if string(uploaded) != string(avatar) || header.Filename != "portrait.png" || header.Header.Get("Content-Type") != "image/png" {
			t.Fatalf("avatar = filename %q content-type %q bytes %v", header.Filename, header.Header.Get("Content-Type"), uploaded)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case userExpertsPath:
			if len(request.MultipartForm.Value) != 5 ||
				request.FormValue("name") != "Research" ||
				request.FormValue("description") != "Research evidence." ||
				request.FormValue("prompt") != "Find evidence." ||
				request.FormValue("category") != "111" ||
				request.FormValue("tags") != `["调研","洞察","报告"]` {
				t.Fatalf("expert request fields = %v", request.MultipartForm.Value)
			}
			_, _ = writer.Write([]byte(`{"data":{"expert":{
				"expert_id":"exp_01","avatar_url":"https://assets.example.com/expert.png",
				"name":"Research","description":"Research evidence.","prompt":"Find evidence.","category":"111",
				"tags":["调研","洞察","报告"],"usage_count":0,
				"type":"user","owner_user_id":"` + owner + `",
				"created_at":"2026-07-25T08:00:00Z","updated_at":"2026-07-25T08:00:00Z"
			}},"error":null,"request_id":"req-expert","server_time":"2026-07-25T08:00:00Z"}`))
		case userExpertGroupsPath:
			if len(request.MultipartForm.Value) != 5 ||
				request.FormValue("name") != "Research Team" ||
				request.FormValue("description") != "Review research." ||
				request.FormValue("category") != "跨学科评审" ||
				request.FormValue("expert_ids") != `["exp_system","exp_01"]` ||
				request.FormValue("tags") != `["评审","协作","报告"]` {
				t.Fatalf("group request fields = %v", request.MultipartForm.Value)
			}
			_, _ = writer.Write([]byte(`{"data":{"expert_group":{
				"expert_group_id":"exg_01","avatar_url":"https://assets.example.com/group.png",
				"name":"Research Team","description":"Review research.","category":"跨学科评审",
				"tags":["评审","协作","报告"],"usage_count":0,"expert_ids":["exp_system","exp_01"],
				"type":"user","owner_user_id":"` + owner + `",
				"created_at":"2026-07-25T08:05:00Z","updated_at":"2026-07-25T08:05:00Z"
			}},"error":null,"request_id":"req-group","server_time":"2026-07-25T08:05:00Z"}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-01", ClientVersion: "dev",
	}, nil)
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	client, err := NewHTTPExpertAssetsClient(expertAssetsTestExecutor{token: "access-current"}, transport)
	if err != nil {
		t.Fatalf("NewHTTPExpertAssetsClient: %v", err)
	}
	scope := BindingScope{ProfileID: "default", CloudUserID: owner, BindingGeneration: 1}
	created, err := client.CreateUserExpert(context.Background(), scope, CreateUserExpertAssetInput{
		AvatarFileName: "portrait.png", AvatarContentType: "image/png", Avatar: avatar,
		Name: " Research ", Description: " Research evidence. ", Prompt: " Find evidence. ", Category: " 111 ",
		Tags: []string{" 调研 ", "洞察", "报告"},
	})
	if err != nil {
		t.Fatalf("CreateUserExpert: %v", err)
	}
	if created.ExpertID != "exp_01" || created.Description != "Research evidence." || created.Category != "111" ||
		len(created.Tags) != 3 || created.Tags[0] != "调研" ||
		created.OwnerUserID == nil || *created.OwnerUserID != owner {
		t.Fatalf("created expert = %+v", created)
	}
	group, err := client.CreateUserExpertGroup(context.Background(), scope, CreateUserExpertGroupAssetInput{
		AvatarFileName: "portrait.png", AvatarContentType: "image/png", Avatar: avatar,
		Name: "Research Team", Description: "Review research.", Category: "跨学科评审",
		Tags: []string{"评审", "协作", "报告"}, ExpertIDs: []string{" exp_system ", "exp_01"},
	})
	if err != nil {
		t.Fatalf("CreateUserExpertGroup: %v", err)
	}
	if group.ExpertGroupID != "exg_01" || group.Description != "Review research." || group.Category != "跨学科评审" ||
		len(group.Tags) != 3 || group.Tags[2] != "报告" ||
		len(group.ExpertIDs) != 2 || group.ExpertIDs[0] != "exp_system" {
		t.Fatalf("created group = %+v", group)
	}
	if len(requestPaths) != 2 || requestPaths[0] != userExpertsPath || requestPaths[1] != userExpertGroupsPath {
		t.Fatalf("request paths = %v", requestPaths)
	}
}

func TestHTTPExpertAssetsClientListsCreatesAndDeletesExpertFavorites(t *testing.T) {
	const owner = "usr_01"
	const nextCursor = "exfav_next/+?="
	requests := make([]string, 0, 8)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		if request.Header.Get("Authorization") != "Bearer access-current" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == expertFavoritesPath && request.URL.Query().Get("cursor") == "":
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"favorite_id":"exfav_01","expert_id":"exp_system",
				"created_at":"2026-07-31T14:30:00Z","updated_at":"2026-07-31T14:30:00Z"
			}],"next_cursor":"` + nextCursor + `","has_more":true},"error":null}`))
		case request.Method == http.MethodGet && request.URL.Path == expertFavoritesPath && request.URL.Query().Get("cursor") == nextCursor:
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"favorite_id":"exfav_02","expert_id":"exp_owned",
				"created_at":"2026-07-31T14:31:00Z","updated_at":"2026-07-31T14:31:00Z"
			}],"next_cursor":null,"has_more":false},"error":null}`))
		case request.Method == http.MethodGet && request.URL.Path == expertGroupFavoritesPath:
			_, _ = writer.Write([]byte(`{"data":{"items":[{
				"favorite_id":"exgfav_01","expert_group_id":"exg_system",
				"created_at":"2026-07-31T14:32:00Z","updated_at":"2026-07-31T14:32:00Z"
			}],"next_cursor":null,"has_more":false},"error":null}`))
		case request.Method == http.MethodPost && request.URL.Path == expertFavoritesPath:
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || len(body) != 1 || body["expert_id"] != "exp_new" {
				t.Fatalf("expert favorite body = %#v, err = %v", body, err)
			}
			_, _ = writer.Write([]byte(`{"data":{"expert_favorite":{
				"favorite_id":"exfav_new","expert_id":"exp_new",
				"created_at":"2026-07-31T14:33:00Z","updated_at":"2026-07-31T14:33:00Z"
			}},"error":null}`))
		case request.Method == http.MethodPost && request.URL.Path == expertGroupFavoritesPath:
			var body map[string]string
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || len(body) != 1 || body["expert_group_id"] != "exg_new" {
				t.Fatalf("expert group favorite body = %#v, err = %v", body, err)
			}
			_, _ = writer.Write([]byte(`{"data":{"expert_group_favorite":{
				"favorite_id":"exgfav_new","expert_group_id":"exg_new",
				"created_at":"2026-07-31T14:34:00Z","updated_at":"2026-07-31T14:34:00Z"
			}},"error":null}`))
		case request.Method == http.MethodDelete && request.URL.Path == expertFavoritesPath+"/exfav_new":
			_, _ = writer.Write([]byte(`{"data":{"deleted_favorite_id":"exfav_new"},"error":null}`))
		case request.Method == http.MethodDelete && request.URL.Path == expertGroupFavoritesPath+"/exgfav_new":
			_, _ = writer.Write([]byte(`{"data":{"deleted_favorite_id":"exgfav_new"},"error":null}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-01", ClientVersion: "dev",
	}, nil)
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	client, err := NewHTTPExpertAssetsClient(expertAssetsTestExecutor{token: "access-current"}, transport)
	if err != nil {
		t.Fatalf("NewHTTPExpertAssetsClient: %v", err)
	}
	favoritesClient, ok := client.(ExpertFavoritesClient)
	if !ok {
		t.Fatal("HTTP expert assets client does not implement ExpertFavoritesClient")
	}
	scope := BindingScope{ProfileID: "default", CloudUserID: owner, BindingGeneration: 1}
	expertFavorites, err := favoritesClient.ListExpertFavorites(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListExpertFavorites: %v", err)
	}
	groupFavorites, err := favoritesClient.ListExpertGroupFavorites(context.Background(), scope)
	if err != nil {
		t.Fatalf("ListExpertGroupFavorites: %v", err)
	}
	if len(expertFavorites) != 2 || expertFavorites[0].ExpertID != "exp_system" || expertFavorites[1].ExpertID != "exp_owned" {
		t.Fatalf("expert favorites = %+v", expertFavorites)
	}
	if len(groupFavorites) != 1 || groupFavorites[0].ExpertGroupID != "exg_system" {
		t.Fatalf("expert group favorites = %+v", groupFavorites)
	}
	createdExpert, err := favoritesClient.CreateExpertFavorite(context.Background(), scope, " exp_new ")
	if err != nil || createdExpert.FavoriteID != "exfav_new" {
		t.Fatalf("CreateExpertFavorite = %+v, %v", createdExpert, err)
	}
	createdGroup, err := favoritesClient.CreateExpertGroupFavorite(context.Background(), scope, " exg_new ")
	if err != nil || createdGroup.FavoriteID != "exgfav_new" {
		t.Fatalf("CreateExpertGroupFavorite = %+v, %v", createdGroup, err)
	}
	if err := favoritesClient.DeleteExpertFavorite(context.Background(), scope, createdExpert.FavoriteID); err != nil {
		t.Fatalf("DeleteExpertFavorite: %v", err)
	}
	if err := favoritesClient.DeleteExpertGroupFavorite(context.Background(), scope, createdGroup.FavoriteID); err != nil {
		t.Fatalf("DeleteExpertGroupFavorite: %v", err)
	}
	if len(requests) != 7 {
		t.Fatalf("requests = %v", requests)
	}
}

type errorReportTestExecutor struct {
	tokens      []string
	calls       int
	deadline    time.Time
	hasDeadline bool
}

func (e *errorReportTestExecutor) Do(ctx context.Context, scope BindingScope, request func(context.Context, string) error) error {
	e.calls++
	e.deadline, e.hasDeadline = ctx.Deadline()
	if err := scope.Validate(); err != nil {
		return err
	}
	for index, token := range e.tokens {
		err := request(ctx, token)
		if err == nil {
			return nil
		}
		if index+1 == len(e.tokens) || !isCloudCode(err, ErrorCode("TOKEN_EXPIRED")) {
			return err
		}
	}
	return errors.New("test executor has no access token")
}

type errorReportMultipartPart struct {
	filename    string
	contentType string
	data        []byte
}

func TestHTTPErrorReportClientSubmitsExactMultipartContract(t *testing.T) {
	for _, test := range []struct {
		name       string
		attachment *ErrorReportAttachmentInput
		replayed   bool
	}{
		{name: "text only"},
		{
			name: "single sanitized image",
			attachment: &ErrorReportAttachmentInput{
				Filename: "feedback.png", ContentType: errorReportContentTypePNG, Data: []byte("sanitized-png"),
			},
			replayed: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests++
				if request.Method != http.MethodPost || request.URL.Path != errorReportsPath || request.URL.RawQuery != "" {
					t.Fatalf("request = %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
				}
				if request.Header.Get("Authorization") != "Bearer access-feedback" {
					t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
				}
				if request.Header.Get("X-Device-Id") != "device-feedback" || request.Header.Get("X-Client-Version") != "0.9.0" {
					t.Fatalf("device headers = %q %q", request.Header.Get("X-Device-Id"), request.Header.Get("X-Client-Version"))
				}
				if request.Header.Get(errorReportIdempotencyHeader) != "feedback-attempt-01" {
					t.Fatalf("%s = %q", errorReportIdempotencyHeader, request.Header.Get(errorReportIdempotencyHeader))
				}
				parts := readErrorReportMultipart(t, request)
				if len(parts) != 1 && test.attachment == nil {
					t.Fatalf("parts = %#v, want description only", parts)
				}
				description, ok := parts[errorReportDescriptionField]
				if !ok || string(description.data) != "导出文件时应用闪退" || description.filename != "" {
					t.Fatalf("description part = %#v", description)
				}
				file, hasFile := parts[errorReportAttachmentField]
				if test.attachment == nil {
					if hasFile {
						t.Fatalf("text-only request included file part: %#v", file)
					}
				} else if !hasFile ||
					file.filename != test.attachment.Filename ||
					file.contentType != test.attachment.ContentType ||
					!bytes.Equal(file.data, test.attachment.Data) {
					t.Fatalf("file part = %#v", file)
				}
				writer.Header().Set("Content-Type", "application/json")
				replayed := "false"
				if test.replayed {
					replayed = "true"
				}
				_, _ = writer.Write([]byte(`{
					"data":{"report":{"report_id":"errpt_01","status":"submitted"},"replayed":` + replayed + `},
					"error":null,"request_id":"req-feedback","server_time":"2026-07-30T04:00:01Z"
				}`))
			}))
			defer server.Close()

			executor := &errorReportTestExecutor{tokens: []string{"access-feedback"}}
			client := newErrorReportTestClient(t, server, executor)
			result, err := client.SubmitErrorReport(context.Background(), errorReportTestScope(), SubmitErrorReportInput{
				Description: "导出文件时应用闪退", IdempotencyKey: "feedback-attempt-01", Attachment: test.attachment,
			})
			if err != nil {
				t.Fatalf("SubmitErrorReport: %v", err)
			}
			if requests != 1 || executor.calls != 1 {
				t.Fatalf("requests = %d, executor calls = %d", requests, executor.calls)
			}
			if result != (ErrorReportSubmission{ReportID: "errpt_01", Status: ErrorReportStatusSubmitted, Replayed: test.replayed}) {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestHTTPErrorReportClientGetsOwnedReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != errorReportsPath+"/errpt_01" || request.URL.RawQuery != "" {
			t.Fatalf("request = %s %s?%s", request.Method, request.URL.Path, request.URL.RawQuery)
		}
		if request.Header.Get("Authorization") != "Bearer access-feedback" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get(errorReportIdempotencyHeader) != "" || request.Header.Get("Content-Type") != "" {
			t.Fatalf("query headers included submit-only values: %#v", request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data":{"report":{"report_id":"errpt_01","status":"submitted","description":"问题描述"}},
			"error":null,"request_id":"req-query","server_time":"2026-07-30T04:02:00Z"
		}`))
	}))
	defer server.Close()

	executor := &errorReportTestExecutor{tokens: []string{"access-feedback"}}
	client := newErrorReportTestClient(t, server, executor)
	result, err := client.GetErrorReport(context.Background(), errorReportTestScope(), "errpt_01")
	if err != nil {
		t.Fatalf("GetErrorReport: %v", err)
	}
	if result != (ErrorReportConfirmation{ReportID: "errpt_01", Status: ErrorReportStatusSubmitted}) {
		t.Fatalf("result = %#v", result)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d", executor.calls)
	}
}

func TestHTTPErrorReportClientRejectsInvalidQueryAndMismatchedReport(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data":{"report":{"report_id":"errpt_other","status":"submitted"}},
			"error":null,"request_id":"req-query-mismatch","server_time":"2026-07-30T04:02:00Z"
		}`))
	}))
	defer server.Close()

	executor := &errorReportTestExecutor{tokens: []string{"access-feedback"}}
	client := newErrorReportTestClient(t, server, executor)
	if _, err := client.GetErrorReport(context.Background(), errorReportTestScope(), "../secret"); err == nil {
		t.Fatal("invalid report ID reached query")
	}
	if requests != 0 || executor.calls != 0 {
		t.Fatalf("invalid query reached executor/http: executor=%d http=%d", executor.calls, requests)
	}
	_, err := client.GetErrorReport(context.Background(), errorReportTestScope(), "errpt_expected")
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != ErrorCode("CLOUD_PROTOCOL_ERROR") {
		t.Fatalf("mismatched query error = %#v", err)
	}
}

func TestHTTPErrorReportClientReplaysIdenticalMultipartAfterTokenExpiry(t *testing.T) {
	var bodies [][]byte
	var contentTypes []string
	var idempotencyKeys []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		bodies = append(bodies, body)
		contentTypes = append(contentTypes, request.Header.Get("Content-Type"))
		idempotencyKeys = append(idempotencyKeys, request.Header.Get(errorReportIdempotencyHeader))
		writer.Header().Set("Content-Type", "application/json")
		if len(bodies) == 1 {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(`{
				"data":null,
				"error":{"code":"TOKEN_EXPIRED","message":"expired","retryable":true},
				"request_id":"req-expired","server_time":"2026-07-30T04:00:00Z"
			}`))
			return
		}
		_, _ = writer.Write([]byte(`{
			"data":{"report":{"report_id":"errpt_replayed","status":"submitted"},"replayed":true},
			"error":null,"request_id":"req-replayed","server_time":"2026-07-30T04:00:01Z"
		}`))
	}))
	defer server.Close()

	executor := &errorReportTestExecutor{tokens: []string{"access-old", "access-new"}}
	client := newErrorReportTestClient(t, server, executor)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	wantDeadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("test context omitted deadline")
	}
	result, err := client.SubmitErrorReport(ctx, errorReportTestScope(), SubmitErrorReportInput{
		Description:    "问题描述",
		IdempotencyKey: "feedback-attempt-replay",
		Attachment: &ErrorReportAttachmentInput{
			Filename: "diagnostic.zip", ContentType: errorReportContentTypeZIP, Data: []byte("stable-zip"),
		},
	})
	if err != nil {
		t.Fatalf("SubmitErrorReport: %v", err)
	}
	if result.ReportID != "errpt_replayed" || !result.Replayed {
		t.Fatalf("result = %#v", result)
	}
	if len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("replayed multipart bodies differ")
	}
	if len(contentTypes) != 2 || contentTypes[0] == "" || contentTypes[0] != contentTypes[1] {
		t.Fatalf("content types = %#v", contentTypes)
	}
	if len(idempotencyKeys) != 2 || idempotencyKeys[0] != "feedback-attempt-replay" || idempotencyKeys[0] != idempotencyKeys[1] {
		t.Fatalf("idempotency keys = %#v", idempotencyKeys)
	}
	if !executor.hasDeadline || !executor.deadline.Equal(wantDeadline) {
		t.Fatalf("executor deadline = %v/%v, want %v", executor.deadline, executor.hasDeadline, wantDeadline)
	}
}

func TestHTTPErrorReportClientRejectsInvalidInputBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	executor := &errorReportTestExecutor{tokens: []string{"access-feedback"}}
	client := newErrorReportTestClient(t, server, executor)

	oversizedAttachment := bytes.Repeat([]byte("x"), ErrorReportAttachmentMaxBytes+1)
	for _, test := range []struct {
		name  string
		input SubmitErrorReportInput
	}{
		{name: "blank description", input: SubmitErrorReportInput{Description: " ", IdempotencyKey: "key"}},
		{name: "untrimmed description", input: SubmitErrorReportInput{Description: " detail ", IdempotencyKey: "key"}},
		{name: "oversized description", input: SubmitErrorReportInput{Description: strings.Repeat("界", 3334), IdempotencyKey: "key"}},
		{name: "blank idempotency key", input: SubmitErrorReportInput{Description: "detail", IdempotencyKey: " "}},
		{
			name: "path filename",
			input: SubmitErrorReportInput{
				Description: "detail", IdempotencyKey: "key",
				Attachment: &ErrorReportAttachmentInput{Filename: `C:\secret.png`, ContentType: errorReportContentTypePNG, Data: []byte("png")},
			},
		},
		{
			name: "unsupported type",
			input: SubmitErrorReportInput{
				Description: "detail", IdempotencyKey: "key",
				Attachment: &ErrorReportAttachmentInput{Filename: "secret.webp", ContentType: "image/webp", Data: []byte("webp")},
			},
		},
		{
			name: "oversized attachment",
			input: SubmitErrorReportInput{
				Description: "detail", IdempotencyKey: "key",
				Attachment: &ErrorReportAttachmentInput{Filename: "large.png", ContentType: errorReportContentTypePNG, Data: oversizedAttachment},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := client.SubmitErrorReport(context.Background(), errorReportTestScope(), test.input); err == nil {
				t.Fatal("SubmitErrorReport returned nil error")
			}
		})
	}
	if requests != 0 || executor.calls != 0 {
		t.Fatalf("invalid input reached executor/http: executor=%d http=%d", executor.calls, requests)
	}
}

func TestHTTPErrorReportClientPreservesCloudErrorsAndRejectsInvalidSuccess(t *testing.T) {
	for _, test := range []struct {
		name          string
		status        int
		response      string
		wantCode      ErrorCode
		wantRequestID string
	}{
		{
			name:   "cloud error",
			status: http.StatusConflict,
			response: `{
				"data":null,
				"error":{"code":"IDEMPOTENCY_CONFLICT","message":"conflict","retryable":false},
				"request_id":"req-conflict","server_time":"2026-07-30T04:00:00Z"
			}`,
			wantCode: ErrorIdempotencyConflict, wantRequestID: "req-conflict",
		},
		{
			name:   "missing report id",
			status: http.StatusOK,
			response: `{
				"data":{"report":{"status":"submitted"},"replayed":false},
				"error":null,"request_id":"req-invalid","server_time":"2026-07-30T04:00:00Z"
			}`,
			wantCode: ErrorCode("CLOUD_PROTOCOL_ERROR"),
		},
		{
			name:   "unsupported status",
			status: http.StatusOK,
			response: `{
				"data":{"report":{"report_id":"errpt_invalid","status":"processing"},"replayed":false},
				"error":null,"request_id":"req-invalid","server_time":"2026-07-30T04:00:00Z"
			}`,
			wantCode: ErrorCode("CLOUD_PROTOCOL_ERROR"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.response))
			}))
			defer server.Close()
			client := newErrorReportTestClient(t, server, &errorReportTestExecutor{tokens: []string{"access-feedback"}})
			_, err := client.SubmitErrorReport(context.Background(), errorReportTestScope(), SubmitErrorReportInput{
				Description: "detail", IdempotencyKey: "feedback-attempt-error",
			})
			var typed *Error
			if !errors.As(err, &typed) || typed.Code != test.wantCode || typed.RequestID != test.wantRequestID {
				t.Fatalf("error = %#v, want code=%q request_id=%q", err, test.wantCode, test.wantRequestID)
			}
		})
	}
}

func readErrorReportMultipart(t *testing.T, request *http.Request) map[string]errorReportMultipartPart {
	t.Helper()
	mediaType, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || parameters["boundary"] == "" {
		t.Fatalf("Content-Type = %q: %v", request.Header.Get("Content-Type"), err)
	}
	reader := multipart.NewReader(request.Body, parameters["boundary"])
	parts := make(map[string]errorReportMultipartPart)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return parts
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		if _, duplicate := parts[part.FormName()]; duplicate {
			t.Fatalf("duplicate multipart field %q", part.FormName())
		}
		parts[part.FormName()] = errorReportMultipartPart{
			filename: part.FileName(), contentType: part.Header.Get("Content-Type"), data: data,
		}
	}
}

func newErrorReportTestClient(t *testing.T, server *httptest.Server, executor AuthenticatedRequestExecutor) ErrorReportClient {
	t.Helper()
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{
		DeviceID: "device-feedback", ClientVersion: "0.9.0",
	}, func() string { return "req-client-feedback" })
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	client, err := NewHTTPErrorReportClient(executor, transport)
	if err != nil {
		t.Fatalf("NewHTTPErrorReportClient: %v", err)
	}
	return client
}

func errorReportTestScope() BindingScope {
	return BindingScope{ProfileID: "feedback", CloudUserID: "usr_feedback", BindingGeneration: 1}
}
