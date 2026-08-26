package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kv/internal/store"
)

const aliasesFilename = "workspace-aliases.json"

type WorkspaceAlias struct {
	Remote          string `json:"remote,omitempty"`
	PathPrefix      string `json:"path_prefix,omitempty"`
	LogicalIdentity string `json:"logical_identity"`
}

type WorkspaceAliases struct {
	Aliases []WorkspaceAlias `json:"aliases"`
}

func LoadAliases(paths store.Paths) (WorkspaceAliases, error) {
	data, err := os.ReadFile(filepath.Join(paths.DataDir, aliasesFilename))
	if os.IsNotExist(err) {
		return WorkspaceAliases{}, nil
	}
	if err != nil {
		return WorkspaceAliases{}, fmt.Errorf("read workspace aliases: %w", err)
	}
	var aliases WorkspaceAliases
	if err := json.Unmarshal(data, &aliases); err != nil {
		return WorkspaceAliases{}, fmt.Errorf("decode workspace aliases: %w", err)
	}
	return aliases, nil
}

func SaveAliases(paths store.Paths, aliases WorkspaceAliases) error {
	if err := os.MkdirAll(paths.DataDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(aliases, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.DataDir, aliasesFilename), append(data, '\n'), 0600)
}

func (aliases WorkspaceAliases) Apply(identity WorkspaceIdentity) WorkspaceIdentity {
	for _, alias := range aliases.Aliases {
		if alias.LogicalIdentity == "" {
			continue
		}
		remoteMatches := alias.Remote != "" && NormalizeRemote(alias.Remote) == identity.Git.Remote
		pathMatches := alias.PathPrefix != "" && strings.HasPrefix(identity.Git.CanonicalCWD, filepath.Clean(alias.PathPrefix))
		if remoteMatches || pathMatches {
			identity.LogicalID = stableID("alias:" + alias.LogicalIdentity)
			return identity
		}
	}
	return identity
}
