package agentpkg

import (
	"context"
	"strings"
	"testing"
)

func TestParseAgentContent_PlainMarkdown(t *testing.T) {
	md := "# SQL Reviewer\n\nYou are an expert SQL reviewer. Check for missing indexes and N+1 queries."
	res, err := ParseAgentContent(md, "sql-reviewer.md")
	if err != nil {
		t.Fatalf("ParseAgentContent plain md failed: %v", err)
	}
	if res.Name != "SQL Reviewer" || res.DetectedFormat != "markdown" {
		t.Fatalf("unexpected name/format: %+v", res)
	}
	if !strings.Contains(res.SystemPrompt, "missing indexes") {
		t.Fatalf("unexpected system prompt: %q", res.SystemPrompt)
	}
}

func TestParseAgentContent_FrontmatterMarkdown(t *testing.T) {
	md := `---
name: "Code Architect"
description: "Designs modular Go services"
model_provider: "openai"
model: "gpt-4o"
temperature: 0.3
max_tokens: 4096
tags: ["go", "architecture"]
---

Always prefer small, single-purpose functions.`

	res, err := ParseAgentContent(md, "architect.agent.md")
	if err != nil {
		t.Fatalf("ParseAgentContent frontmatter md failed: %v", err)
	}
	if res.DetectedFormat != "markdown-frontmatter" || res.Name != "Code Architect" || res.ModelName != "gpt-4o" {
		t.Fatalf("unexpected frontmatter result: %+v", res)
	}
	if res.Temperature != 0.3 || res.MaxTokens != 4096 || len(res.Tags) != 2 {
		t.Fatalf("unexpected params/tags: %+v", res)
	}
	if res.SystemPrompt != "Always prefer small, single-purpose functions." {
		t.Fatalf("unexpected body: %q", res.SystemPrompt)
	}
}

func TestParseAgentContent_DifyYAML(t *testing.T) {
	dify := `app:
  name: "Customer Support Bot"
  description: "Handles billing inquiries"
model_config:
  pre_prompt: "You are a polite billing specialist."
  model:
    provider: "anthropic"
    name: "claude-3-5-sonnet"
    completion_params:
      temperature: 0.2
      max_tokens: 1024
`
	res, err := ParseAgentContent(dify, "support.yml")
	if err != nil {
		t.Fatalf("ParseAgentContent dify yaml failed: %v", err)
	}
	if res.DetectedFormat != "dify-dsl" || res.Name != "Customer Support Bot" {
		t.Fatalf("unexpected dify result: %+v", res)
	}
	if res.ModelProvider != "anthropic" || res.ModelName != "claude-3-5-sonnet" || res.Temperature != 0.2 || res.MaxTokens != 1024 {
		t.Fatalf("unexpected model config: %+v", res)
	}
}

func TestParseAgentContent_OpenAIJSON(t *testing.T) {
	raw := `{
  "name": "Data Analyst",
  "description": "Analyzes CSV datasets",
  "model": "gpt-4-turbo",
  "instructions": "Write clean Python pandas code to summarize the dataset.",
  "tags": ["data", "python"]
}`
	res, err := ParseAgentContent(raw, "analyst.json")
	if err != nil {
		t.Fatalf("ParseAgentContent json failed: %v", err)
	}
	if res.DetectedFormat != "openai-assistant-json" || res.Name != "Data Analyst" || res.ModelName != "gpt-4-turbo" {
		t.Fatalf("unexpected json result: %+v", res)
	}
	if !strings.Contains(res.SystemPrompt, "pandas") {
		t.Fatalf("unexpected instructions: %q", res.SystemPrompt)
	}
}

func TestBuildAgentMarkdown(t *testing.T) {
	out := BuildAgentMarkdown("qa-bot", "QA Bot", "Answers QA questions", "openai", "gpt-4o", "Be concise.", 0.5, 2048)
	parsed, err := ParseAgentContent(out, "qa-bot.agent.md")
	if err != nil {
		t.Fatalf("roundtrip ParseAgentContent failed: %v", err)
	}
	if parsed.Slug != "qa-bot" || parsed.Name != "QA Bot" || parsed.ModelProvider != "openai" || parsed.SystemPrompt != "Be concise." {
		t.Fatalf("unexpected roundtrip result: %+v", parsed)
	}
}

func TestIsDomainOrSubdomain(t *testing.T) {
	if !isDomainOrSubdomain("agentverse.ai", "agentverse.ai") {
		t.Fatal("exact domain should match")
	}
	if !isDomainOrSubdomain("sub.agentverse.ai", "agentverse.ai") {
		t.Fatal("subdomain should match")
	}
	if isDomainOrSubdomain("agentverse.ai.evil.com", "agentverse.ai") {
		t.Fatal("suffix bypass domain should NOT match")
	}
	if isDomainOrSubdomain("evilagentverse.ai", "agentverse.ai") {
		t.Fatal("prefix bypass domain should NOT match")
	}
}

func TestImportAgentFromURL_RejectsUnsafeSchemesAndLoopback(t *testing.T) {
	for _, bad := range []string{
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_INFO",
		"http://127.0.0.1:8080/agent.md",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if _, err := ImportAgentFromURL(context.Background(), bad); err == nil {
			t.Fatalf("expected ImportAgentFromURL to reject %q", bad)
		}
	}
}
