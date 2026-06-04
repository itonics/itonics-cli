package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// confirm prompts on stderr and returns nil on y/yes, an error otherwise.
func confirm(format string, args ...any) error {
	fmt.Fprintf(os.Stderr, format+" [y/N] ", args...)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return errors.New("aborted")
	}
	a := strings.ToLower(strings.TrimSpace(scanner.Text()))
	if a == "y" || a == "yes" {
		return nil
	}
	return errors.New("aborted")
}
