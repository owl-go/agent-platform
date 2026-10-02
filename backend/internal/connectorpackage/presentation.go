package connectorpackage

import (
	_ "embed"
	"encoding/base64"
)

//go:embed icons/teambition.png
var teambitionIcon []byte

//go:embed icons/moka-hr.png
var mokaHRIcon []byte

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors, including the official Teambition and Moka images. Other packages keep
// the generic Connector icon.
func DisplayIcon(source string) string {
	switch source {
	case "teambition":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(teambitionIcon)
	case "moka-hr":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(mokaHRIcon)
	case "feishu", "dingtalk", "notion", "modao":
		return source
	}
	return "plug"
}

// DisplayName and DisplayDescription keep previously published Notion revisions
// on the current product label without mutating their immutable package metadata.
func DisplayName(source, declared string) string {
	if source == "notion" {
		return "Notion"
	}
	return declared
}

func DisplayDescription(source, declared string) string {
	if source == "notion" {
		return "Read and manage Notion pages and query data sources"
	}
	return declared
}
