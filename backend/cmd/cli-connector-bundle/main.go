// cli-connector-bundle uses the same ZIP assembler as the platform publisher.
package main

import (
	"context"
	"fmt"
	"os"

	"agent-platform/backend/internal/cliconnector"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: cli-connector-bundle <source.zip> <bundle.tgz>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(source, output string) error {
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 50<<20 {
		return fmt.Errorf("source ZIP must be a regular file no larger than 50 MiB")
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	artifact, err := (cliconnector.ZIPPackageBuilder{}).Build(context.Background(), content)
	if err != nil {
		return err
	}
	return os.WriteFile(output, artifact.BundleBytes, 0o644)
}
