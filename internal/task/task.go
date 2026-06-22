package task

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v3"
)

// TaskFrontmatter represents the YAML frontmatter schema of a task file.
type TaskFrontmatter struct {
	ID                 string   `yaml:"id"`
	Title              string   `yaml:"title"`
	Status             string   `yaml:"status"`
	Type               string   `yaml:"type"`
	Complexity         string   `yaml:"complexity"`
	Dependencies       []string `yaml:"dependencies"`
	AgentRunner        string   `yaml:"agent/runner"`
	Sources            []string `yaml:"sources"`
	AcceptanceCriteria []string `yaml:"acceptanceCriteria"`
}

// Task holds the parsed frontmatter and description content of a task.
type Task struct {
	Frontmatter TaskFrontmatter
	Content     string
}

// ParseTask parses a task markdown file with YAML frontmatter.
func ParseTask(filePath string) (*Task, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read task file: %v", err)
	}

	raw := string(data)
	return ParseTaskContent(raw)
}

// ParseTaskContent parses the raw string content of a task markdown file.
func ParseTaskContent(raw string) (*Task, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	if !strings.HasPrefix(normalized, "---\n") {
		return &Task{
			Content: raw,
		}, nil
	}

	parts := strings.SplitN(normalized, "---\n", 3)
	if len(parts) < 3 {
		return &Task{
			Content: raw,
		}, nil
	}

	var fm TaskFrontmatter
	err := yaml.Unmarshal([]byte(parts[1]), &fm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse task frontmatter YAML: %v", err)
	}

	return &Task{
		Frontmatter: fm,
		Content:     strings.TrimSpace(parts[2]),
	}, nil
}

// SaveTask writes the Task structure back to a markdown file.
func SaveTask(filePath string, task *Task) error {
	fmBytes, err := yaml.Marshal(&task.Frontmatter)
	if err != nil {
		return fmt.Errorf("failed to marshal task frontmatter: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(fmBytes)
	sb.WriteString("---\n\n")
	sb.WriteString(task.Content)
	sb.WriteString("\n")

	err = ioutil.WriteFile(filePath, []byte(sb.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write task file: %v", err)
	}

	return nil
}
