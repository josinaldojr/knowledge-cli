package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing TUI..."
	}

	// Adjust width constraints
	sidebarWidth := 32
	contentWidth := m.Width - sidebarWidth - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Styles
	accentColor := lipgloss.Color("#7D56F4")
	grayColor := lipgloss.Color("#8B8B97")
	borderColor := lipgloss.Color("#3B3B42")
	successColor := lipgloss.Color("#04B575")
	errorColor := lipgloss.Color("#FF4A4A")

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(accentColor).
		Padding(0, 2).
		MarginBottom(1)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(grayColor).
		MarginBottom(1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1)

	// --- 1. HEADER & STATUS ---
	headerText := "  KV AI DEVELOPMENT HARNESS  "
	statsText := fmt.Sprintf("Workspace: %s | Vault: %s | Sessions: %d", m.WorkspaceName, m.ActiveVault, len(m.ExistingSessions))
	
	// Create padded header bar
	barWidth := m.Width - 2
	if barWidth < 60 {
		barWidth = 60
	}
	paddingSize := barWidth - len(headerText) - len(statsText) - 2
	if paddingSize < 2 {
		paddingSize = 2
	}
	headerBar := headerStyle.Render(headerText) + strings.Repeat(" ", paddingSize) + lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Background(accentColor).Render(statsText)

	// --- 2. SIDEBAR (CATEGORIES & COMMANDS) ---
	var sb strings.Builder

	// Render Categories as horizontal tabs (just category list)
	sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("CATEGORIES") + "\n")
	for idx, cat := range m.Categories {
		isActiveCat := idx == m.ActiveCategory
		if isActiveCat {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(accentColor).Padding(0, 1).Render(cat) + "\n")
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Padding(0, 1).Render(cat) + "\n")
		}
	}
	sb.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("COMMANDS") + "\n")

	// Render Commands list in the active category
	for idx, cmd := range m.FilteredCmds {
		isSelected := idx == m.SelectedIndex
		isActivePane := m.ActivePane == PaneSidebar

		var item string
		if isSelected && isActivePane {
			item = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▶ " + cmd.DisplayName)
		} else if isSelected {
			item = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Render("○ " + cmd.DisplayName)
		} else {
			item = lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render("  " + cmd.DisplayName)
		}
		sb.WriteString(item + "\n")
	}

	sidebarRendered := borderStyle.
		Width(sidebarWidth).
		Height(m.Height - 6).
		Render(sb.String())

	// --- 3. CONTENT AREA ---
	var content strings.Builder
	activeCmd := m.FilteredCmds[m.SelectedIndex]

	switch m.ActivePane {
	case PaneSidebar:
		// Show details and parameters list of the selected command
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor).MarginBottom(1)
		content.WriteString(titleStyle.Render(activeCmd.DisplayName) + "\n")
		content.WriteString(subtitleStyle.Render(activeCmd.Description) + "\n\n")

		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3E3E6")).Render("Command Template:") + "\n")
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("  kv "+activeCmd.Name) + "\n\n")

		if len(activeCmd.Args) > 0 {
			content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3E3E6")).Render("Parameters:") + "\n")
			for _, arg := range activeCmd.Args {
				reqStr := ""
				if arg.Required {
					reqStr = lipgloss.NewStyle().Foreground(errorColor).Render(" (required)")
				}
				flagType := "Argument"
				if arg.IsFlag {
					flagType = "Flag"
				}
				content.WriteString(fmt.Sprintf("  * %s %s: %s%s\n", lipgloss.NewStyle().Foreground(accentColor).Render(arg.Key), flagType, arg.Description, reqStr))
			}
			content.WriteString("\n" + lipgloss.NewStyle().Italic(true).Foreground(accentColor).Render("Press [ENTER] to configure and run this command.") + "\n")
		} else {
			content.WriteString(lipgloss.NewStyle().Italic(true).Foreground(successColor).Render("This command does not require any parameters.") + "\n\n")
			content.WriteString(lipgloss.NewStyle().Italic(true).Foreground(accentColor).Render("Press [ENTER] to run immediately.") + "\n")
		}

	case PaneContent:
		if m.Form != nil {
			content.WriteString(m.Form.View(lipgloss.Style{}))
		}

	case PaneExecution:
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(accentColor).MarginBottom(1)
		content.WriteString(titleStyle.Render("Command Execution") + "\n")
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#8B8B97")).Render("Command: ")+lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render(m.ExecCmdLine) + "\n\n")

		if m.Executing {
			content.WriteString(fmt.Sprintf("  %s Running action, please wait...\n\n", m.Spinner.View()))
		} else {
			var statusBadge string
			if m.ExitCode == 0 && m.ExecErr == nil {
				statusBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(successColor).Padding(0, 2).Render("SUCCESS")
			} else {
				statusBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(errorColor).Padding(0, 2).Render(fmt.Sprintf("FAILED (Exit Code: %d)", m.ExitCode))
			}
			content.WriteString(statusBadge + "\n\n")

			// Execution Output Box
			out := strings.TrimSpace(m.ExecOutput)
			if out == "" {
				out = "<no output returned>"
			}

			lines := strings.Split(out, "\n")
			maxLines := m.Height - 15 - 4
			if maxLines < 1 {
				maxLines = 1
			}

			maxScroll := len(lines) - maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			scrollOffset := m.ExecScrollOffset
			if scrollOffset > maxScroll {
				scrollOffset = maxScroll
			}
			if scrollOffset < 0 {
				scrollOffset = 0
			}

			endIdx := scrollOffset + maxLines
			if endIdx > len(lines) {
				endIdx = len(lines)
			}

			scrollInfo := ""
			if len(lines) > maxLines {
				scrollInfo = fmt.Sprintf(" (Lines %d-%d of %d) [Use ↑/↓ or j/k to scroll]", scrollOffset+1, endIdx, len(lines))
			}
			content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E3E3E6")).Render("Console Output:"+scrollInfo) + "\n")

			visibleLines := lines[scrollOffset:endIdx]
			outSliced := strings.Join(visibleLines, "\n")

			outputStyle := lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(borderColor).
				Padding(1).
				Width(contentWidth - 4).
				MaxHeight(m.Height - 15).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#1B1B1E"))

			content.WriteString(outputStyle.Render(outSliced) + "\n\n")
			var footerText string
			if activeCmd.IsLongRunning {
				footerText = "Press [ENTER] or [ESC] to stop the server and return."
			} else {
				footerText = "Press [ENTER] or [ESC] to return to the commands list."
			}
			content.WriteString(lipgloss.NewStyle().Italic(true).Foreground(accentColor).Render(footerText) + "\n")
		}
	}

	contentRendered := borderStyle.
		Width(contentWidth).
		Height(m.Height - 6).
		Render(content.String())

	mainLayout := lipgloss.JoinHorizontal(lipgloss.Top, sidebarRendered, contentRendered)

	// --- 4. FOOTER ---
	var helpText string
	switch m.ActivePane {
	case PaneSidebar:
		helpText = "arrows: navigate commands • tab: edit form • enter: select command • q: quit"
	case PaneContent:
		helpText = "tab/shift+tab: navigate fields • arrows/space: edit options • enter: submit/execute • esc: cancel"
	case PaneExecution:
		if m.Executing {
			helpText = "Running command... Please wait."
		} else {
			if activeCmd.IsLongRunning {
				helpText = "enter/esc: stop server and go back • ctrl+c: stop server and quit"
			} else {
				helpText = "enter/esc: go back • ctrl+c: quit"
			}
		}
	}

	footer := lipgloss.NewStyle().
		Foreground(grayColor).
		Padding(0, 2).
		Render(helpText)

	return "\n" + headerBar + "\n\n" + mainLayout + "\n\n" + footer
}
