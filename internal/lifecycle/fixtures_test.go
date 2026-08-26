package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLifecycleIntegrationFixturesDeclareMemoryBoundaryAndGenealogy(t *testing.T) {
	for _, name := range []string{"discussion-no-artifact-change.json", "propose-to-archive-traceability.json"} {
		contents, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		var fixture map[string]any
		if err := json.Unmarshal(contents, &fixture); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if fixture["scenario"] == "" || fixture["expected"] == nil {
			t.Fatalf("invalid fixture %s: %#v", name, fixture)
		}
	}
}
