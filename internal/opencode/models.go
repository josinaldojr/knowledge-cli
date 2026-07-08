package opencode

import (
	"bytes"
	"os/exec"
	"strings"
)

// ListModels returns all available models using `opencode models`
func ListModels() ([]string, error) {
	cmd := exec.Command("opencode", "models")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		// Fallback list of models in case opencode is not installed or fails
		return []string{
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
		}, nil
	}
	lines := strings.Split(stdout.String(), "\n")
	var models []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			models = append(models, trimmed)
		}
	}
	if len(models) == 0 {
		return []string{"opencode/deepseek-v4-flash-free"}, nil
	}
	return models, nil
}
