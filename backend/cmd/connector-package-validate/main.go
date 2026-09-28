package main

import (
	"fmt"
	"os"

	"agent-platform/backend/internal/connectorpackage"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: connector-package-validate <package.zip>")
		os.Exit(2)
	}
	info, err := os.Stat(os.Args[1])
	if err != nil || !info.Mode().IsRegular() || info.Size() > 50<<20 {
		fmt.Fprintln(os.Stderr, "Connector Package must be a regular file no larger than 50 MiB")
		os.Exit(1)
	}
	archive, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	item, err := connectorpackage.Parse(archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	capabilities := 0
	if item.CLI != nil {
		capabilities = len(item.CLI.Capabilities)
	}
	fmt.Printf("source=%s version=%s sha256=%s bundle_sha256=%s skills=%d capabilities=%d\n", item.Metadata.Source, item.Metadata.Version, item.SHA256, item.CLIBundleSHA256, len(item.Skills), capabilities)
}
