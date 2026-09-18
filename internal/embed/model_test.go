package embed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"kv/internal/store"
)

// withIsolatedCacheDir points KV_CACHE_HOME at a throwaway directory for the
// duration of a test, so EnsureModel's caching logic can be exercised
// without touching (or depending on) the real user cache directory.
func withIsolatedCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KV_CACHE_HOME", dir)
	t.Setenv("KV_DATA_HOME", filepath.Join(dir, "data"))
	return dir
}

func testServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestEnsureModelDownloadsIntoCache(t *testing.T) {
	withIsolatedCacheDir(t)
	modelSrv := testServer(t, "fake-onnx-bytes")
	vocabSrv := testServer(t, "[PAD]\n[UNK]\n")

	cfg := ModelConfig{Name: "test-model", ModelURL: modelSrv.URL, VocabURL: vocabSrv.URL}
	modelPath, vocabPath, err := ensureModel(context.Background(), cfg, http.DefaultClient)
	if err != nil {
		t.Fatalf("ensureModel failed: %v", err)
	}

	modelData, err := os.ReadFile(modelPath)
	if err != nil || string(modelData) != "fake-onnx-bytes" {
		t.Errorf("expected cached model file with server content, got %q (err=%v)", modelData, err)
	}
	vocabData, err := os.ReadFile(vocabPath)
	if err != nil || string(vocabData) != "[PAD]\n[UNK]\n" {
		t.Errorf("expected cached vocab file with server content, got %q (err=%v)", vocabData, err)
	}

	dir, err := modelCacheDir("test-model")
	if err != nil {
		t.Fatalf("modelCacheDir failed: %v", err)
	}
	paths, _ := store.ResolvePaths()
	if filepath.Dir(filepath.Dir(dir)) != paths.CacheDir {
		t.Errorf("expected model cache dir under kv cache dir %q, got %q", paths.CacheDir, dir)
	}
}

func TestEnsureModelSkipsDownloadWhenAlreadyCached(t *testing.T) {
	withIsolatedCacheDir(t)
	dir, err := modelCacheDir("cached-model")
	if err != nil {
		t.Fatalf("modelCacheDir failed: %v", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("failed to prepare cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.onnx"), []byte("already-here"), 0644); err != nil {
		t.Fatalf("failed to seed cached model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vocab.txt"), []byte("[PAD]\n"), 0644); err != nil {
		t.Fatalf("failed to seed cached vocab: %v", err)
	}

	var served bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		_, _ = w.Write([]byte("should not be fetched"))
	}))
	t.Cleanup(srv.Close)

	cfg := ModelConfig{Name: "cached-model", ModelURL: srv.URL, VocabURL: srv.URL}
	modelPath, _, err := ensureModel(context.Background(), cfg, http.DefaultClient)
	if err != nil {
		t.Fatalf("ensureModel failed: %v", err)
	}
	if served {
		t.Errorf("expected no download when files are already cached, but the server was hit")
	}
	data, _ := os.ReadFile(modelPath)
	if string(data) != "already-here" {
		t.Errorf("expected the pre-existing cached file to be left untouched, got %q", data)
	}
}

func TestEnsureModelPropagatesDownloadFailure(t *testing.T) {
	withIsolatedCacheDir(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	cfg := ModelConfig{Name: "blocked-model", ModelURL: srv.URL, VocabURL: srv.URL}
	if _, _, err := ensureModel(context.Background(), cfg, http.DefaultClient); err == nil {
		t.Error("expected an error when the download is denied, got nil")
	}
}
