package chunker

import (
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	if chunks := SplitText("   \r\n  ", 100, 10); len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for blank text, got %d", len(chunks))
	}

	short := "Hello ASCP knowledge base"
	chunks := SplitText(short, 100, 20)
	if len(chunks) != 1 || chunks[0].Content != short || chunks[0].Index != 0 {
		t.Fatalf("unexpected short chunks: %+v", chunks)
	}

	// 25 runes with chunkSize=10, overlap=2 -> step=8 -> [0:10], [8:18], [16:25]
	long := strings.Repeat("a", 10) + strings.Repeat("b", 8) + strings.Repeat("c", 7)
	chunks = SplitText(long, 10, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Index != 0 || chunks[1].Index != 1 || chunks[2].Index != 2 {
		t.Fatalf("unexpected chunk indices: %+v", chunks)
	}
	// Verify overlap between chunk 0 and chunk 1
	if chunks[0].Content[8:] != chunks[1].Content[:2] {
		t.Fatalf("expected overlap of 2 runes, got %q and %q", chunks[0].Content, chunks[1].Content)
	}
}
