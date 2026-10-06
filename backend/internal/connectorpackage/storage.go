package connectorpackage

import (
	"encoding/json"
	"fmt"
)

// ObjectKey and RuntimePolicy are shared by interactive uploads and starter
// catalog initialization, so both persist the same verified package contract.
func (pkg Package) ObjectKey() string {
	return fmt.Sprintf("connectors/%s/%s/%s.zip", pkg.Metadata.Source, pkg.Metadata.Version, pkg.SHA256)
}
func (pkg Package) BundleObjectKey() string {
	return fmt.Sprintf("cli-connectors/packages/%s/%s/%s.tgz", pkg.Metadata.Source, pkg.Metadata.Version, pkg.CLIBundleSHA256)
}
func (pkg Package) RuntimePolicy() ([]byte, error) {
	policy := map[string]any{"auth_mode": pkg.Metadata.AuthMode, "metadata": pkg.Metadata, "mcp": pkg.MCP, "cli": pkg.CLI}
	if len(pkg.CLIBundle) > 0 {
		policy["cli_bundle_object_key"] = pkg.BundleObjectKey()
		policy["cli_bundle_sha256"] = pkg.CLIBundleSHA256
	}
	return json.Marshal(policy)
}
