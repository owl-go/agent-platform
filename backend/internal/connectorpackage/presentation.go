package connectorpackage

import (
	_ "embed"
	"encoding/base64"
)

//go:embed icons/teambition.png
var teambitionIcon []byte

//go:embed icons/picset-ai.png
var picsetIcon []byte

//go:embed icons/kling-ai.png
var klingIcon []byte

//go:embed icons/linear.png
var linearIcon []byte

//go:embed icons/pixso.png
var pixsoIcon []byte

//go:embed icons/camscanner.png
var camscannerIcon []byte

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors, including the official Teambition image. Other packages keep
// the generic Connector icon.
func DisplayIcon(source string) string {
	switch source {
	case "picset-ai":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(picsetIcon)

	case "kling-ai":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(klingIcon)
	case "pixso":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pixsoIcon)
	case "linear":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(linearIcon)
	case "camscanner":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(camscannerIcon)
	case "teambition":
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(teambitionIcon)
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
