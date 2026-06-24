package boundary

import (
	"path/filepath"
	"regexp"
	"strings"
)

// MatchPath checks if a path matches a pattern.
// Both patterns and paths should be cleaned/normalized.
func MatchPath(pattern, path string) (bool, error) {
	// Normalize separators and clean paths
	pat := filepath.ToSlash(filepath.Clean(pattern))
	pth := filepath.ToSlash(filepath.Clean(path))

	// If the pattern doesn't contain glob characters, perform a directory prefix check or exact match
	if !strings.ContainsAny(pat, "*?[]") {
		if pth == pat {
			return true, nil
		}
		// Match subdirectories (e.g. pattern "/a/b" matches "/a/b/c")
		if strings.HasPrefix(pth, pat+"/") {
			return true, nil
		}
		return false, nil
	}

	// Compile glob pattern to regex
	rx, err := globToRegex(pat)
	if err != nil {
		return false, err
	}

	return rx.MatchString(pth), nil
}

// globToRegex translates a wildcard pattern to a regular expression.
func globToRegex(glob string) (*regexp.Regexp, error) {
	var buf strings.Builder
	buf.WriteString("^")
	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			// Check if double star **
			if i+1 < len(glob) && glob[i+1] == '*' {
				i++ // skip next star
				// Check if there is a slash after ** or before ** to avoid matching partially
				if i+1 < len(glob) && glob[i+1] == '/' {
					i++
					buf.WriteString("(?:.*/)?")
				} else {
					buf.WriteString(".*")
				}
			} else {
				buf.WriteString("[^/]*")
			}
		case '?':
			buf.WriteString("[^/]")
		case '.', '+', '$', '^', '(', ')', '|', '{', '}', '\\':
			buf.WriteByte('\\')
			buf.WriteByte(c)
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteString("$")
	return regexp.Compile(buf.String())
}
