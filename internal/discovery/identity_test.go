package discovery

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDeriveIdentitySeparatesLogicalAndLocalIdentity(t *testing.T) {
	first := initRepository(t, "https://github.com/example/repo.git")
	second := initRepository(t, "git@github.com:example/repo.git")
	firstIdentity, err := DeriveIdentity(first)
	if err != nil {
		t.Fatal(err)
	}
	secondIdentity, err := DeriveIdentity(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstIdentity.LogicalID != secondIdentity.LogicalID {
		t.Fatal("clones of the same remote have different logical IDs")
	}
	if firstIdentity.InstanceID == secondIdentity.InstanceID {
		t.Fatal("separate clones share an instance ID")
	}
}

func TestDeriveIdentityForNonGitDirectory(t *testing.T) {
	identity, err := DeriveIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if identity.LogicalID == "" || identity.InstanceID == "" || identity.LogicalID != identity.InstanceID {
		t.Fatalf("identity = %#v", identity)
	}
}

func initRepository(t *testing.T, remote string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repository")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", remote}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
