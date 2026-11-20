package chunker

import (
	"strings"
	"unicode/utf8"
)

type Chunk struct {
	Index      int
	Content    string
	TokenCount int
}

// SplitText splits text into overlapping chunks using a rune-based sliding window.
func SplitText(text string, chunkSize, chunkOverlap int) []Chunk {
	normalized := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if normalized == "" {
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 5
	}

	runes := []rune(normalized)
	total := len(runes)
	if total <= chunkSize {
		return []Chunk{{
			Index:      0,
			Content:    normalized,
			TokenCount: estimateTokens(normalized),
		}}
	}

	step := chunkSize - chunkOverlap
	if step <= 0 {
		step = 1
	}

	var chunks []Chunk
	idx := 0
	for start := 0; start < total; start += step {
		end := start + chunkSize
		if end > total {
			end = total
		}
		piece := strings.TrimSpace(string(runes[start:end]))
		if piece != "" {
			chunks = append(chunks, Chunk{
				Index:      idx,
				Content:    piece,
				TokenCount: estimateTokens(piece),
			})
			idx++
		}
		if end == total {
			break
		}
	}
	return chunks
}

func estimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return n
}
