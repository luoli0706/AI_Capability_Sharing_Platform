package mcpregistry

import "testing"

func TestParseMCPConfigJSON_ServerJSONRemoteAndStdio(t *testing.T) {
	remoteJSON := `{
  "name": "ai.smithery/smithery-ai-github",
  "title": "GitHub MCP",
  "description": "GitHub integration server",
  "version": "1.2.0",
  "remotes": [
    {
      "type": "streamable-http",
      "url": "https://mcp.example.com/github",
      "headers": [{"name": "Authorization", "description": "Bearer token"}]
    }
  ]
}`
	items, err := ParseMCPConfigJSON(remoteJSON)
	if err != nil {
		t.Fatalf("ParseMCPConfigJSON remote failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Slug != "smithery-ai-github" || it.Name != "GitHub MCP" {
		t.Fatalf("unexpected slug/name: %+v", it)
	}
	if it.TransportType != "streamable_http" || it.AuthType != "bearer" || it.EndpointURL != "https://mcp.example.com/github" {
		t.Fatalf("unexpected transport/auth: %+v", it)
	}

	stdioJSON := `{
  "name": "io.github/fetch-server",
  "description": "Fetch web pages",
  "packages": [
    {"registryType": "pypi", "identifier": "mcp-server-fetch"}
  ]
}`
	items, err = ParseMCPConfigJSON(stdioJSON)
	if err != nil {
		t.Fatalf("ParseMCPConfigJSON stdio failed: %v", err)
	}
	if items[0].TransportType != "stdio" || items[0].EndpointURL != "stdio://mcp-server-fetch" || items[0].PackageCommand != "uvx mcp-server-fetch" {
		t.Fatalf("unexpected stdio mapping: %+v", items[0])
	}
}

func TestParseMCPConfigJSON_MCPServersMap(t *testing.T) {
	cfg := `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    },
    "remote-docs": {
      "type": "sse",
      "url": "https://docs.example.com/sse",
      "headers": {"X-API-Key": "secret"}
    }
  }
}`
	items, err := ParseMCPConfigJSON(cfg)
	if err != nil {
		t.Fatalf("ParseMCPConfigJSON mcpServers failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Slug != "filesystem" || items[0].TransportType != "stdio" || items[0].PackageCommand != "npx -y @modelcontextprotocol/server-filesystem /tmp" {
		t.Fatalf("unexpected filesystem item: %+v", items[0])
	}
	if items[1].Slug != "remote-docs" || items[1].TransportType != "sse" || items[1].AuthType != "header" {
		t.Fatalf("unexpected remote-docs item: %+v", items[1])
	}
}
