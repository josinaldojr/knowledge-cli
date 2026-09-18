package embed

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestNewONNXEmbedderDegradesGracefullyWhenModelUnreachable exercises the
// real degradation path this package is built around: when the model
// cannot be fetched (here, because this sandbox's network policy blocks
// huggingface.co — see the session notes on egress restrictions),
// NewONNXEmbedder must fail fast with ErrUnavailable rather than leave a
// caller with a broken embedder. This is the condition wiki.HybridSearchIndex
// checks for to fall back to lexical-only ranking.
func TestNewONNXEmbedderDegradesGracefullyWhenModelUnreachable(t *testing.T) {
	withIsolatedCacheDir(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := NewONNXEmbedder(ctx, DefaultModelConfig())
	if err == nil {
		t.Fatal("expected an error when the model cannot be downloaded in this environment, got nil")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected error to wrap ErrUnavailable, got: %v", err)
	}
}
