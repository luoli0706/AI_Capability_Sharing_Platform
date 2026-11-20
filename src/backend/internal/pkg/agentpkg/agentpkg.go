package agentpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentImportResult holds normalized metadata parsed from an Agent file or URL.
type AgentImportResult struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	ModelProvider  string   `json:"model_provider"`
	ModelName      string   `json:"model_name"`
	SystemPrompt   string   `json:"system_prompt"`
	Temperature    float64  `json:"temperature"`
	MaxTokens      int      `json:"max_tokens"`
	Tags           []string `json:"tags"`
	SourceURL      string   `json:"source_url"`
	DetectedFormat string   `json:"detected_format"`
}

func defaultResult() AgentImportResult {
	return AgentImportResult{
		Temperature: 0.70,
		MaxTokens:   2048,
		Tags:        []string{},
	}
}

// ParseAgentContent parses JSON, YAML/Dify DSL, or Markdown (with/without YAML frontmatter).
func ParseAgentContent(raw string, filename string) (AgentImportResult, error) {
	cleaned := strings.TrimPrefix(raw, "\xef\xbb\xbf")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	trimmed := strings.TrimSpace(cleaned)
	if trimmed == "" {
		return AgentImportResult{}, errors.New("empty agent content")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	var res AgentImportResult
	var err error

	switch {
	case strings.HasPrefix(trimmed, "{"):
		res, err = parseAgentJSON(trimmed)
	case ext == ".yaml" || ext == ".yml":
		res, err = parseAgentYAML(trimmed)
	case strings.HasPrefix(trimmed, "---\n") || trimmed == "---":
		res, err = parseAgentMarkdown(trimmed, filename)
	case ext == ".md" || ext == ".markdown" || ext == ".txt":
		res, err = parseAgentMarkdown(trimmed, filename)
	default:
		if yRes, yErr := parseAgentYAML(trimmed); yErr == nil && (yRes.SystemPrompt != "" || yRes.Name != "") {
			res = yRes
		} else {
			res, err = parseAgentMarkdown(trimmed, filename)
		}
	}
	if err != nil {
		return AgentImportResult{}, err
	}

	if res.Name == "" && filename != "" {
		base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		res.Name = strings.TrimSuffix(base, ".agent")
	}
	if res.Slug == "" {
		res.Slug = sanitizeSlug(res.Name)
	}
	if res.Temperature <= 0 {
		res.Temperature = 0.70
	}
	if res.MaxTokens <= 0 {
		res.MaxTokens = 2048
	}
	if res.Tags == nil {
		res.Tags = []string{}
	}
	return res, nil
}

func parseAgentJSON(raw string) (AgentImportResult, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return AgentImportResult{}, fmt.Errorf("invalid JSON: %w", err)
	}

	res := defaultResult()
	res.DetectedFormat = "json"
	if _, hasInstr := m["instructions"]; hasInstr {
		res.DetectedFormat = "openai-assistant-json"
	}

	res.Slug = firstString(m, "slug", "asset_code", "id")
	res.Name = firstString(m, "name", "title")
	res.Description = firstString(m, "description", "summary", "bio")
	res.ModelProvider = firstString(m, "model_provider", "provider")
	res.ModelName = firstString(m, "model_name", "model", "llm")
	res.SystemPrompt = firstString(m, "system_prompt", "instructions", "instruction", "pre_prompt", "prompt")

	if t := firstFloat(m, "temperature"); t > 0 {
		res.Temperature = t
	}
	if mt := firstInt(m, "max_tokens"); mt > 0 {
		res.MaxTokens = mt
	}
	res.Tags = extractStringSlice(m["tags"])
	return res, nil
}

func parseAgentYAML(raw string) (AgentImportResult, error) {
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		return AgentImportResult{}, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(m) == 0 {
		return AgentImportResult{}, errors.New("empty YAML mapping")
	}

	res := defaultResult()
	res.DetectedFormat = "yaml"

	// Dify DSL detection: app + model_config
	if appMap, ok := toStringMap(m["app"]); ok {
		res.DetectedFormat = "dify-dsl"
		res.Name = firstString(appMap, "name")
		res.Description = firstString(appMap, "description")
	}
	if mcMap, ok := toStringMap(m["model_config"]); ok {
		res.DetectedFormat = "dify-dsl"
		if p := firstString(mcMap, "pre_prompt", "system_prompt"); p != "" {
			res.SystemPrompt = p
		}
		if modelMap, ok := toStringMap(mcMap["model"]); ok {
			res.ModelProvider = firstString(modelMap, "provider")
			res.ModelName = firstString(modelMap, "name")
			if cp, ok := toStringMap(modelMap["completion_params"]); ok {
				if t := firstFloat(cp, "temperature"); t > 0 {
					res.Temperature = t
				}
				if mt := firstInt(cp, "max_tokens"); mt > 0 {
					res.MaxTokens = mt
				}
			}
		}
	}

	// Flat YAML fallbacks
	if res.Name == "" {
		res.Name = firstString(m, "name", "title")
	}
	if res.Description == "" {
		res.Description = firstString(m, "description", "summary")
	}
	if res.ModelProvider == "" {
		res.ModelProvider = firstString(m, "model_provider", "provider")
	}
	if res.ModelName == "" {
		res.ModelName = firstString(m, "model_name", "model")
	}
	if res.SystemPrompt == "" {
		res.SystemPrompt = firstString(m, "system_prompt", "instructions", "prompt")
	}
	if res.Slug == "" {
		res.Slug = firstString(m, "slug")
	}
	if t := firstFloat(m, "temperature"); t > 0 {
		res.Temperature = t
	}
	if mt := firstInt(m, "max_tokens"); mt > 0 {
		res.MaxTokens = mt
	}
	if tags := extractStringSlice(m["tags"]); len(tags) > 0 {
		res.Tags = tags
	}
	return res, nil
}

func parseAgentMarkdown(trimmed string, filename string) (AgentImportResult, error) {
	res := defaultResult()

	if strings.HasPrefix(trimmed, "---\n") {
		rest := strings.TrimPrefix(trimmed, "---\n")
		if endIdx := strings.Index(rest, "\n---"); endIdx >= 0 {
			fmRaw := rest[:endIdx]
			body := strings.TrimSpace(rest[endIdx+4:])
			var fm map[string]interface{}
			if err := yaml.Unmarshal([]byte(fmRaw), &fm); err == nil && len(fm) > 0 {
				res.DetectedFormat = "markdown-frontmatter"
				res.Slug = firstString(fm, "slug")
				res.Name = firstString(fm, "name", "title")
				res.Description = firstString(fm, "description", "summary")
				res.ModelProvider = firstString(fm, "model_provider", "provider")
				res.ModelName = firstString(fm, "model_name", "model")
				if t := firstFloat(fm, "temperature"); t > 0 {
					res.Temperature = t
				}
				if mt := firstInt(fm, "max_tokens"); mt > 0 {
					res.MaxTokens = mt
				}
				res.Tags = extractStringSlice(fm["tags"])
				res.SystemPrompt = body
				return res, nil
			}
		}
	}

	res.DetectedFormat = "markdown"
	res.SystemPrompt = trimmed
	for _, line := range strings.Split(trimmed, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "# ") {
			res.Name = strings.TrimSpace(strings.TrimPrefix(l, "# "))
			break
		}
	}
	if res.Name == "" && filename != "" {
		base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		res.Name = strings.TrimSuffix(base, ".agent")
	}
	return res, nil
}

// ImportAgentFromURL fetches and parses an Agent definition from GitHub, Agentverse, ModelScope, or raw URLs.
func ImportAgentFromURL(ctx context.Context, rawURL string) (AgentImportResult, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return AgentImportResult{}, errors.New("invalid URL")
	}

	host := strings.ToLower(u.Host)
	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Agentverse URL
	if strings.Contains(host, "agentverse.ai") {
		return importAgentverse(ctx, client, u, rawURL)
	}

	// 2. ModelScope URL
	if strings.Contains(host, "modelscope.cn") {
		return importModelScope(ctx, client, u, rawURL)
	}

	// 3. GitHub blob -> raw.githubusercontent.com
	fetchURL := rawURL
	if host == "github.com" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			fetchURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
				parts[0], parts[1], parts[3], strings.Join(parts[4:], "/"))
		}
	}

	body, err := httpGetText(ctx, client, fetchURL)
	if err != nil {
		return AgentImportResult{}, err
	}

	res, err := ParseAgentContent(body, filepath.Base(u.Path))
	if err != nil {
		return AgentImportResult{}, err
	}
	res.SourceURL = rawURL
	return res, nil
}

func importAgentverse(ctx context.Context, client *http.Client, u *url.URL, rawURL string) (AgentImportResult, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	var addr string
	for i, p := range parts {
		if p == "details" && i+1 < len(parts) {
			addr = parts[i+1]
			break
		}
	}
	if addr != "" {
		apiURL := "https://agentverse.ai/v1/agents/" + url.PathEscape(addr)
		if body, err := httpGetText(ctx, client, apiURL); err == nil {
			if res, pErr := parseAgentJSON(body); pErr == nil && res.Name != "" {
				res.SourceURL = rawURL
				res.DetectedFormat = "agentverse-api"
				if res.Slug == "" {
					res.Slug = sanitizeSlug(res.Name)
				}
				return res, nil
			}
		}
	}

	res := defaultResult()
	res.SourceURL = rawURL
	res.DetectedFormat = "agentverse-url"
	if htmlBody, err := httpGetText(ctx, client, rawURL); err == nil {
		res.Name, res.Description = extractHTMLMeta(htmlBody)
	}
	if res.Name == "" {
		res.Name = addr
	}
	res.Slug = sanitizeSlug(res.Name)
	return res, nil
}

func importModelScope(ctx context.Context, client *http.Client, u *url.URL, rawURL string) (AgentImportResult, error) {
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	slug := "modelscope-agent"
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		slug = parts[len(parts)-1]
	}

	res := defaultResult()
	res.Slug = sanitizeSlug(slug)
	res.Name = slug
	res.SourceURL = rawURL
	res.DetectedFormat = "modelscope-url"

	if htmlBody, err := httpGetText(ctx, client, rawURL); err == nil {
		title, desc := extractHTMLMeta(htmlBody)
		if title != "" {
			res.Name = title
		}
		if desc != "" {
			res.Description = desc
		}
	}
	return res, nil
}

func httpGetText(ctx context.Context, client *http.Client, targetURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ASCP-Platform/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, targetURL)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var (
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reMetaDesc = regexp.MustCompile(`(?is)<meta[^>]+name=["']description["'][^>]+content=["']([^"']*)["']`)
)

func extractHTMLMeta(html string) (title, desc string) {
	if m := reTitle.FindStringSubmatch(html); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	if m := reMetaDesc.FindStringSubmatch(html); len(m) > 1 {
		desc = strings.TrimSpace(m[1])
	}
	return title, desc
}

// BuildAgentMarkdown generates a clean .agent.md document with YAML frontmatter and system prompt.
func BuildAgentMarkdown(slug, name, description, provider, modelName, systemPrompt string, temp float64, maxTokens int) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("slug: %q\n", slug))
	sb.WriteString(fmt.Sprintf("name: %q\n", name))
	sb.WriteString(fmt.Sprintf("description: %q\n", description))
	if provider != "" {
		sb.WriteString(fmt.Sprintf("model_provider: %q\n", provider))
	}
	if modelName != "" {
		sb.WriteString(fmt.Sprintf("model_name: %q\n", modelName))
	}
	sb.WriteString(fmt.Sprintf("temperature: %.2f\n", temp))
	sb.WriteString(fmt.Sprintf("max_tokens: %d\n", maxTokens))
	sb.WriteString("---\n\n")
	if trimmed := strings.TrimSpace(systemPrompt); trimmed != "" {
		sb.WriteString(trimmed)
		sb.WriteString("\n")
	}
	return sb.String()
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func firstFloat(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			}
		}
	}
	return 0
}

func firstInt(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case int:
				return n
			case float64:
				return int(n)
			}
		}
	}
	return 0
}

func toStringMap(v interface{}) (map[string]interface{}, bool) {
	if m, ok := v.(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

func extractStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func sanitizeSlug(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var sb strings.Builder
	prevDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			sb.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		return "agent"
	}
	return out
}
