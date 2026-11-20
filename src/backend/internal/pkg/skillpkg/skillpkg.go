package skillpkg

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

const maxSkillFileBytes = 2 << 20

// ParseSkillMarkdown parses optional YAML frontmatter (--- ... ---) for name and description,
// and returns the trimmed markdown body.
func ParseSkillMarkdown(raw string) (name, description, body string) {
	cleaned := strings.TrimPrefix(raw, "\xef\xbb\xbf")
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	trimmed := strings.TrimSpace(cleaned)

	if !strings.HasPrefix(trimmed, "---\n") && trimmed != "---" {
		return "", "", trimmed
	}

	rest := strings.TrimPrefix(trimmed, "---\n")
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return "", "", trimmed
	}

	frontmatter := rest[:endIdx]
	after := rest[endIdx+4:]
	body = strings.TrimSpace(after)

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colonIdx := strings.IndexByte(line, ':')
		if colonIdx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		val := strings.TrimSpace(line[colonIdx+1:])
		val = unquoteYAML(val)
		switch key {
		case "name":
			name = val
		case "description":
			description = val
		}
	}
	return name, description, body
}

// BuildSkillMarkdown formats name, description, and markdown body into a SKILL.md document.
func BuildSkillMarkdown(name, description, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %q\n", name))
	sb.WriteString(fmt.Sprintf("description: %q\n", description))
	sb.WriteString("---\n\n")
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		sb.WriteString(trimmed)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ParseSkillZip inspects a zip archive, locates SKILL.md (at root or 1 level deep),
// parses its frontmatter/body, and returns the contained file paths.
func ParseSkillZip(data []byte) (name, description, body string, files []string, err error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", "", "", nil, fmt.Errorf("invalid zip archive: %w", err)
	}

	var skillFile *zip.File
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		cleanName := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasPrefix(cleanName, "__MACOSX/") || strings.HasPrefix(cleanName, "../") {
			continue
		}
		files = append(files, cleanName)

		parts := strings.Split(cleanName, "/")
		if len(parts) <= 2 && strings.EqualFold(parts[len(parts)-1], "SKILL.md") {
			if skillFile == nil || len(parts) == 1 {
				skillFile = f
			}
		}
	}

	if skillFile == nil {
		return "", "", "", files, errors.New("SKILL.md not found in zip archive")
	}

	rc, err := skillFile.Open()
	if err != nil {
		return "", "", "", files, err
	}
	defer func() { _ = rc.Close() }()

	rawBytes, err := io.ReadAll(io.LimitReader(rc, maxSkillFileBytes))
	if err != nil {
		return "", "", "", files, err
	}

	name, description, body = ParseSkillMarkdown(string(rawBytes))
	return name, description, body, files, nil
}

// BuildSkillZip creates an in-memory .zip archive containing <slug>/SKILL.md.
func BuildSkillZip(slug, name, description, body string) ([]byte, error) {
	safeSlug := strings.Trim(strings.ReplaceAll(slug, "/", "-"), " .")
	if safeSlug == "" {
		safeSlug = "skill"
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w, err := zw.Create(safeSlug + "/SKILL.md")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, BuildSkillMarkdown(name, description, body)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ResolveSkillsShURL converts skills.sh or raw markdown URLs into candidate raw SKILL.md URLs.
func ResolveSkillsShURL(rawURL string) ([]string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid URL")
	}

	host := strings.ToLower(u.Host)
	if host == "raw.githubusercontent.com" || strings.HasSuffix(strings.ToLower(u.Path), ".md") {
		return []string{u.String()}, nil
	}

	if host == "skills.sh" || host == "www.skills.sh" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, errors.New("skills.sh URL must follow https://skills.sh/<owner>/<repo>/<skill>")
		}
		owner, repo, skill := parts[0], parts[1], parts[2]
		return []string{
			fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/skills/%s/SKILL.md", owner, repo, skill),
			fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/skills/%s/SKILL.md", owner, repo, skill),
			fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s/SKILL.md", owner, repo, skill),
		}, nil
	}

	return nil, errors.New("unsupported skill URL; provide a skills.sh URL or a raw .md URL")
}

func unquoteYAML(val string) string {
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		return val[1 : len(val)-1]
	}
	return val
}
