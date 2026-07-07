package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Initializing TUI..."
	}

	cfg := m.GetLayoutConfig()

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

	var borderStyle lipgloss.Style
	var spacingHeader string
	var spacingFooter string
	var spacingTop string

	if m.Height < 28 {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)
		spacingTop = ""
		spacingHeader = "\n"
		spacingFooter = "\n"
	} else {
		borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1)
		spacingTop = "\n"
		spacingHeader = "\n\n"
		spacingFooter = "\n\n"
	}


	// --- 1. HEADER & STATUS ---
	headerText := "  KV AI DEVELOPMENT HARNESS  "
	var statsText string
	if m.Width >= 100 {
		statsText = fmt.Sprintf("Workspace: %s | Vault: %s | Sessions: %d", m.WorkspaceName, m.ActiveVault, len(m.ExistingSessions))
	} else if m.Width >= 70 {
		vaultName := filepath.Base(m.ActiveVault)
		statsText = fmt.Sprintf("WS: %s | Vault: %s | Sess: %d", m.WorkspaceName, vaultName, len(m.ExistingSessions))
	} else {
		statsText = fmt.Sprintf("WS: %s | Sess: %d", m.WorkspaceName, len(m.ExistingSessions))
	}
	
	// Create padded header bar
	barWidth := m.Width - 2
	if barWidth < 60 {
		barWidth = 60
	}
	combinedLen := len(headerText) + len(statsText)
	paddingSize := barWidth - combinedLen - 2
	if paddingSize < 2 {
		if barWidth - len(headerText) - 4 > 5 {
			statsText = statsText[:barWidth-len(headerText)-6] + "..."
			paddingSize = 2
		} else {
			statsText = ""
			paddingSize = barWidth - len(headerText) - 2
			if paddingSize < 2 {
				paddingSize = 2
			}
		}
	}
	headerBar := headerStyle.Render(headerText) + strings.Repeat(" ", paddingSize) + lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Background(accentColor).Render(statsText)

	// --- 2. SIDEBAR (CATEGORIES & COMMANDS) ---
	var sb strings.Builder
	if cfg.ShowSidebar {
		if cfg.SidebarStacked {
			// Render categories horizontally
			sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("CATEGORIES: "))
			var catRenders []string
			for idx, cat := range m.Categories {
				isActiveCat := idx == m.ActiveCategory
				if isActiveCat {
					catRenders = append(catRenders, lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(accentColor).Padding(0, 1).Render(cat))
				} else {
					catRenders = append(catRenders, lipgloss.NewStyle().Foreground(grayColor).Padding(0, 1).Render(cat))
				}
			}
			sb.WriteString(strings.Join(catRenders, " ") + "\n")

			// Render commands horizontally
			sb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("COMMANDS:   "))
			var cmdRenders []string
			for idx, cmd := range m.FilteredCmds {
				isSelected := idx == m.SelectedIndex
				isActivePane := m.ActivePane == PaneSidebar

				var item string
				if isSelected && isActivePane {
					item = lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("▶" + cmd.DisplayName)
				} else if isSelected {
					item = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Render("○" + cmd.DisplayName)
				} else {
					item = lipgloss.NewStyle().Foreground(lipgloss.Color("#E3E3E6")).Render(cmd.DisplayName)
				}
				cmdRenders = append(cmdRenders, item)
			}
			sb.WriteString(strings.Join(cmdRenders, "  ") + "\n")
		} else {
			// Render Categories as vertical list
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

			// Calculate available height for commands to avoid overflowing
			reservedLines := 3 + len(m.Categories)
			availableCmdLines := cfg.SidebarInnerHeight - reservedLines
			if availableCmdLines < 2 {
				availableCmdLines = 2
			}

			startIdx := 0
			endIdx := len(m.FilteredCmds)
			if endIdx > availableCmdLines {
				startIdx = m.SelectedIndex - (availableCmdLines / 2)
				if startIdx < 0 {
					startIdx = 0
				}
				endIdx = startIdx + availableCmdLines
				if endIdx > len(m.FilteredCmds) {
					endIdx = len(m.FilteredCmds)
					startIdx = endIdx - availableCmdLines
				}
			}

			if startIdx > 0 {
				sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("  ▲ ...") + "\n")
			}
			for idx := startIdx; idx < endIdx; idx++ {
				cmd := m.FilteredCmds[idx]
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
			if endIdx < len(m.FilteredCmds) {
				sb.WriteString(lipgloss.NewStyle().Foreground(grayColor).Render("  ▼ ...") + "\n")
			}
		}
	}

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
		}

		// Execution Output Box
		out := strings.TrimSpace(m.ExecOutput)
		if out == "" {
			if m.Executing {
				out = "Starting process..."
			} else {
				out = "<no output returned>"
			}
		}

		lines := strings.Split(out, "\n")
		maxLines := cfg.ContentInnerHeight - 8
		if maxLines < 3 {
			maxLines = 3
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
			Width(cfg.ContentInnerWidth - 4).
			MaxHeight(maxLines + 2).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#1B1B1E"))

		content.WriteString(outputStyle.Render(outSliced) + "\n\n")
		var footerText string
		if m.Executing {
			footerText = "Press [ENTER] or [ESC] to cancel execution and return."
		} else if activeCmd.IsLongRunning {
			footerText = "Press [ENTER] or [ESC] to stop the server and return."
		} else {
			footerText = "Press [ENTER] or [ESC] to return to the commands list."
		}
		content.WriteString(lipgloss.NewStyle().Italic(true).Foreground(accentColor).Render(footerText) + "\n")
	}

	var sidebarRendered string
	if cfg.ShowSidebar {
		sidebarRendered = borderStyle.
			Width(cfg.SidebarInnerWidth).
			Height(cfg.SidebarInnerHeight).
			Render(sb.String())
	}

	contentRendered := borderStyle.
		Width(cfg.ContentInnerWidth).
		Height(cfg.ContentInnerHeight).
		Render(content.String())

	var mainLayout string
	if cfg.ShowSidebar {
		if cfg.SidebarStacked {
			mainLayout = lipgloss.JoinVertical(lipgloss.Left, sidebarRendered, contentRendered)
		} else {
			mainLayout = lipgloss.JoinHorizontal(lipgloss.Top, sidebarRendered, contentRendered)
		}
	} else {
		mainLayout = contentRendered
	}

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

	return spacingTop + headerBar + spacingHeader + mainLayout + spacingFooter + footer
}
