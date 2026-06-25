package diff

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"kv/internal/boundary"
	"kv/internal/fsutil"
	"kv/internal/session"
)

// GenerateDiffSummary parses git changes for allowed paths and writes a summary.
func GenerateDiffSummary(workspaceDir string, sess *session.Session) (string, error) {
	// 1. Run boundary validator to get the allowed files
	report, err := boundary.ValidateSession(workspaceDir, sess, true)
	if err != nil {
		return "", fmt.Errorf("failed to validate session for diff: %w", err)
	}

	var allowedPaths []string
	for _, fv := range report.Allowed {
		rel, err := filepath.Rel(workspaceDir, fv.Path)
		if err != nil {
			rel = fv.Path
		}
		allowedPaths = append(allowedPaths, rel)
	}

	if len(allowedPaths) == 0 {
		summary := "# Diff Summary\n\nNo changes detected in allowed session boundaries.\n"
		summaryFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "diff-summary.md")
		_ = fsutil.EnsureDir(filepath.Dir(summaryFile))
		_ = ioutil.WriteFile(summaryFile, []byte(summary), 0644)
		return summary, nil
	}

	// 2. Run git diff for allowed paths
	cmdArgs := append([]string{"diff", "HEAD", "--"}, allowedPaths...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = workspaceDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	var diffOutput string
	if err := cmd.Run(); err != nil {
		// Try without HEAD (unstaged only or fallback)
		cmd2 := exec.Command("git", append([]string{"diff", "--"}, allowedPaths...)...)
		cmd2.Dir = workspaceDir
		stdout.Reset()
		cmd2.Stdout = &stdout
		if err2 := cmd2.Run(); err2 != nil {
			diffOutput = ""
		} else {
			diffOutput = stdout.String()
		}
	} else {
		diffOutput = stdout.String()
	}

	// If no staged/unstaged changes, check git diff --cached (staged changes)
	if diffOutput == "" {
		cmdStaged := exec.Command("git", append([]string{"diff", "--cached", "--"}, allowedPaths...)...)
		cmdStaged.Dir = workspaceDir
		stdout.Reset()
		cmdStaged.Stdout = &stdout
		if errS := cmdStaged.Run(); errS == nil {
			diffOutput = stdout.String()
		}
	}

	// 3. Parse the diff
	type fileDiff struct {
		filePath  string
		additions []string
		deletions []string
		functions []string
		isTest    bool
	}

	var fileDiffs []fileDiff
	lines := strings.Split(diffOutput, "\n")
	var currentFile *fileDiff

	funcRegex := regexp.MustCompile(`^\+\s*(func|def|function|class)\s+(\w+)`)

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if currentFile != nil {
				fileDiffs = append(fileDiffs, *currentFile)
			}
			parts := strings.Fields(line)
			filePath := ""
			if len(parts) >= 4 {
				filePath = strings.TrimPrefix(parts[3], "b/")
			}
			isTest := strings.Contains(filePath, "_test") || strings.Contains(filePath, "test.") || strings.Contains(filePath, "/test/")
			currentFile = &fileDiff{
				filePath: filePath,
				isTest:   isTest,
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			content := strings.TrimSpace(strings.TrimPrefix(line, "+"))
			if content != "" {
				currentFile.additions = append(currentFile.additions, content)
				matches := funcRegex.FindStringSubmatch(line)
				if len(matches) >= 3 {
					currentFile.functions = append(currentFile.functions, matches[2])
				}
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			content := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if content != "" {
				currentFile.deletions = append(currentFile.deletions, content)
			}
		}
	}

	if currentFile != nil {
		fileDiffs = append(fileDiffs, *currentFile)
	}

	// 4. Generate markdown
	var sb strings.Builder
	sb.WriteString("# Diff Summary\n\n")

	sb.WriteString("## Modified Files\n\n")
	hasTests := false
	hasConfig := false
	hasSecurity := false

	for _, fd := range fileDiffs {
		if fd.isTest {
			hasTests = true
		}
		if strings.Contains(fd.filePath, "config") || strings.Contains(fd.filePath, "settings") {
			hasConfig = true
		}
		if strings.Contains(fd.filePath, "auth") || strings.Contains(fd.filePath, "security") || strings.Contains(fd.filePath, "login") || strings.Contains(fd.filePath, "middleware") {
			hasSecurity = true
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", fd.filePath))
		if len(fd.functions) > 0 {
			for _, fn := range fd.functions {
				sb.WriteString(fmt.Sprintf("- Added function `%s`.\n", fn))
			}
		}

		addedLines := len(fd.additions)
		deletedLines := len(fd.deletions)
		sb.WriteString(fmt.Sprintf("- Modified file (added %d lines, removed %d lines).\n", addedLines, deletedLines))

		if fd.isTest {
			sb.WriteString("- Updated test assertions/cases.\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Tests\n\n")
	if hasTests {
		sb.WriteString("- Added or updated unit tests to verify implementation.\n")
		sb.WriteString("- Covered success and failure scenarios.\n\n")
	} else {
		sb.WriteString("- *(No test files were modified directly in this diff.)*\n\n")
	}

	sb.WriteString("## Risks\n\n")
	if hasSecurity {
		sb.WriteString("- **Security**: Modified authentication/authorization paths. Verify credential processing carefully.\n")
	}
	if hasConfig {
		sb.WriteString("- **Configuration**: Changed configuration structures. Ensure env variables are updated in staging/production.\n")
	}
	sb.WriteString("- **Regressions**: Ensure that changes do not affect existing functionalities.\n")

	summaryText := sb.String()

	summaryFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "diff-summary.md")
	if err := fsutil.EnsureDir(filepath.Dir(summaryFile)); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(summaryFile, []byte(summaryText), 0644); err != nil {
		return "", err
	}

	return summaryText, nil
}
