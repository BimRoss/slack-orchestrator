// Command catalog-export prints DefaultCapabilityContractJSON to stdout (CI, seed scripts, jq).
package main

import (
	"fmt"
	"os"

	"github.com/bimross/slack-orchestrator/internal/inbound"
)

func main() {
	if _, err := os.Stdout.Write(inbound.DefaultCapabilityContractJSON()); err != nil {
		fmt.Fprintf(os.Stderr, "catalog-export: %v\n", err)
		os.Exit(1)
	}
}
