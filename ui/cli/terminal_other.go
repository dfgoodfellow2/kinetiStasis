//go:build !linux

package cli

import (
	"fmt"
	"os"
)

// readPassword cannot disable terminal echo on this platform, so it warns and
// reads a plain line. Prefer --password or DIET_PASSWORD for non-interactive
// use.
func readPassword(f *os.File) (string, error) {
	fmt.Fprintln(os.Stderr, "warning: cannot disable terminal echo on this platform; password will be visible")
	return readLinePlain(f)
}
