package connectorpackage

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors. Other packages keep the generic Connector icon.
func DisplayIcon(source string) string {
	if source == "feishu" {
		return "feishu"
	}
	return "plug"
}
