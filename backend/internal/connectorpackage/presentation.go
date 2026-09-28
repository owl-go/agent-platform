package connectorpackage

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors. Other packages keep the generic Connector icon.
func DisplayIcon(source string) string {
	switch source {
	case "feishu", "dingtalk":
		return source
	}
	return "plug"
}
