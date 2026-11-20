package skillpkg

import (
	"strings"
	"testing"
)

func TestParseSkillMarkdown(t *testing.T) {
	raw := `---
name: "git-commit-helper"
description: 'Generates conventional commit messages'
---

# Git Commit Helper

Use this skill to format commits.`

	name, desc, body := ParseSkillMarkdown(raw)
	if name != "git-commit-helper" {
		t.Fatalf("expected name 'git-commit-helper', got %q", name)
	}
	if desc != "Generates conventional commit messages" {
		t.Fatalf("expected description, got %q", desc)
	}
	if !strings.HasPrefix(body, "# Git Commit Helper") {
		t.Fatalf("unexpected body: %q", body)
	}

	n2, d2, b2 := ParseSkillMarkdown("Plain markdown without frontmatter")
	if n2 != "" || d2 != "" || b2 != "Plain markdown without frontmatter" {
		t.Fatalf("unexpected plain parse: %q %q %q", n2, d2, b2)
	}
}

func TestBuildAndParseSkillZipRoundtrip(t *testing.T) {
	zipBytes, err := BuildSkillZip("code-reviewer", "Code Reviewer", "Reviews PR diffs", "## Instructions\nCheck edge cases.")
	if err != nil {
		t.Fatalf("BuildSkillZip failed: %v", err)
	}

	name, desc, body, files, err := ParseSkillZip(zipBytes)
	if err != nil {
		t.Fatalf("ParseSkillZip failed: %v", err)
	}
	if name != "Code Reviewer" || desc != "Reviews PR diffs" {
		t.Fatalf("unexpected metadata: name=%q desc=%q", name, desc)
	}
	if !strings.Contains(body, "Check edge cases.") {
		t.Fatalf("unexpected body: %q", body)
	}
	if len(files) != 1 || files[0] != "code-reviewer/SKILL.md" {
		t.Fatalf("unexpected files: %+v", files)
	}
}

func TestResolveSkillsShURL(t *testing.T) {
	urls, err := ResolveSkillsShURL("https://www.skills.sh/anthropics/skills/pdf")
	if err != nil {
		t.Fatalf("ResolveSkillsShURL failed: %v", err)
	}
	if len(urls) != 3 {
		t.Fatalf("expected 3 candidate URLs, got %d", len(urls))
	}
	if urls[0] != "https://raw.githubusercontent.com/anthropics/skills/main/skills/pdf/SKILL.md" {
		t.Fatalf("unexpected first candidate: %s", urls[0])
	}

	raw, err := ResolveSkillsShURL("https://raw.githubusercontent.com/org/repo/main/SKILL.md")
	if err != nil || len(raw) != 1 {
		t.Fatalf("expected direct raw URL pass-through, got %+v, err=%v", raw, err)
	}
}
