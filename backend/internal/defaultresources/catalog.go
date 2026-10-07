// Package defaultresources loads credential-free platform definitions from a
// release directory. It has no database, authorization or availability state.
package defaultresources

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/systemskills"
)

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
	archives   map[string][]byte
}

func decodeDefinition(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode resource definition: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("resource definition contains trailing data")
	}
	return nil
}

func (catalog Catalog) validate() error {
	seen := map[string]bool{}
	names := map[string]bool{}
	for _, definition := range systemskills.Definitions() {
		seen["skill:"+definition.Key] = true
		names["skill:"+strings.ToLower(strings.TrimSpace(definition.Name))] = true
	}
	check := func(kind, key, version string) error {
		if !resourceKey.MatchString(key) || !resourceVersion.MatchString(version) || seen[kind+":"+key] {
			return fmt.Errorf("invalid or duplicate default %s key %q", kind, key)
		}
		seen[kind+":"+key] = true
		return nil
	}
	checkName := func(kind, name string) error {
		key := kind + ":" + strings.ToLower(strings.TrimSpace(name))
		if names[key] {
			return fmt.Errorf("duplicate %s catalog name %q", kind, name)
		}
		names[key] = true
		return nil
	}
	for _, s := range catalog.Skills {
		if err := checkName("skill", s.Name); err != nil {
			return err
		}
		if err := check("skill", s.Key, s.Version); err != nil {
			return err
		}
		if err := catalog.verifyArchive(s.Archive, s.SHA256); err != nil {
			return err
		}
	}
	for _, e := range catalog.Experts {
		if err := checkName("expert", e.Name); err != nil {
			return err
		}
		if err := check("expert", e.Key, e.Version); err != nil {
			return err
		}
		for _, key := range e.SkillKeys {
			if !seen["skill:"+key] {
				return fmt.Errorf("default Expert %s references unknown Skill %s", e.Key, key)
			}
		}
	}
	for _, c := range catalog.Connectors {
		if err := check("connector", c.Source, c.Version); err != nil {
			return err
		}
		if err := catalog.verifyArchive(c.Archive, c.SHA256); err != nil {
			return err
		}
	}
	return nil
}

func (catalog Catalog) verifyArchive(name, expected string) error {
	data, err := catalog.Archive(name)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("default archive %s checksum mismatch", name)
	}
	return nil
}

func (catalog Catalog) Archive(name string) ([]byte, error) {
	data, ok := catalog.archives[name]
	if !ok {
		return nil, fmt.Errorf("archive %s is outside the loaded resource catalog", name)
	}
	return bytes.Clone(data), nil
}

func (catalog Catalog) ParseConnector(c Connector) (connectorpackage.Package, error) {
	data, err := catalog.Archive(c.Archive)
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
