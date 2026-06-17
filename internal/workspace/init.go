package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"kv/internal/fsutil"
)

// Init initializes the current workspace with a pointer to the given vault path.
func Init(vaultRawPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	// 1. Resolve absolute vault path
	absVault, err := fsutil.ResolveAbs(vaultRawPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path of vault: %v", err)
	}

	// 2. Validate that it contains the .kv-vault file
	markerPath := filepath.Join(absVault, ".kv-vault")
	if !fsutil.IsFile(markerPath) {
		return fmt.Errorf("the path '%s' is not a valid Knowledge Vault (missing '.kv-vault' file)", absVault)
	}

	// 3. Determine the path to write to .knowledge-vault (relative preferred)
	relPath, err := filepath.Rel(cwd, absVault)
	writePath := relPath
	if err != nil {
		// If relative path fails (e.g., crossing drive roots on Windows), use absolute path
		writePath = absVault
	}

	// Clean the write path to use standard system separator
	writePath = filepath.Clean(writePath)

	// 4. Write to .knowledge-vault in cwd with backup if already exists
	backedUp, err := WriteMarker(cwd, writePath)
	if err != nil {
		return fmt.Errorf("failed to write marker file: %v", err)
	}

	// 5. Display output
	fmt.Printf("Workspace initialized.\n\n")
	fmt.Printf("Workspace:       %s\n", cwd)
	fmt.Printf("Vault resolved:  %s\n", absVault)
	markerFile := filepath.Join(cwd, MarkerFilename)
	if backedUp {
		fmt.Printf("Marker created:  %s (backup created at %s%s)\n", markerFile, markerFile, BackupSuffix)
	} else {
		fmt.Printf("Marker created:  %s\n", markerFile)
	}
	fmt.Printf("Marker content:  %s\n", writePath)

	return nil
}
