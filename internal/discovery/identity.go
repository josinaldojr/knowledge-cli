package discovery

import (
	"crypto/sha256"
	"encoding/hex"
)

type WorkspaceIdentity struct {
	LogicalID  string
	InstanceID string
	Git        GitMetadata
}

// DeriveIdentity keeps clones of the same remote in one logical workspace and
// distinguishes their local checkout/worktree instances.
func DeriveIdentity(cwd string) (WorkspaceIdentity, error) {
	git, err := DiscoverGit(cwd)
	if err != nil {
		return WorkspaceIdentity{}, err
	}
	logicalInput := "path:" + git.CanonicalCWD
	instanceInput := "path:" + git.CanonicalCWD
	if git.IsGit {
		if git.Remote != "" {
			logicalInput = "git:" + git.Remote
		} else {
			logicalInput = "path:" + git.Root
		}
		instanceInput = "git:" + git.Root + "|common:" + git.CommonDir
	}
	return WorkspaceIdentity{LogicalID: stableID(logicalInput), InstanceID: stableID(instanceInput), Git: git}, nil
}

func stableID(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
