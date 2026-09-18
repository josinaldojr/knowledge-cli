package embed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"kv/internal/store"
)

// ModelConfig describes a sentence-transformers model exported to ONNX: the
// name of the sentence.transformers model (used for its cache subdirectory,
// download URLs for its ONNX graph and vocabulary, the graph's input/output
// tensor names, and how many tokens to encode per input. Defaults target
// sentence-transformers/all-MiniLM-L6-v2: small (~90MB), widely used, and
// exported by the model author under the "onnx" subfolder.
type ModelConfig struct {
	Name       string
	ModelURL   string
	VocabURL   string
	InputNames []string
	// OutputName is the ONNX graph output holding per-token embeddings
	// (shape [batch, seq_len, hidden_size]), pooled by MeanPool.
	OutputName string
	MaxTokens  int
	// SharedLibraryPath points onnxruntime_go at the onnxruntime native
	// shared library. Leave empty to use the platform default search
	// (onnxruntime.so/.dylib/.dll on the library search path).
	SharedLibraryPath string
}

// DefaultModelConfig returns the configuration used when kv's hybrid
// retrieval is asked to build a local embedder without an explicit
// override.
func DefaultModelConfig() ModelConfig {
	return ModelConfig{
		Name:       "all-MiniLM-L6-v2",
		ModelURL:   "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx",
		VocabURL:   "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt",
		InputNames: []string{"input_ids", "attention_mask", "token_type_ids"},
		OutputName: "last_hidden_state",
		MaxTokens:  256,
	}
}

// modelCacheDir returns the directory a model's files are cached under:
// <kv cache dir>/models/<name>, alongside kv's other rebuildable, non-vault
// artifacts (see docs/automatic-mcp-memory.md's storage table).
func modelCacheDir(name string) (string, error) {
	paths, err := store.ResolvePaths()
	if err != nil {
		return "", fmt.Errorf("resolve kv cache directory: %w", err)
	}
	return filepath.Join(paths.CacheDir, "models", name), nil
}

// EnsureModel makes cfg's ONNX model and vocabulary available on local disk,
// downloading them into the kv cache directory on first use and reusing
// that copy afterward. It never re-downloads a file that is already
// present and non-empty.
func EnsureModel(ctx context.Context, cfg ModelConfig) (modelPath, vocabPath string, err error) {
	return ensureModel(ctx, cfg, http.DefaultClient)
}

func ensureModel(ctx context.Context, cfg ModelConfig, client *http.Client) (modelPath, vocabPath string, err error) {
	dir, err := modelCacheDir(cfg.Name)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", fmt.Errorf("create model cache dir: %w", err)
	}

	modelPath = filepath.Join(dir, "model.onnx")
	vocabPath = filepath.Join(dir, "vocab.txt")

	if err := ensureCached(ctx, client, cfg.ModelURL, modelPath); err != nil {
		return "", "", fmt.Errorf("model: %w", err)
	}
	if err := ensureCached(ctx, client, cfg.VocabURL, vocabPath); err != nil {
		return "", "", fmt.Errorf("vocab: %w", err)
	}
	return modelPath, vocabPath, nil
}

// ensureCached downloads url to dest unless dest already exists and is
// non-empty. The download is written to a temporary file in the same
// directory and renamed into place, so a failed or interrupted download
// never leaves a corrupt file at dest.
func ensureCached(ctx context.Context, client *http.Client, url, dest string) error {
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".part-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dest)
}
