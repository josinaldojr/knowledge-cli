package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectGarbageRemovesExpiredContentButPreservesProtectedHashes(t *testing.T) {
	store := mustSnapshotStore(t, t.TempDir())
	manifest := Manifest{ContentHash: "expired", CanonicalPath: "/workspace/proposal.md"}
	if _, err := store.Put(manifest, []byte("expired")); err != nil {
		t.Fatal(err)
	}
	protected := Manifest{ContentHash: "protected", CanonicalPath: "/workspace/design.md"}
	if _, err := store.Put(protected, []byte("protected")); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"expired", "protected"} {
		path := filepath.Join(store.root, "contents", hash)
		old := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := store.CollectGarbage(time.Now(), RetentionPolicy{ContentMaxAge: time.Hour, ProtectedHashes: map[string]struct{}{"protected": {}}})
	if err != nil || removed != 1 {
		t.Fatalf("garbage collection = %d, %v", removed, err)
	}
	if _, err := store.Get("expired"); !os.IsNotExist(err) {
		t.Fatalf("expired content still available: %v", err)
	}
	if _, err := store.Get("protected"); err != nil {
		t.Fatalf("protected content removed: %v", err)
	}
}

func TestStoreDeduplicatesAndUsesPrivateContent(t *testing.T) {
	store, err := Open(t.TempDir(), 32, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{ContentHash: "abc"}
	first, err := store.Put(manifest, []byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Put(manifest, []byte("content"))
	if err != nil || first != second {
		t.Fatalf("paths = %q, %q, %v", first, second, err)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 || filepath.Base(first) != "abc" {
		t.Fatalf("mode/path = %v, %s", info.Mode(), first)
	}
	contents, err := store.Get("abc")
	if err != nil || string(contents) != "content" {
		t.Fatalf("contents = %q, err = %v", contents, err)
	}
	if _, err := store.Get("../abc"); err == nil {
		t.Fatal("traversal hash accepted")
	}
}

func TestStoreRejectsOversizeAndRedactedContent(t *testing.T) {
	store, err := Open(t.TempDir(), 2, func(contents []byte) bool { return string(contents) == "secret" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(Manifest{ContentHash: "large"}, []byte("large")); err == nil {
		t.Fatal("oversized content accepted")
	}
	store, err = Open(t.TempDir(), 10, func(contents []byte) bool { return string(contents) == "secret" })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(Manifest{ContentHash: "secret"}, []byte("secret")); err == nil {
		t.Fatal("redacted content accepted")
	}
}
