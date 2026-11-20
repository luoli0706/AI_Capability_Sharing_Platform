package mcpregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const defaultRegistryBase = "https://registry.modelcontextprotocol.io/v0.1/servers"

// MCPImportItem is the normalized MCP server metadata returned by registry search or JSON parsing.
type MCPImportItem struct {
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Version        string `json:"version"`
	TransportType  string `json:"transport_type"`
	EndpointURL    string `json:"endpoint_url"`
	AuthType       string `json:"auth_type"`
	PackageCommand string `json:"package_command"`
	SourceURL      string `json:"source_url"`
}

type registryResponse struct {
	Servers []registryEntry `json:"servers"`
}

type registryEntry struct {
	Server serverSpec `json:"server"`
	Meta   struct {
		Official struct {
			IsLatest bool `json:"isLatest"`
		} `json:"io.modelcontextprotocol.registry/official"`
	} `json:"_meta"`
}

type serverSpec struct {
	Name        string          `json:"name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	Remotes     []remoteSpec    `json:"remotes"`
	Packages    []packageSpec   `json:"packages"`
}

type remoteSpec struct {
	Type    string       `json:"type"`
	URL     string       `json:"url"`
	Headers []headerSpec `json:"headers"`
}

type headerSpec struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type packageSpec struct {
	RegistryType string `json:"registryType"`
	Identifier   string `json:"identifier"`
}

// SearchServers queries the official MCP Registry and returns deduplicated, normalized items.
func SearchServers(ctx context.Context, query string, limit int) ([]MCPImportItem, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	endpoint := fmt.Sprintf("%s?search=%s&limit=%d", defaultRegistryBase, url.QueryEscape(strings.TrimSpace(query)), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ASCP-Platform/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp registry returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var parsed registryResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid registry response: %w", err)
	}

	return dedupAndMapEntries(parsed.Servers), nil
}

func dedupAndMapEntries(entries []registryEntry) []MCPImportItem {
	dedup := make(map[string]registryEntry)
	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		key := entry.Server.Name
		if key == "" {
			continue
		}
		if existing, ok := dedup[key]; !ok {
			order = append(order, key)
			dedup[key] = entry
		} else if entry.Meta.Official.IsLatest && !existing.Meta.Official.IsLatest {
			dedup[key] = entry
		}
	}

	items := make([]MCPImportItem, 0, len(order))
	for _, key := range order {
		items = append(items, mapServerSpec(dedup[key].Server))
	}
	return items
}

// ParseMCPConfigJSON parses mcpServers, VS Code servers, official MCP Registry responses/entries,
// flat server.json, or single flat client entries.
func ParseMCPConfigJSON(raw string) ([]MCPImportItem, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("empty JSON content")
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &generic); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// 1. Claude Desktop / Cursor: {"mcpServers": {...}}
	if rawServers, ok := generic["mcpServers"]; ok {
		return parseMCPServersMap(rawServers)
	}

	// 2 & 3. VS Code {"servers": {...}} or Official MCP Registry {"servers": [...]}
	if rawServers, ok := generic["servers"]; ok {
		sTrimmed := strings.TrimSpace(string(rawServers))
		if strings.HasPrefix(sTrimmed, "[") {
			var entries []registryEntry
			if err := json.Unmarshal(rawServers, &entries); err != nil {
				return nil, fmt.Errorf("invalid registry servers array: %w", err)
			}
			items := dedupAndMapEntries(entries)
			if len(items) == 0 {
				return nil, errors.New("no valid servers found in registry array")
			}
			return items, nil
		}
		return parseMCPServersMap(rawServers)
	}

	// 4. Official MCP Registry single wrapped entry: {"server": {...}, "_meta": {...}}
	if _, ok := generic["server"]; ok {
		var entry registryEntry
		if err := json.Unmarshal([]byte(trimmed), &entry); err != nil {
			return nil, fmt.Errorf("invalid registry server entry: %w", err)
		}
		if entry.Server.Name != "" || len(entry.Server.Remotes) > 0 || len(entry.Server.Packages) > 0 {
			return []MCPImportItem{mapServerSpec(entry.Server)}, nil
		}
	}

	// 5. Flat official server.json
	var spec serverSpec
	if err := json.Unmarshal([]byte(trimmed), &spec); err == nil {
		if spec.Name != "" || len(spec.Remotes) > 0 || len(spec.Packages) > 0 {
			return []MCPImportItem{mapServerSpec(spec)}, nil
		}
	}

	// 6. Single flat client entry: {"url": "...", "command": "...", "args": [...]}
	var flat mcpServerEntry
	if err := json.Unmarshal([]byte(trimmed), &flat); err == nil {
		if flat.URL != "" || flat.Command != "" {
			name := flat.Name
			if name == "" {
				name = "mcp-server"
			}
			return []MCPImportItem{mapClientServerEntry(name, flat)}, nil
		}
	}

	return nil, errors.New("unrecognized MCP JSON format")
}

type mcpServerEntry struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Headers map[string]string `json:"headers"`
}

func parseMCPServersMap(rawServers json.RawMessage) ([]MCPImportItem, error) {
	var servers map[string]mcpServerEntry
	if err := json.Unmarshal(rawServers, &servers); err != nil {
		return nil, fmt.Errorf("invalid servers object: %w", err)
	}
	if len(servers) == 0 {
		return nil, errors.New("servers map is empty")
	}

	keys := make([]string, 0, len(servers))
	for k := range servers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]MCPImportItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, mapClientServerEntry(k, servers[k]))
	}
	return items, nil
}

func mapClientServerEntry(key string, srv mcpServerEntry) MCPImportItem {
	slug := sanitizeSlug(key)
	item := MCPImportItem{
		Slug:     slug,
		Name:     key,
		AuthType: "none",
	}
	if srv.URL != "" {
		item.TransportType = normalizeTransport(srv.Type)
		item.EndpointURL = srv.URL
		item.AuthType = detectMapAuthType(srv.Headers)
	} else if srv.Command != "" {
		item.TransportType = "stdio"
		cmd := strings.TrimSpace(srv.Command + " " + strings.Join(srv.Args, " "))
		item.PackageCommand = cmd
		item.EndpointURL = "stdio://" + slug
	}
	return item
}

func mapServerSpec(spec serverSpec) MCPImportItem {
	name := spec.Title
	if name == "" {
		name = spec.Name
	}
	item := MCPImportItem{
		Slug:        sanitizeSlug(spec.Name),
		Name:        name,
		Description: spec.Description,
		Version:     spec.Version,
		AuthType:    "none",
	}
	if spec.Name != "" {
		item.SourceURL = defaultRegistryBase + "?search=" + url.QueryEscape(spec.Name)
	}

	if len(spec.Remotes) > 0 {
		r := spec.Remotes[0]
		item.TransportType = normalizeTransport(r.Type)
		item.EndpointURL = r.URL
		item.AuthType = detectHeaderSliceAuthType(r.Headers)
	} else if len(spec.Packages) > 0 {
		p := spec.Packages[0]
		item.TransportType = "stdio"
		item.EndpointURL = "stdio://" + p.Identifier
		switch strings.ToLower(p.RegistryType) {
		case "npm":
			item.PackageCommand = "npx -y " + p.Identifier
		case "pypi":
			item.PackageCommand = "uvx " + p.Identifier
		default:
			item.PackageCommand = p.Identifier
		}
	}
	return item
}

func normalizeTransport(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "sse":
		return "sse"
	case "stdio":
		return "stdio"
	default:
		return "streamable_http"
	}
}

func detectHeaderSliceAuthType(headers []headerSpec) string {
	if len(headers) == 0 {
		return "none"
	}
	for _, h := range headers {
		combined := strings.ToLower(h.Name + " " + h.Description)
		if strings.Contains(combined, "authorization") || strings.Contains(combined, "bearer") {
			return "bearer"
		}
	}
	return "header"
}

func detectMapAuthType(headers map[string]string) string {
	if len(headers) == 0 {
		return "none"
	}
	for k, v := range headers {
		combined := strings.ToLower(k + " " + v)
		if strings.Contains(combined, "authorization") || strings.Contains(combined, "bearer") {
			return "bearer"
		}
	}
	return "header"
}

func sanitizeSlug(raw string) string {
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}
	raw = strings.ToLower(raw)
	var sb strings.Builder
	prevDash := false
	for _, r := range raw {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			sb.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			sb.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		return "mcp-server"
	}
	return out
}
