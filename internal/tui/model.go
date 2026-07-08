package tui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kv/internal/opencode"
	"kv/internal/vault"
	"kv/internal/workspace"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Pane int

const (
	PaneSidebar Pane = iota
	PaneContent
	PaneExecution
)

type Model struct {
	Commands        []Command
	FilteredCmds    []Command
	Categories      []string
	ActiveCategory  int
	SelectedIndex   int // index within filtered commands
	ActivePane      Pane

	Form            *Form
	Spinner         spinner.Model
	Executing       bool
	ExecCmdLine     string
	ExecOutput      string
	ExecScrollOffset int
	ExecErr         error
	ExitCode        int
	RunningCmd      *exec.Cmd

	WorkspaceDir    string
	WorkspaceName   string
	WorkspaceApps   []string          // List of App IDs
	WorkspaceAppPaths map[string]string // Maps App ID -> App Path
	ActiveVault     string
	ExistingSessions []string // List of Session IDs
	WorkspaceWorkflows []string
	WorkspaceTasks     []string
	OpencodeModels     []string

	Width           int
	Height          int
}

type ExecResultMsg struct {
	Stdout string
	Err    error
	Code   int
}

type ExecProgressMsg struct {
	Chunk string
	Chan  <-chan tea.Msg
}

type ExecFinishedMsg struct {
	Err  error
	Code int
}

func readProgress(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func NewModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	allCmds := GetCommands()

	// Gather distinct categories
	var categories []string
	seen := make(map[string]bool)
	for _, c := range allCmds {
		if !seen[c.Group] {
			seen[c.Group] = true
			categories = append(categories, c.Group)
		}
	}

	m := Model{
		Commands:       allCmds,
		Categories:     categories,
		ActiveCategory: 0,
		SelectedIndex:  0,
		ActivePane:     PaneSidebar,
		Spinner:        s,
	}

	m.reloadWorkspaceInfo()
	m.filterCommands()
	return m
}

func (m *Model) reloadWorkspaceInfo() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	m.WorkspaceApps = nil
	m.WorkspaceAppPaths = make(map[string]string)
	m.ExistingSessions = nil
	m.WorkspaceName = "None"
	m.WorkspaceDir = ""
	m.ActiveVault = "None"

	// Find workspace yaml directory
	wsDir, err := workspace.FindWorkspaceYamlDir(cwd)
	if err == nil {
		m.WorkspaceDir = wsDir
		wsYaml, err := workspace.LoadWorkspaceYaml(wsDir)
		if err == nil {
			m.WorkspaceName = wsYaml.Workspace.Name
			for _, app := range wsYaml.Workspace.Apps {
				m.WorkspaceApps = append(m.WorkspaceApps, app.ID)
				m.WorkspaceAppPaths[app.ID] = app.Path
			}
		}
	} else {
		// Fallback to legacy workspace marker dir
		wsDir, err = workspace.FindWorkspaceDir(cwd)
		if err == nil {
			m.WorkspaceDir = wsDir
			m.WorkspaceName = "Legacy Workspace"
		}
	}

	// Active Vault path
	vaultRes, err := vault.FindVault(cwd)
	if err == nil {
		m.ActiveVault = vaultRes.Path
	}

	// Existing Sessions list
	if m.WorkspaceDir != "" {
		sessionsDir := filepath.Join(m.WorkspaceDir, ".kv", "sessions")
		if infos, err := ioutil.ReadDir(sessionsDir); err == nil {
			var sessList []string
			for _, info := range infos {
				if info.IsDir() && strings.HasPrefix(info.Name(), "sess-") {
					sessList = append(sessList, info.Name())
				}
			}
			// Sort descending so newest is first
			sort.Slice(sessList, func(i, j int) bool {
				return sessList[i] > sessList[j]
			})
			m.ExistingSessions = sessList
		}
	}

	m.WorkspaceWorkflows = nil
	m.WorkspaceTasks = nil
	m.OpencodeModels = nil

	if m.WorkspaceDir != "" {
		workflowsDir := filepath.Join(m.WorkspaceDir, ".kv", "workflows")
		if infos, err := ioutil.ReadDir(workflowsDir); err == nil {
			var wfList []string
			var taskList []string
			for _, info := range infos {
				if info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
					wfSlug := info.Name()
					wfList = append(wfList, wfSlug)

					// Read tasks for this workflow
					tasksDir := filepath.Join(workflowsDir, wfSlug, "tasks")
					if taskInfos, err := ioutil.ReadDir(tasksDir); err == nil {
						for _, tInfo := range taskInfos {
							if !tInfo.IsDir() && filepath.Ext(tInfo.Name()) == ".md" {
								tID := strings.TrimSuffix(tInfo.Name(), ".md")
								taskList = append(taskList, tID)
							}
						}
					}
				}
			}
			sort.Strings(wfList)
			sort.Strings(taskList)
			m.WorkspaceWorkflows = wfList

			// Deduplicate task list
			var uniqueTasks []string
			seenTasks := make(map[string]bool)
			for _, t := range taskList {
				if !seenTasks[t] {
					seenTasks[t] = true
					uniqueTasks = append(uniqueTasks, t)
				}
			}
			m.WorkspaceTasks = uniqueTasks
		}
	}

	// Load OpenCode models
	if models, err := opencode.ListModels(); err == nil {
		m.OpencodeModels = models
	} else {
		m.OpencodeModels = []string{
			"opencode/big-pickle",
			"opencode/deepseek-v4-flash-free",
			"opencode/mimo-v2.5-free",
			"opencode/nemotron-3-ultra-free",
			"opencode/north-mini-code-free",
			"opencode-go/deepseek-v4-flash",
			"opencode-go/deepseek-v4-pro",
			"opencode-go/glm-5.1",
			"opencode-go/glm-5.2",
			"opencode-go/kimi-k2.6",
			"opencode-go/kimi-k2.7-code",
			"opencode-go/mimo-v2.5",
			"opencode-go/mimo-v2.5-pro",
			"opencode-go/minimax-m2.7",
			"opencode-go/minimax-m3",
			"opencode-go/qwen3.6-plus",
			"opencode-go/qwen3.7-max",
			"opencode-go/qwen3.7-plus",
			"openai/gpt-5.3-codex-spark",
			"openai/gpt-5.4",
			"openai/gpt-5.4-fast",
			"openai/gpt-5.4-mini",
			"openai/gpt-5.4-mini-fast",
			"openai/gpt-5.5",
			"openai/gpt-5.5-fast",
			"openai/gpt-5.5-pro",
		}
	}
}

func (m *Model) filterCommands() {
	cat := m.Categories[m.ActiveCategory]
	var filtered []Command
	for _, c := range m.Commands {
		if c.Group == cat {
			filtered = append(filtered, c)
		}
	}
	m.FilteredCmds = filtered
	if m.SelectedIndex >= len(filtered) {
		m.SelectedIndex = 0
	}
}

func (m *Model) Init() tea.Cmd {
	return m.Spinner.Tick
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if m.Form != nil {
			cfg := m.GetLayoutConfig()
			m.Form.UpdateWidth(cfg.ContentInnerWidth)
		}


	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case ExecResultMsg:
		m.Executing = false
		m.ExecOutput = msg.Stdout
		m.ExecErr = msg.Err
		m.ExitCode = msg.Code
		m.ActivePane = PaneExecution
		m.reloadWorkspaceInfo() // Reload state (e.g. if new app/session was added)
		return m, nil

	case ExecProgressMsg:
		m.ExecOutput += msg.Chunk
		// Auto-scroll to the bottom of the output
		lines := strings.Split(m.ExecOutput, "\n")
		m.ExecScrollOffset = len(lines)
		return m, readProgress(msg.Chan)

	case ExecFinishedMsg:
		m.Executing = false
		m.ExecErr = msg.Err
		m.ExitCode = msg.Code
		m.ActivePane = PaneExecution
		m.reloadWorkspaceInfo() // Reload state (e.g. if new app/session was added)
		return m, nil

	case FormSubmitMsg:
		if m.Form != nil {
			// Extract command values
			values := m.Form.GetValues()
			var args []string

			// Split command name (e.g., "session start" -> ["session", "start"])
			cmdParts := strings.Split(m.Form.Command.Name, " ")
			args = append(args, cmdParts...)

			// Append form values
			for _, argDef := range m.Form.Command.Args {
				val, ok := values[argDef.Key]
				if !ok || val == "" {
					continue
				}

				if argDef.IsFlag {
					if argDef.Type == InputTypeBoolean {
						if val == "true" {
							args = append(args, argDef.Key)
						}
					} else {
						if m.Form.Command.Name == "session init" && argDef.Key == "--apps" {
							var translated []string
							appsList := strings.Split(val, ",")
							for _, appID := range appsList {
								appID = strings.TrimSpace(appID)
								if path, exists := m.WorkspaceAppPaths[appID]; exists {
									translated = append(translated, fmt.Sprintf("%s=%s", appID, path))
								} else {
									translated = append(translated, appID)
								}
							}
							val = strings.Join(translated, ",")
						}
						args = append(args, argDef.Key, val)
					}
				} else {
					// Positional arguments
					// (Special case: comma-separated argument expansions if needed)
					args = append(args, val)
				}
			}

			m.Executing = true
			m.ExecOutput = ""
			m.ExecErr = nil
			m.ExitCode = 0
			m.ExecCmdLine = "kv " + strings.Join(args, " ")
			m.ExecScrollOffset = 0
			m.ActivePane = PaneExecution

			// Start command execution asynchronously
			binPath, err := os.Executable()
			if err != nil {
				// Fallback to calling built binary from PATH
				binPath = "kv"
			}

			if m.Form.Command.IsLongRunning {
				c := exec.Command(binPath, args...)
				cwd, _ := os.Getwd()
				c.Dir = cwd
				m.RunningCmd = c

				errStart := c.Start()
				if errStart != nil {
					m.Executing = false
					m.ExecOutput = "Failed to start process: " + errStart.Error()
					m.ExecErr = errStart
					m.ExitCode = 1
					return m, nil
				}

				exitChan := make(chan error, 1)
				go func() {
					exitChan <- c.Wait()
				}()

				execCmd := func() tea.Msg {
					select {
					case errWait := <-exitChan:
						code := 0
						if errWait != nil {
							code = 1
							if exitErr, ok := errWait.(*exec.ExitError); ok {
								code = exitErr.ExitCode()
							}
						}
						return ExecResultMsg{
							Stdout: "Process exited immediately.",
							Err:    errWait,
							Code:   code,
						}
					case <-time.After(800 * time.Millisecond):
						// Still running!
						return ExecResultMsg{
							Stdout: "Server started successfully and is running in the background.",
							Err:    nil,
							Code:   0,
						}
					}
				}
				return m, execCmd
			}

			// We need to execute the binary with the arguments and stream progress
			execCmd := func() tea.Msg {
				c := exec.Command(binPath, args...)
				cwd, _ := os.Getwd()
				c.Dir = cwd

				stdout, err := c.StdoutPipe()
				if err != nil {
					return ExecFinishedMsg{Err: err, Code: 1}
				}
				c.Stderr = c.Stdout // combine stderr and stdout

				progressChan := make(chan tea.Msg, 100)

				go func() {
					if err := c.Start(); err != nil {
						progressChan <- ExecFinishedMsg{Err: err, Code: 1}
						close(progressChan)
						return
					}

					buf := make([]byte, 2048)
					for {
						n, err := stdout.Read(buf)
						if n > 0 {
							progressChan <- ExecProgressMsg{
								Chunk: string(buf[:n]),
								Chan:  progressChan,
							}
						}
						if err != nil {
							break
						}
					}

					errWait := c.Wait()
					code := 0
					if errWait != nil {
						code = 1
						if exitErr, ok := errWait.(*exec.ExitError); ok {
							code = exitErr.ExitCode()
						}
					}
					progressChan <- ExecFinishedMsg{Err: errWait, Code: code}
					close(progressChan)
				}()

				// Return the first progress message or block until first chunk/finish
				return <-progressChan
			}

			return m, execCmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.RunningCmd != nil && m.RunningCmd.Process != nil {
				_ = m.RunningCmd.Process.Signal(os.Interrupt)
				time.Sleep(100 * time.Millisecond)
				_ = m.RunningCmd.Process.Kill()
				_ = m.RunningCmd.Wait()
			}
			return m, tea.Quit

		case "q":
			if m.ActivePane == PaneSidebar {
				return m, tea.Quit
			}
		}

		if m.ActivePane == PaneSidebar {
			switch msg.String() {
			case "up", "k":
				if m.SelectedIndex > 0 {
					m.SelectedIndex--
				}
			case "down", "j":
				if m.SelectedIndex < len(m.FilteredCmds)-1 {
					m.SelectedIndex++
				}
			case "left", "h":
				if m.ActiveCategory > 0 {
					m.ActiveCategory--
					m.SelectedIndex = 0
					m.filterCommands()
				}
			case "right", "l":
				if m.ActiveCategory < len(m.Categories)-1 {
					m.ActiveCategory++
					m.SelectedIndex = 0
					m.filterCommands()
				}
			case "enter":
				selected := m.FilteredCmds[m.SelectedIndex]
				if len(selected.Args) > 0 {
					m.ActivePane = PaneContent
					m.Form = NewForm(selected, m.WorkspaceApps, m.ExistingSessions, m.WorkspaceWorkflows, m.WorkspaceTasks, m.OpencodeModels)
					cfg := m.GetLayoutConfig()
					m.Form.UpdateWidth(cfg.ContentInnerWidth)
				} else {
					// No arguments, run directly!
					m.Form = &Form{Command: selected}
					return m, func() tea.Msg { return FormSubmitMsg{} }
				}
			}
		} else if m.ActivePane == PaneContent {
			if msg.String() == "esc" {
				m.ActivePane = PaneSidebar
				m.Form = nil
			} else if m.Form != nil {
				var formCmd tea.Cmd
				m.Form, formCmd = m.Form.Update(msg)
				cmds = append(cmds, formCmd)
			}
		} else if m.ActivePane == PaneExecution {
			// Press enter or esc to return to command list
			if msg.String() == "enter" || msg.String() == "esc" {
				if m.RunningCmd != nil && m.RunningCmd.Process != nil {
					_ = m.RunningCmd.Process.Signal(os.Interrupt)
					time.Sleep(100 * time.Millisecond)
					_ = m.RunningCmd.Process.Kill()
					_ = m.RunningCmd.Wait()
					m.RunningCmd = nil
				}
				m.ActivePane = PaneSidebar
				m.Form = nil
			} else {
				switch msg.String() {
				case "up", "k":
					if m.ExecScrollOffset > 0 {
						m.ExecScrollOffset--
					}
				case "down", "j":
					out := strings.TrimSpace(m.ExecOutput)
					lines := strings.Split(out, "\n")
					maxLines := m.Height - 15 - 4
					if maxLines < 1 {
						maxLines = 1
					}
					maxScroll := len(lines) - maxLines
					if maxScroll < 0 {
						maxScroll = 0
					}
					if m.ExecScrollOffset < maxScroll {
						m.ExecScrollOffset++
					}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// LayoutConfig holds the calculated dimensions for TUI panels.
type LayoutConfig struct {
	ShowSidebar        bool
	SidebarStacked     bool
	SidebarInnerWidth  int
	SidebarInnerHeight int
	ContentInnerWidth  int
	ContentInnerHeight int
	OuterHeight        int
}

func (m *Model) GetLayoutConfig() LayoutConfig {
	var overheadLines int
	var borderHeightOverhead int

	if m.Height < 28 {
		overheadLines = 4
		borderHeightOverhead = 2
	} else {
		overheadLines = 7
		borderHeightOverhead = 4
	}

	outerHeight := m.Height - overheadLines
	if outerHeight < 10 {
		outerHeight = 10
	}

	// 1. Determine if sidebar should be shown
	showSidebar := true
	if m.ActivePane != PaneSidebar {
		// Hide sidebar for forms and execution on normal screens
		if m.Width < 110 {
			showSidebar = false
		}
	}

	// 2. If sidebar is shown, check if it should be stacked vertically
	sidebarStacked := false
	if showSidebar {
		if m.ActivePane == PaneSidebar && m.Width < 100 {
			sidebarStacked = true
		} else if m.ActivePane != PaneSidebar && m.Width < 120 {
			// If we show sidebar in Content/Execution pane, stack it if width < 120
			sidebarStacked = true
		}
	}

	// 3. Compute dimensions
	cfg := LayoutConfig{
		ShowSidebar:    showSidebar,
		SidebarStacked: sidebarStacked,
		OuterHeight:    outerHeight,
	}

	if !showSidebar {
		cfg.SidebarInnerWidth = 0
		cfg.SidebarInnerHeight = 0
		cfg.ContentInnerWidth = m.Width - 4
		cfg.ContentInnerHeight = outerHeight - borderHeightOverhead
	} else if sidebarStacked {
		cfg.SidebarInnerWidth = m.Width - 4
		cfg.SidebarInnerHeight = (outerHeight / 2) - borderHeightOverhead
		if cfg.SidebarInnerHeight < 3 {
			cfg.SidebarInnerHeight = 3
		}

		cfg.ContentInnerWidth = m.Width - 4
		cfg.ContentInnerHeight = outerHeight - (cfg.SidebarInnerHeight + borderHeightOverhead) - borderHeightOverhead
		if cfg.ContentInnerHeight < 3 {
			cfg.ContentInnerHeight = 3
		}
	} else {
		// Horizontal split side-by-side
		sidebarOuterWidth := 32
		if sidebarOuterWidth > m.Width/3 {
			sidebarOuterWidth = m.Width / 3
		}
		if sidebarOuterWidth < 24 {
			sidebarOuterWidth = 24
		}

		cfg.SidebarInnerWidth = sidebarOuterWidth - 4
		cfg.ContentInnerWidth = m.Width - sidebarOuterWidth - 4
		cfg.SidebarInnerHeight = outerHeight - borderHeightOverhead
		cfg.ContentInnerHeight = outerHeight - borderHeightOverhead
	}

	// Sanity checks to prevent negative values
	if cfg.SidebarInnerWidth < 1 {
		cfg.SidebarInnerWidth = 1
	}
	if cfg.SidebarInnerHeight < 1 {
		cfg.SidebarInnerHeight = 1
	}
	if cfg.ContentInnerWidth < 1 {
		cfg.ContentInnerWidth = 1
	}
	if cfg.ContentInnerHeight < 1 {
		cfg.ContentInnerHeight = 1
	}

	return cfg
}

