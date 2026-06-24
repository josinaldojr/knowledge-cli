package boundary

import (
	"fmt"
	"path/filepath"
	"strings"

	"kv/internal/session"
)

// FileStatus represents the status of a file with respect to session boundary.
type FileStatus string

const (
	StatusAllowed    FileStatus = "allowed"
	StatusDenied     FileStatus = "denied"
	StatusOutOfScope FileStatus = "out_of_scope"
)

// FileValidation details the validation status of a single file.
type FileValidation struct {
	Path   string     `json:"path"`
	Status FileStatus `json:"status"`
}

// Report contains the summary of boundary validation.
type Report struct {
	SessionID  string           `json:"session_id"`
	Goal       string           `json:"goal"`
	Allowed    []FileValidation `json:"allowed"`
	Denied     []FileValidation `json:"denied"`
	OutOfScope []FileValidation `json:"out_of_scope"`
}

// IsValid returns true if there are no denied or out of scope files.
func (r *Report) IsValid() bool {
	return len(r.Denied) == 0 && len(r.OutOfScope) == 0
}

// ValidateSession changes checks git changes against the session's boundaries.
func ValidateSession(workspaceDir string, sess *session.Session, includeUntracked bool) (*Report, error) {
	// Resolve symlinks for workspaceDir
	evalWS, err := filepath.EvalSymlinks(workspaceDir)
	if err == nil {
		workspaceDir = evalWS
	}

	// 1. Get git changes
	_, changedFiles, err := GetGitChanges(workspaceDir, includeUntracked)
	if err != nil {
		return nil, fmt.Errorf("failed to read git changes: %w", err)
	}

	report := &Report{
		SessionID: sess.ID,
		Goal:      sess.Goal,
	}

	// 2. Resolve and normalize patterns
	allowedPatterns := make([]string, len(sess.Boundary.AllowedPaths))
	for i, p := range sess.Boundary.AllowedPaths {
		allowedPatterns[i] = ResolvePattern(workspaceDir, p)
	}

	deniedPatterns := make([]string, len(sess.Boundary.DeniedPaths))
	for i, p := range sess.Boundary.DeniedPaths {
		deniedPatterns[i] = ResolvePattern(workspaceDir, p)
	}

	// 3. Validate each changed file
	for _, file := range changedFiles {
		// Resolve symlinks on the file itself to be safe
		evalFile, err := filepath.EvalSymlinks(file)
		if err == nil {
			file = evalFile
		}

		// Filter out tool config/metadata directories (.kv/ or .git/)
		rel, err := filepath.Rel(workspaceDir, file)
		if err == nil {
			relNorm := filepath.ToSlash(rel)
			if strings.HasPrefix(relNorm, ".kv/") || relNorm == ".kv" || strings.HasPrefix(relNorm, ".git/") || relNorm == ".git" {
				continue
			}
		}

		status, err := validateFile(workspaceDir, file, allowedPatterns, deniedPatterns)
		if err != nil {
			return nil, fmt.Errorf("failed to validate file %s: %w", file, err)
		}

		fv := FileValidation{
			Path:   file,
			Status: status,
		}

		switch status {
		case StatusAllowed:
			report.Allowed = append(report.Allowed, fv)
		case StatusDenied:
			report.Denied = append(report.Denied, fv)
		case StatusOutOfScope:
			report.OutOfScope = append(report.OutOfScope, fv)
		}
	}

	return report, nil
}

func validateFile(workspaceDir, file string, allowed []string, denied []string) (FileStatus, error) {
	// Ensure file path is absolute and clean
	absFile := file
	if !filepath.IsAbs(absFile) {
		absFile = filepath.Clean(filepath.Join(workspaceDir, file))
	}

	// 1. Check denied paths first (highest priority)
	for _, pattern := range denied {
		matched, err := MatchPath(pattern, absFile)
		if err != nil {
			return "", err
		}
		if matched {
			return StatusDenied, nil
		}
	}

	// 2. Check allowed paths
	for _, pattern := range allowed {
		matched, err := MatchPath(pattern, absFile)
		if err != nil {
			return "", err
		}
		if matched {
			return StatusAllowed, nil
		}
	}

	// 3. Default to out of scope
	return StatusOutOfScope, nil
}

// ResolvePattern resolves a pattern relative to workspace directory if it is not absolute.
func ResolvePattern(workspaceDir, pattern string) string {
	var abs string
	if filepath.IsAbs(pattern) {
		abs = filepath.Clean(pattern)
	} else {
		abs = filepath.Clean(filepath.Join(workspaceDir, pattern))
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return eval
	}
	return abs
}

// PrintReport displays the report in a readable format on the terminal.
func PrintReport(report *Report, workspaceDir string) {
	fmt.Printf("Session ID: %s\n", report.SessionID)
	if report.Goal != "" {
		fmt.Printf("Goal:       %s\n", report.Goal)
	}
	fmt.Println()

	printFiles := func(title string, list []FileValidation) {
		if len(list) == 0 {
			return
		}
		fmt.Println(title)
		for _, f := range list {
			// Resolve relative path to workspace for display
			relPath := f.Path
			if filepath.IsAbs(relPath) {
				if r, err := filepath.Rel(workspaceDir, relPath); err == nil {
					relPath = r
				}
			}
			fmt.Printf("  - %s\n", filepath.Clean(relPath))
		}
		fmt.Println()
	}

	printFiles("Files inside scope (Allowed):", report.Allowed)
	printFiles("Files blocked by denied_paths (Denied):", report.Denied)
	printFiles("Files out of scope (OutOfScope):", report.OutOfScope)

	if len(report.Allowed) == 0 && len(report.Denied) == 0 && len(report.OutOfScope) == 0 {
		fmt.Println("No changed files detected.")
		fmt.Println()
	}

	if report.IsValid() {
		fmt.Println("Validation Result: SUCCESS")
	} else {
		fmt.Println("Validation Result: FAILED (staged/unstaged changes violate boundary constraints)")
	}
}
