package connectorpackage

// DisplayIcon identifies a bundled presentation asset for known platform
// Connectors. Other packages keep the generic Connector icon.
func DisplayIcon(source string) string {
	switch source {
	case "feishu", "dingtalk", "notion":
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
