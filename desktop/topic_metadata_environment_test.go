package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/brand"
)

var desktopTopicMetadataFiles = []string{
	topicTitlesFile,
	topicTitleSourcesFile,
	topicCreatedAtsFile,
	topicModelsFile,
}

func TestProductionProjectTopicMetadataPathsUseBrandDirectory(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	initializerCalled := false
	for _, filename := range desktopTopicMetadataFiles {
		path, err := resolveTopicMetadataPath(
			"v1.2.3",
			workspace,
			filename,
			func() (desktopProfile, error) {
				initializerCalled = true
				return desktopProfile{}, errors.New("production must not initialize a dev profile")
			},
			func() string { return filepath.Join(t.TempDir(), "desktop") },
		)
		if err != nil {
			t.Fatalf("resolve %s: %v", filename, err)
		}
		if want := filepath.Join(workspace, brand.ProjectDirName, filename); path != want {
			t.Fatalf("production %s path = %q, want %q", filename, path, want)
		}
	}
	if initializerCalled {
		t.Fatal("production topic path read the development profile")
	}
}

func TestDevelopmentProjectTopicMetadataPathsShareEnvironmentNamespace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	profile := desktopProfile{Development: true, EnvironmentID: "0123456789abcdef"}
	resolver := testTopicMetadataResolver("dev", profile, nil, filepath.Join(t.TempDir(), "profile"))
	wantDir := filepath.Join(workspace, brand.ProjectDirName, "desktop", "environments", profile.EnvironmentID)
	for _, filename := range desktopTopicMetadataFiles {
		path, err := resolver(workspace, filename)
		if err != nil {
			t.Fatalf("resolve %s: %v", filename, err)
		}
		if want := filepath.Join(wantDir, filename); path != want {
			t.Fatalf("development %s path = %q, want %q", filename, path, want)
		}
	}
}

func TestDevelopmentTopicEnvironmentIsStableAcrossWorkingDirectories(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "shared-workspace")
	locations := testDesktopProfileLocations(base)
	firstEnv := map[string]string{
		cloudBaseURLEnv:   " HTTPS://Cloud.Example.COM:443//api/ ",
		devProfileRootEnv: filepath.Join(base, "profile-a"),
	}
	first, err := initializeDesktopProfile("dev", mapGetenv(firstEnv), mapSetenv(firstEnv), locations)
	if err != nil {
		t.Fatal(err)
	}

	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })
	otherCWD := filepath.Join(base, "another-worktree")
	if err := os.MkdirAll(otherCWD, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatal(err)
	}
	secondEnv := map[string]string{
		cloudBaseURLEnv:   "https://cloud.example.com/api",
		devProfileRootEnv: filepath.Join(base, "profile-b"),
	}
	second, err := initializeDesktopProfile("dev", mapGetenv(secondEnv), mapSetenv(secondEnv), locations)
	if err != nil {
		t.Fatal(err)
	}
	thirdEnv := map[string]string{
		cloudBaseURLEnv:   "https://other.example.com/api",
		devProfileRootEnv: filepath.Join(base, "profile-c"),
	}
	third, err := initializeDesktopProfile("dev", mapGetenv(thirdEnv), mapSetenv(thirdEnv), locations)
	if err != nil {
		t.Fatal(err)
	}

	firstPath, err := testTopicMetadataResolver("dev", first, nil, first.HomeRoot)(workspace, topicTitlesFile)
	if err != nil {
		t.Fatal(err)
	}
	secondPath, err := testTopicMetadataResolver("dev", second, nil, second.HomeRoot)(workspace, topicTitlesFile)
	if err != nil {
		t.Fatal(err)
	}
	thirdPath, err := testTopicMetadataResolver("dev", third, nil, third.HomeRoot)(workspace, topicTitlesFile)
	if err != nil {
		t.Fatal(err)
	}
	if firstPath != secondPath {
		t.Fatalf("same cloud URL changed topic path across cwd/worktree: %q != %q", firstPath, secondPath)
	}
	if firstPath == thirdPath {
		t.Fatalf("different cloud URLs shared topic path %q", firstPath)
	}
}

func TestDevelopmentTopicMetadataIgnoresAndPreservesLegacyProjectFiles(t *testing.T) {
	workspace := t.TempDir()
	legacyDir := filepath.Join(workspace, ".kwu")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := map[string][]byte{
		topicTitlesFile:       []byte(`{"legacy":"production title"}`),
		topicTitleSourcesFile: []byte(`{"legacy":"manual"}`),
		topicCreatedAtsFile:   []byte(`{"legacy":123}`),
		topicModelsFile:       []byte(`{"legacy":"provider/model"}`),
	}
	for filename, data := range legacy {
		if err := os.WriteFile(filepath.Join(legacyDir, filename), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	resolver := testTopicMetadataResolver(
		"dev",
		desktopProfile{Development: true, EnvironmentID: "0123456789abcdef"},
		nil,
		filepath.Join(t.TempDir(), "profile"),
	)

	if got := loadTopicStringMap(workspace, topicTitlesFile, resolver).values; len(got) != 0 {
		t.Fatalf("dev read legacy titles: %#v", got)
	}
	if got := loadTopicStringMap(workspace, topicTitleSourcesFile, resolver).values; len(got) != 0 {
		t.Fatalf("dev read legacy title sources: %#v", got)
	}
	if got := loadTopicInt64Map(workspace, topicCreatedAtsFile, resolver).values; len(got) != 0 {
		t.Fatalf("dev read legacy created-at values: %#v", got)
	}
	if got := loadTopicStringMap(workspace, topicModelsFile, resolver).values; len(got) != 0 {
		t.Fatalf("dev read legacy models: %#v", got)
	}

	if err := saveTopicMetadataMap(workspace, topicTitlesFile, map[string]string{"dev": "development title"}, resolver); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicMetadataMap(workspace, topicTitleSourcesFile, map[string]string{"dev": "auto"}, resolver); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicMetadataMap(workspace, topicCreatedAtsFile, map[string]int64{"dev": 456}, resolver); err != nil {
		t.Fatal(err)
	}
	if err := saveTopicMetadataMap(workspace, topicModelsFile, map[string]string{"dev": "dev/model"}, resolver); err != nil {
		t.Fatal(err)
	}
	for filename, want := range legacy {
		got, err := os.ReadFile(filepath.Join(legacyDir, filename))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("legacy %s changed: got %q, want %q", filename, got, want)
		}
	}
}

func TestDevelopmentTopicMetadataEnvironmentsDoNotPolluteEachOther(t *testing.T) {
	workspace := t.TempDir()
	resolverA := testTopicMetadataResolver("dev", desktopProfile{Development: true, EnvironmentID: "aaaaaaaaaaaaaaaa"}, nil, filepath.Join(t.TempDir(), "profile-a"))
	resolverB := testTopicMetadataResolver("dev", desktopProfile{Development: true, EnvironmentID: "bbbbbbbbbbbbbbbb"}, nil, filepath.Join(t.TempDir(), "profile-b"))
	tests := []struct {
		filename string
		valueA   any
		valueB   any
	}{
		{filename: topicTitlesFile, valueA: map[string]string{"topic": "A title"}, valueB: map[string]string{"topic": "B title"}},
		{filename: topicTitleSourcesFile, valueA: map[string]string{"topic": "auto"}, valueB: map[string]string{"topic": "manual"}},
		{filename: topicCreatedAtsFile, valueA: map[string]int64{"topic": 100}, valueB: map[string]int64{"topic": 200}},
		{filename: topicModelsFile, valueA: map[string]string{"topic": "a/model"}, valueB: map[string]string{"topic": "b/model"}},
	}
	for _, test := range tests {
		if err := saveTopicMetadataMap(workspace, test.filename, test.valueA, resolverA); err != nil {
			t.Fatalf("save environment A %s: %v", test.filename, err)
		}
		if err := saveTopicMetadataMap(workspace, test.filename, test.valueB, resolverB); err != nil {
			t.Fatalf("save environment B %s: %v", test.filename, err)
		}
		if test.filename == topicCreatedAtsFile {
			if got := loadTopicInt64Map(workspace, test.filename, resolverA).values; !reflect.DeepEqual(got, test.valueA) {
				t.Fatalf("environment A %s = %#v, want %#v", test.filename, got, test.valueA)
			}
			if got := loadTopicInt64Map(workspace, test.filename, resolverB).values; !reflect.DeepEqual(got, test.valueB) {
				t.Fatalf("environment B %s = %#v, want %#v", test.filename, got, test.valueB)
			}
			continue
		}
		if got := loadTopicStringMap(workspace, test.filename, resolverA).values; !reflect.DeepEqual(got, test.valueA) {
			t.Fatalf("environment A %s = %#v, want %#v", test.filename, got, test.valueA)
		}
		if got := loadTopicStringMap(workspace, test.filename, resolverB).values; !reflect.DeepEqual(got, test.valueB) {
			t.Fatalf("environment B %s = %#v, want %#v", test.filename, got, test.valueB)
		}
	}
}

func TestDevelopmentGlobalTopicMetadataStaysAtProfileGlobalRoot(t *testing.T) {
	profileRoot := filepath.Join(t.TempDir(), "profile")
	profile := desktopProfile{Development: true, EnvironmentID: "0123456789abcdef"}
	resolver := testTopicMetadataResolver("dev", profile, nil, profileRoot)
	for _, filename := range desktopTopicMetadataFiles {
		path, err := resolver("", filename)
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(profileRoot, "global", filename); path != want {
			t.Fatalf("global %s path = %q, want %q", filename, path, want)
		}
		if strings.Contains(path, filepath.Join("desktop", "environments", profile.EnvironmentID)) {
			t.Fatalf("global %s path redundantly nests environment ID: %q", filename, path)
		}
	}
}

func TestDevelopmentTopicMetadataProfileFailuresNeverFallBackToLegacyPath(t *testing.T) {
	workspace := t.TempDir()
	legacyPath := filepath.Join(workspace, ".kwu", topicTitlesFile)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyData := []byte(`{"legacy":"production title"}`)
	if err := os.WriteFile(legacyPath, legacyData, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		profile desktopProfile
		err     error
	}{
		{name: "initialization failure", err: errors.New("marker conflict")},
		{name: "missing ID", profile: desktopProfile{Development: true}},
		{name: "invalid ID", profile: desktopProfile{Development: true, EnvironmentID: "../production"}},
		{name: "non-development profile", profile: desktopProfile{EnvironmentID: "0123456789abcdef"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := testTopicMetadataResolver("dev", test.profile, test.err, filepath.Join(t.TempDir(), "profile"))
			if path, err := resolver(workspace, topicTitlesFile); err == nil || path != "" {
				t.Fatalf("failed profile resolved path %q, err=%v", path, err)
			}
			if got := loadTopicStringMap(workspace, topicTitlesFile, resolver).values; len(got) != 0 {
				t.Fatalf("failed profile read legacy data: %#v", got)
			}
			if err := saveTopicMetadataMap(workspace, topicTitlesFile, map[string]string{"dev": "unsafe"}, resolver); err == nil {
				t.Fatal("failed profile wrote topic metadata")
			}
			got, err := os.ReadFile(legacyPath)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, legacyData) {
				t.Fatalf("failed profile changed legacy data: got %q, want %q", got, legacyData)
			}
		})
	}
}

func testTopicMetadataResolver(buildVersion string, profile desktopProfile, profileErr error, desktopConfigRoot string) topicMetadataPathResolver {
	return func(workspaceRoot, filename string) (string, error) {
		return resolveTopicMetadataPath(
			buildVersion,
			workspaceRoot,
			filename,
			func() (desktopProfile, error) { return profile, profileErr },
			func() string { return desktopConfigRoot },
		)
	}
}
