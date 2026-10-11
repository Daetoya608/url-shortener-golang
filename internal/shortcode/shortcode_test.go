package shortcode

import (
	"strings"
	"testing"
)

func TestGenerateLengthAndAlphabet(t *testing.T) {
	for i := 0; i < 1000; i++ {
		c := Generate()
		if len(c) != Length {
			t.Fatalf("len(%q) = %d, want %d", c, len(c), Length)
		}
		for _, r := range c {
			if !strings.ContainsRune(Alphabet, r) {
				t.Fatalf("code %q contains non-base62 rune %q", c, r)
			}
		}
	}
}

func TestGenerateDiffers(t *testing.T) {
	a, b := Generate(), Generate()
	if a == b {
		t.Fatalf("two calls returned the same code %q", a)
	}
	seen := make(map[string]struct{})
	for i := 0; i < 10000; i++ {
		c := Generate()
		if _, ok := seen[c]; ok {
			t.Fatalf("duplicate code %q after %d calls", c, i)
		}
		seen[c] = struct{}{}
	}
}

func TestValidAlias(t *testing.T) {
	valid := []string{"abc", "my-link", "my_link", "ABC123", "a-_", strings.Repeat("x", 32)}
	for _, s := range valid {
		if !ValidAlias(s) {
			t.Errorf("ValidAlias(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "ab", strings.Repeat("x", 33), "has space", "slash/x", "dot.x", "кириллица", "q?x", "abc\n"}
	for _, s := range invalid {
		if ValidAlias(s) {
			t.Errorf("ValidAlias(%q) = true, want false", s)
		}
	}
}
