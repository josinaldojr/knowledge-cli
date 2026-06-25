package report

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"kv/internal/boundary"
	"kv/internal/fsutil"
	"kv/internal/session"
)

// GenerateSessionReport generates the report.md consolidating session execution info.
func GenerateSessionReport(workspaceDir string, sess *session.Session, qualityResults map[string]string, qualityPassed bool) error {
	// 1. Check boundary validator to see changed files
	boundaryReport, err := boundary.ValidateSession(workspaceDir, sess, true)
	boundaryStatus := "passed"
	if err != nil || (boundaryReport != nil && !boundaryReport.IsValid()) {
		boundaryStatus = "failed"
	}

	// 2. Read context.md token count estimation
	contextMDFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "context.md")
	contextSizeStr := "unknown"
	if fsutil.IsFile(contextMDFile) {
		data, err := ioutil.ReadFile(contextMDFile)
		if err == nil {
			tokens := len(data) / 4
			contextSizeStr = fmt.Sprintf("%.1fk tokens", float64(tokens)/1000.0)
		}
	}

	// 3. Read diff-summary.md if it exists
	diffSummaryFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "diff-summary.md")
	diffSummaryContent := ""
	if fsutil.IsFile(diffSummaryFile) {
		data, err := ioutil.ReadFile(diffSummaryFile)
		if err == nil {
			diffSummaryContent = string(data)
			diffSummaryContent = strings.TrimPrefix(diffSummaryContent, "# Diff Summary\n\n")
			diffSummaryContent = strings.TrimPrefix(diffSummaryContent, "# Diff Summary\n")
		}
	}

	var changedFiles []string
	if boundaryReport != nil {
		for _, f := range boundaryReport.Allowed {
			rel, _ := filepath.Rel(workspaceDir, f.Path)
			changedFiles = append(changedFiles, rel)
		}
		for _, f := range boundaryReport.Denied {
			rel, _ := filepath.Rel(workspaceDir, f.Path)
			changedFiles = append(changedFiles, rel+" (BLOCKED)")
		}
		for _, f := range boundaryReport.OutOfScope {
			rel, _ := filepath.Rel(workspaceDir, f.Path)
			changedFiles = append(changedFiles, rel+" (OUT OF SCOPE)")
		}
	}

	var sb strings.Builder
	sb.WriteString("# Session Report\n\n")

	sb.WriteString("## Status\n\n")
	status := "success"
	if boundaryStatus == "failed" || !qualityPassed {
		status = "failed"
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", status))

	sb.WriteString("## Goal\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", sess.Goal))

	sb.WriteString("## Context\n\n")
	sb.WriteString(fmt.Sprintf("- Project files included: %d\n", len(sess.Apps)))
	sb.WriteString(fmt.Sprintf("- Vault docs included: %d\n", len(sess.Vault.Sources)))
	sb.WriteString(fmt.Sprintf("- Estimated tokens: %s\n\n", contextSizeStr))

	sb.WriteString("## Boundary\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", boundaryStatus))

	sb.WriteString("## Changed Files\n\n")
	if len(changedFiles) == 0 {
		sb.WriteString("*(No changes detected)*\n\n")
	} else {
		for _, f := range changedFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Quality Gates\n\n")
	if len(sess.Quality.Commands) == 0 {
		sb.WriteString("*(No quality gates configured)*\n\n")
	} else {
		sb.WriteString("| Command | Status |\n")
		sb.WriteString("| --- | --- |\n")
		for cmd, res := range qualityResults {
			statusStr := "passed"
			if strings.Contains(res, "FAILED") || strings.Contains(res, "Blocked") {
				statusStr = "failed"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", cmd, statusStr))
		}
		sb.WriteString("\n")
	}

	if diffSummaryContent != "" {
		sb.WriteString(diffSummaryContent)
	}

	sb.WriteString("## Next Steps\n\n")
	if status == "success" {
		sb.WriteString("- Review the changes and merge the branch/pull request.\n")
		sb.WriteString("- Promote session learnings to the long-term vault.\n")
	} else {
		sb.WriteString("- Fix the quality gate failures or boundary violations before merging.\n")
		sb.WriteString("- Run `kv boundary validate` to audit changes.\n")
	}

	reportText := sb.String()

	reportFile := filepath.Join(workspaceDir, ".kv", "sessions", sess.ID, "report.md")
	if err := fsutil.EnsureDir(filepath.Dir(reportFile)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(reportFile, []byte(reportText), 0644); err != nil {
		return err
	}

	sess.Status = status
	_ = session.SaveSession(workspaceDir, sess)

	_ = session.LogEvent(workspaceDir, sess.ID, "report_generated", map[string]interface{}{
		"status": status,
	})

	return nil
}
