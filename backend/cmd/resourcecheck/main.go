// resourcecheck validates the release resource directories without touching a
// database, credentials, or external services.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"agent-platform/backend/internal/defaultresources"
)

func main() {
	root := flag.String("root", "..", "directory containing resources.json, connectors/, skills/ and experts/")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	catalog, err := defaultresources.LoadDirectory(ctx, *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Resource directories validated: %d Connector Packages, %d Skills, %d Experts (+3 built-in creation Skills)\n", len(catalog.Connectors), len(catalog.Skills), len(catalog.Experts))
}
