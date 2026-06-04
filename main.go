// Command itonics is a CLI for the ITONICS Innovation OData v2 API.
package main

import (
	"fmt"
	"os"

	"github.com/itonics/itonics-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
