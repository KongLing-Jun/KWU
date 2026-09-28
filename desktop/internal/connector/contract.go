package connector

// These DTOs mirror the schema 2 wire implemented by Reasonix-Cloud. Keep
// them data-only: product behavior belongs in the package, never in an ID
// registry in Desktop.

const (
	ConnectorSchemaVersion = 2
	MCPSchemaVersion       = 2
	TokenSchemaVersion     = 1
)

type ConnectorManifest struct {
	SchemaVersion    int                 `json:"schema_version"`
	ID               string              `json:"id"`
	Version          string              `json:"version"`
	Name             string              `json:"name"`
	Provider         string              `json:"provider"`
	Description      string              `json:"description"`
	Category         string              `json:"category"`
	IconURL          *string             `json:"icon_url,omitempty"`
	Examples         []string            `json:"examples"`
	Kind             string              `json:"kind"`
	Capabilities     []string            `json:"capabilities"`
	MinClientVersion string              `json:"min_client_version"`
	Platforms        []ManifestPlatform  `json:"platforms"`
	Entrypoints      ManifestEntrypoints `json:"entrypoints"`
}

type ManifestPlatform struct {
	OS            string   `json:"os"`
	Architectures []string `json:"architectures"`
}

type ManifestEntrypoints struct {
	MCP    string `json:"mcp,omitempty"`
	Skills string `json:"skills,omitempty"`
}

type MCPManifest struct {
	SchemaVersion int         `json:"schema_version"`
	Servers       []MCPServer `json:"servers"`
}

type MCPServer struct {
	ID            string            `json:"id"`
	Transport     string            `json:"transport"`
	URL           string            `json:"url"`
	StaticHeaders map[string]string `json:"static_headers,omitempty"`
	Auth          MCPAuth           `json:"auth"`
}

type MCPAuth struct {
	Type     string    `json:"type"`
	Provider string    `json:"provider,omitempty"`
	Scopes   *[]string `json:"scopes,omitempty"`
	Resource string    `json:"resource,omitempty"`
	ClientID string    `json:"client_id,omitempty"`
	Schema   string    `json:"schema,omitempty"`
}

type TokenSchema struct {
	SchemaVersion      int           `json:"schema_version"`
	DocumentationURL   string        `json:"documentation_url"`
	DocumentationLabel string        `json:"documentation_label"`
	Fields             []TokenField  `json:"fields"`
	Headers            []TokenHeader `json:"headers"`
}

type TokenField struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type TokenHeader struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

type Manifest struct {
	Connector ConnectorManifest
	MCP       *MCPManifest
	Token     *TokenSchema
}
