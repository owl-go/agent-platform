package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"agent-platform/backend/internal/cliconnector"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: connector-zip-build <source.zip> <bundle.tgz>")
		os.Exit(2)
	}
	source, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	artifact, err := (cliconnector.ZIPPackageBuilder{}).Build(context.Background(), source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], artifact.BundleBytes, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("bundle_sha256=%x\n", sha256.Sum256(artifact.BundleBytes))
}
