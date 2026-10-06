// Package defaultresources owns the credential-free starter catalog shipped in
// the service binary. It has no database, authorization or availability state.
package defaultresources

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"agent-platform/backend/internal/connectorpackage"
)

//go:embed assets
var assets embed.FS

type Skill struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Icon    string `json:"icon"`
	Version string `json:"version"`
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
}
type Expert struct {
	Key                string   `json:"key"`
	Version            string   `json:"version"`
	Name               string   `json:"name"`
	Icon               string   `json:"icon"`
	IconBackground     string   `json:"icon_background"`
	Introduction       string   `json:"introduction"`
	CoreCapability     string   `json:"core_capability"`
	OperatingProcedure string   `json:"operating_procedure"`
	OutputStandard     string   `json:"output_standard"`
	Cautions           string   `json:"cautions"`
	SkillKeys          []string `json:"skill_keys"`
}
type Connector struct {
	Source  string `json:"source"`
	Version string `json:"version"`
	Archive string `json:"archive"`
	SHA256  string `json:"sha256"`
}
type Catalog struct {
	Version    string      `json:"version"`
	Skills     []Skill     `json:"skills"`
	Experts    []Expert    `json:"experts"`
	Connectors []Connector `json:"connectors"`
}

func Load() (Catalog, error) {
	data, err := assets.ReadFile("assets/manifest.json")
	if err != nil {
		return Catalog{}, fmt.Errorf("read default resource manifest: %w", err)
	}
	var catalog Catalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode default resource manifest: %w", err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return Catalog{}, fmt.Errorf("default resource manifest contains trailing data")
	}
	seen := map[string]bool{}
	check := func(kind, key, version string) error {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(version) == "" || seen[kind+":"+key] {
			return fmt.Errorf("invalid or duplicate default %s key %q", kind, key)
		}
		seen[kind+":"+key] = true
		return nil
	}
	for _, s := range catalog.Skills {
		if err = check("skill", s.Key, s.Version); err != nil {
			return Catalog{}, err
		}
		if err = verifyArchive(s.Archive, s.SHA256); err != nil {
			return Catalog{}, err
		}
	}
	for _, e := range catalog.Experts {
		if err = check("expert", e.Key, e.Version); err != nil {
			return Catalog{}, err
		}
		for _, key := range e.SkillKeys {
			if !seen["skill:"+key] {
				return Catalog{}, fmt.Errorf("default Expert %s references unknown Skill %s", e.Key, key)
			}
		}
	}
	for _, c := range catalog.Connectors {
		if err = check("connector", c.Source, c.Version); err != nil {
			return Catalog{}, err
		}
		if err = verifyArchive(c.Archive, c.SHA256); err != nil {
			return Catalog{}, err
		}
	}
	return catalog, nil
}

func verifyArchive(name, expected string) error {
	data, err := Archive(name)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("default archive %s checksum mismatch", name)
	}
	return nil
}

func Archive(name string) ([]byte, error) {
	if path.Clean(name) != name || strings.Contains(name, "\\") || (!strings.HasPrefix(name, "skills/") && !strings.HasPrefix(name, "connectors/")) {
		return nil, fmt.Errorf("default archive has invalid directory")
	}
	data, err := assets.ReadFile("assets/" + name)
	if err != nil {
		return nil, fmt.Errorf("read default archive %s: %w", name, err)
	}
	return data, nil
}

func ParseConnector(c Connector) (connectorpackage.Package, error) {
	data, err := Archive(c.Archive)
	if err != nil {
		return connectorpackage.Package{}, err
	}
	pkg, err := connectorpackage.Parse(data)
	if err != nil {
		return connectorpackage.Package{}, fmt.Errorf("default Connector %s: %w", c.Source, err)
	}
	if pkg.Metadata.Source != c.Source || pkg.Metadata.Version != c.Version {
		return connectorpackage.Package{}, fmt.Errorf("default Connector manifest identity mismatch: %s", c.Source)
	}
	return pkg, nil
}
