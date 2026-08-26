package discovery

import (
	"path/filepath"
	"testing"

	"kv/internal/store"
)

func TestAliasesPersistOutsideWorkspaceAndOverrideIdentity(t *testing.T) {
	paths := store.Paths{DataDir: t.TempDir()}
	want := WorkspaceAliases{Aliases: []WorkspaceAlias{{Remote: "https://github.com/example/repo.git", LogicalIdentity: "company/repo"}}}
	if err := SaveAliases(paths, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAliases(paths)
	if err != nil {
		t.Fatal(err)
	}
	identity := got.Apply(WorkspaceIdentity{LogicalID: "original", Git: GitMetadata{Remote: "github.com/example/repo", CanonicalCWD: filepath.Join(t.TempDir(), "repo")}})
	if identity.LogicalID != stableID("alias:company/repo") {
		t.Fatalf("logical ID = %q", identity.LogicalID)
	}
}
