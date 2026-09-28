package connector

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"testing"
)

func TestHTTPSOnlyPackageContract(t *testing.T) {
	tests := []struct {
		name   string
		files  func() map[string][]byte
		accept bool
	}{
		{name: "none", files: func() map[string][]byte { return httpsFixture("none", false) }, accept: true},
		{name: "oauth", files: func() map[string][]byte { return httpsFixture("oauth", false) }, accept: true},
		{name: "platform-neutral OAuth resource", files: func() map[string][]byte {
			files := httpsFixture("oauth", false)
			mutateJSON(t, files, "connector.json", func(value map[string]any) { delete(value, "platforms") })
			mutateFirstServer(t, files, func(server map[string]any) {
				server["auth"].(map[string]any)["resource"] = "https://mcp.example.com"
			})
			return files
		}, accept: true},
		{name: "cross-origin OAuth resource", files: func() map[string][]byte {
			files := httpsFixture("oauth", false)
			mutateFirstServer(t, files, func(server map[string]any) {
				server["auth"].(map[string]any)["resource"] = "https://other.example.com"
			})
			return files
		}},
		{name: "github OAuth provider", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) {
				server["url"] = "https://api.githubcopilot.com/mcp/"
				server["auth"] = map[string]any{"type": "oauth", "provider": "github"}
			})
			return files
		}, accept: true},
		{name: "token schema referenced by mcp auth", files: func() map[string][]byte { return httpsFixture("token", false) }, accept: true},
		{name: "mcp with prompt-only skill", files: func() map[string][]byte { return httpsFixture("none", true) }, accept: true},
		{name: "token entrypoint", files: func() map[string][]byte {
			files := httpsFixture("token", false)
			mutateJSON(t, files, "connector.json", func(value map[string]any) { value["entrypoints"].(map[string]any)["token"] = "token-schema.json" })
			return files
		}},
		{name: "cli entrypoint and file", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateJSON(t, files, "connector.json", func(value map[string]any) { value["entrypoints"].(map[string]any)["cli"] = "cli.json" })
			files["cli.json"] = []byte(`{"schema_version":2}`)
			return files
		}},
		{name: "stdio", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["transport"] = "stdio"; delete(server, "url") })
			return files
		}},
		{name: "sse", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["transport"] = "sse" })
			return files
		}},
		{name: "command", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["command"] = map[string]any{"executable": "node"} })
			return files
		}},
		{name: "skills only", files: func() map[string][]byte {
			files := httpsFixture("none", true)
			delete(files, "mcp.json")
			mutateJSON(t, files, "connector.json", func(value map[string]any) {
				value["kind"] = "skills"
				value["capabilities"] = []any{"skills"}
				delete(value["entrypoints"].(map[string]any), "mcp")
			})
			return files
		}},
		{name: "http url", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "http://mcp.example.com/mcp" })
			return files
		}},
		{name: "localhost url", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "https://localhost/mcp" })
			return files
		}},
		{name: "private ip url", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "https://192.168.1.2/mcp" })
			return files
		}},
		{name: "userinfo url", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "https://user:pass@mcp.example.com/mcp" })
			return files
		}},
		{name: "token in query", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "https://mcp.example.com/mcp?access_token=value" })
			return files
		}},
		{name: "token in url template", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) { server["url"] = "https://mcp.example.com/{token}" })
			return files
		}},
		{name: "cookie token header", files: func() map[string][]byte {
			files := httpsFixture("token", false)
			mutateJSON(t, files, "token-schema.json", func(value map[string]any) {
				value["headers"] = []any{map[string]any{"name": "Cookie", "template": "token={api_key}"}}
			})
			return files
		}},
		{name: "token missing documentation url", files: func() map[string][]byte {
			files := httpsFixture("token", false)
			mutateJSON(t, files, "token-schema.json", func(value map[string]any) { delete(value, "documentation_url") })
			return files
		}},
		{name: "token documentation is not public https", files: func() map[string][]byte {
			files := httpsFixture("token", false)
			mutateJSON(t, files, "token-schema.json", func(value map[string]any) { value["documentation_url"] = "https://localhost/tokens" })
			return files
		}},
		{name: "unreferenced token schema", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			files["token-schema.json"] = tokenSchemaFixture()
			return files
		}},
		{name: "provider outside oauth", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) {
				server["auth"] = map[string]any{"type": "none", "provider": "github"}
			})
			return files
		}},
		{name: "unsupported oauth provider", files: func() map[string][]byte {
			files := httpsFixture("none", false)
			mutateFirstServer(t, files, func(server map[string]any) {
				server["auth"] = map[string]any{"type": "oauth", "provider": "unknown"}
			})
			return files
		}},
		{name: "skill extra markdown", files: func() map[string][]byte {
			files := httpsFixture("none", true)
			files["skills/sample/reference.md"] = []byte("extra")
			return files
		}},
		{name: "skill script", files: func() map[string][]byte {
			files := httpsFixture("none", true)
			files["skills/sample/run.ps1"] = []byte("exit 0")
			return files
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := test.files()
			item := fixtureCatalogItem(files)
			_, err := parseFixturePackage(files, item)
			if test.accept && err != nil {
				t.Fatalf("valid HTTPS-only package rejected: %v", err)
			}
			if !test.accept && err == nil {
				t.Fatal("prohibited managed package accepted")
			}
		})
	}
}

func httpsFixture(authType string, skills bool) map[string][]byte {
	kind := "mcp"
	capabilities := []any{"remote_mcp"}
	entrypoints := map[string]any{"mcp": "mcp.json"}
	if skills {
		kind = "composite"
		capabilities = append(capabilities, "skills")
		entrypoints["skills"] = "skills"
	}
	auth := map[string]any{"type": authType}
	if authType == "oauth" {
		auth["scopes"] = []any{"docs.read"}
	}
	if authType == "token" {
		auth["schema"] = "token-schema.json"
	}
	files := map[string][]byte{
		"connector.json": mustJSON(map[string]any{
			"schema_version": 2, "id": "fixture-connector", "version": "1.0.0", "name": "Fixture Connector",
			"provider": "Fixture Provider", "description": "HTTPS-only fixture.", "category": "test", "examples": []any{"Read a document"},
			"kind": kind, "capabilities": capabilities, "min_client_version": "0.1.0",
			"platforms": []any{map[string]any{"os": "windows", "architectures": []any{"x64"}}}, "entrypoints": entrypoints,
		}),
		"mcp.json": mustJSON(map[string]any{
			"schema_version": 2,
			"servers":        []any{map[string]any{"id": "remote", "transport": "streamable-http", "url": "https://mcp.example.com/mcp", "auth": auth}},
		}),
	}
	if authType == "token" {
		files["token-schema.json"] = tokenSchemaFixture()
	}
	if skills {
		files["skills/sample/SKILL.md"] = []byte("Use the remote MCP tools from this connector.")
	}
	return files
}

func tokenSchemaFixture() []byte {
	return mustJSON(map[string]any{
		"schema_version": 1, "documentation_label": "Get an API key", "documentation_url": "https://docs.example.com/tokens",
		"fields":  []any{map[string]any{"id": "api_key", "label": "API key", "type": "secret", "required": true}},
		"headers": []any{map[string]any{"name": "Authorization", "template": "Bearer {api_key}"}},
	})
}

func parseFixturePackage(files map[string][]byte, item CatalogItem) (Manifest, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, raw := range files {
		entry, err := writer.Create(name)
		if err != nil {
			return Manifest{}, err
		}
		if _, err := entry.Write(raw); err != nil {
			return Manifest{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Manifest{}, err
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		return Manifest{}, err
	}
	inspected, err := inspectPackageZip(reader)
	if err != nil {
		return Manifest{}, err
	}
	return parseManifests(inspected, item, testPolicy("dev"))
}

func fixtureCatalogItem(files map[string][]byte) CatalogItem {
	var manifest ConnectorManifest
	_ = json.Unmarshal(files["connector.json"], &manifest)
	return CatalogItem{
		ID: manifest.ID, Name: manifest.Name, Provider: manifest.Provider, Description: manifest.Description,
		Category: manifest.Category, Examples: manifest.Examples, Kind: manifest.Kind, Version: manifest.Version,
		MinClientVersion: manifest.MinClientVersion, Compatible: true, Capabilities: manifest.Capabilities,
	}
}

func mutateFirstServer(t *testing.T, files map[string][]byte, mutate func(map[string]any)) {
	t.Helper()
	mutateJSON(t, files, "mcp.json", func(value map[string]any) {
		mutate(value["servers"].([]any)[0].(map[string]any))
	})
}

func mutateJSON(t *testing.T, files map[string][]byte, name string, mutate func(map[string]any)) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(files[name], &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	files[name] = mustJSON(value)
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
