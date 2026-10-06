package defaultresources

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// Recorded after comparing every expanded directory with the former embedded
// ZIPs. A layout-only migration preserves content at the original version.
func TestDirectoryMigrationPreservesVersionedIdentities(t *testing.T) {
	data, err := os.ReadFile("testdata/migration-identities.json")
	if err != nil {
		t.Fatal(err)
	}
	type identity struct {
		Version string `json:"version"`
		SHA256  string `json:"sha256"`
	}
	var previous map[string]identity
	if err := json.Unmarshal(data, &previous); err != nil {
		t.Fatal(err)
	}
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	current := map[string]identity{}
	for _, item := range catalog.Skills {
		current["skill:"+item.Key] = identity{item.Version, item.SHA256}
	}
	for _, item := range catalog.Connectors {
		current["connector:"+item.Source] = identity{item.Version, item.SHA256}
	}
	for _, item := range catalog.Experts {
		body, _ := json.Marshal(item)
		sum := sha256.Sum256(body)
		current["expert:"+item.Key] = identity{item.Version, hex.EncodeToString(sum[:])}
	}
	for key, expected := range previous {
		actual, ok := current[key]
		if !ok {
			t.Fatalf("previous resource missing: %s", key)
		}
		if actual.Version == expected.Version && actual.SHA256 != expected.SHA256 {
			t.Fatalf("resource content changed at the same version: %s", key)
		}
	}
}
