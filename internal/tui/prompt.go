package tui

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"kv/internal/workspace"
)

// SelectModel is a reusable bubbletea model for picking an option from a list
type SelectModel struct {
	Title    string
	Choices  []string
	Cursor   int
	Selected string
	Quitted  bool
}

func (m SelectModel) Init() tea.Cmd {
	return nil
}

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.Quitted = true
			return m, tea.Quit
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Choices)-1 {
				m.Cursor++
			}
		case "enter":
			if len(m.Choices) > 0 {
				m.Selected = m.Choices[m.Cursor]
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SelectModel) View() string {
	if m.Quitted {
		return "Selection cancelled.\n"
	}
	var s strings.Builder
	s.WriteString(fmt.Sprintf("\n  \033[1;36m%s\033[0m\n\n", m.Title))
	for i, choice := range m.Choices {
		cursor := " "
		if m.Cursor == i {
			cursor = "\033[1;35m>\033[0m"
			s.WriteString(fmt.Sprintf(" %s \033[1;35m%s\033[0m\n", cursor, choice))
		} else {
			s.WriteString(fmt.Sprintf(" %s %s\n", cursor, choice))
		}
	}
	s.WriteString("\n  (Use Up/Down or j/k to navigate, Enter to select, Esc/q to cancel)\n")
	return s.String()
}

// PromptSelect displays a selection menu and returns the selected string
func PromptSelect(title string, choices []string) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no choices available")
	}
	p := tea.NewProgram(SelectModel{
		Title:   title,
		Choices: choices,
	})
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	sm := m.(SelectModel)
	if sm.Quitted || sm.Selected == "" {
		return "", fmt.Errorf("selection cancelled")
	}
	return sm.Selected, nil
}

// PromptWorkflow loads available workflows and prompts the user to choose one
func PromptWorkflow(workspaceDir string) (string, error) {
	workflowsDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows")
	files, err := ioutil.ReadDir(workflowsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read workflows directory: %w", err)
	}
	var slugs []string
	for _, f := range files {
		if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
			slugs = append(slugs, f.Name())
		}
	}
	if len(slugs) == 0 {
		return "", fmt.Errorf("no workflows found under %s", workflowsDir)
	}
	sort.Strings(slugs)
	return PromptSelect("Selecione o Workflow:", slugs)
}

// PromptTask loads available tasks for a workflow and prompts the user to choose one
func PromptTask(workspaceDir, workflowSlug string) (string, error) {
	tasksDir := filepath.Join(workspaceDir, workspace.ConfigDirName, "workflows", workflowSlug, "tasks")
	files, err := ioutil.ReadDir(tasksDir)
	if err != nil {
		return "", fmt.Errorf("failed to read tasks directory: %w", err)
	}
	var taskIDs []string
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".md" {
			taskIDs = append(taskIDs, strings.TrimSuffix(f.Name(), ".md"))
		}
	}
	if len(taskIDs) == 0 {
		return "", fmt.Errorf("no tasks found under %s", tasksDir)
	}
	sort.Strings(taskIDs)
	return PromptSelect(fmt.Sprintf("Selecione a Task para o workflow '%s':", workflowSlug), taskIDs)
}

// PromptModel prompts the user to select an OpenCode model from available models
func PromptModel() (string, error) {
	models := []string{
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

	// Try to execute opencode models to get the actual list
	cmd := exec.Command("opencode", "models")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		lines := strings.Split(stdout.String(), "\n")
		var realModels []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				realModels = append(realModels, trimmed)
			}
		}
		if len(realModels) > 0 {
			models = realModels
		}
	}

	return PromptSelect("Selecione o modelo do OpenCode para execução:", models)
}
