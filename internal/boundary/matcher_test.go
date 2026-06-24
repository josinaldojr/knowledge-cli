package boundary

import "testing"

func TestMatchPath(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected bool
	}{
		// Exact matches
		{"/a/b/c", "/a/b/c", true},
		{"/a/b/c", "/a/b/d", false},

		// Subpath matches (without wildcards)
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b/c/d", true},
		{"/a/b", "/a/bc", false},

		// Glob matches
		{"/a/b/*.go", "/a/b/main.go", true},
		{"/a/b/*.go", "/a/b/c/main.go", false},
		{"/a/b/**/*.go", "/a/b/c/main.go", true},
		{"/a/b/**/*.go", "/a/b/c/d/main.go", true},
		{"/a/b/**/test.go", "/a/b/test.go", true},
		{"/a/b/**/test.go", "/a/b/c/test.go", true},
	}

	for _, tc := range tests {
		matched, err := MatchPath(tc.pattern, tc.path)
		if err != nil {
			t.Errorf("MatchPath(%q, %q) returned error: %v", tc.pattern, tc.path, err)
			continue
		}
		if matched != tc.expected {
			t.Errorf("MatchPath(%q, %q) = %t; want %t", tc.pattern, tc.path, matched, tc.expected)
		}
	}
}
