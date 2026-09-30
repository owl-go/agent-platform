package connectorpackage

import (
	_ "embed"
	"encoding/base64"
)

//go:embed icons/teambition.png
var teambitionIcon []byte

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors, including the official Teambition image. Other packages keep
// the generic Connector icon.
func DisplayIcon(source string) string {
	switch source {
	case "teambition":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(teambitionIcon)
	case "feishu", "dingtalk":
		return source
	}
	return "plug"
}
