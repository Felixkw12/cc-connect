package wpsagentspace

import (
	"testing"
	"unicode/utf8"
)

// TestTruncateKeepsValidUTF8 verifies that truncating a message that contains
// multi-byte characters never leaves a half character behind (Issue #2103).
func TestTruncateKeepsValidUTF8(t *testing.T) {
	const s = "wps 消息中文 abc 🙂"

	for maxLen := 0; maxLen <= utf8.RuneCountInString(s)+2; maxLen++ {
		got := truncate(s, maxLen)
		if !utf8.ValidString(got) {
			t.Errorf("maxLen=%d: result is not valid UTF-8: %q", maxLen, got)
		}
	}
}

// TestTruncateCutsOnCharacterBoundary verifies the cut keeps whole characters
// and counts characters, not bytes.
func TestTruncateCutsOnCharacterBoundary(t *testing.T) {
	got := truncate("中文abc", 2)
	if want := "中文..."; got != want {
		t.Fatalf("truncate = %q, want %q", got, want)
	}
}

// TestTruncateKeepsShortString verifies a string that already fits is
// returned unchanged.
func TestTruncateKeepsShortString(t *testing.T) {
	const s = "短"
	if got := truncate(s, 8); got != s {
		t.Fatalf("truncate = %q, want %q", got, s)
	}
}
