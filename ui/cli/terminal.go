package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// readLinePlain reads one line without touching terminal modes.
func readLinePlain(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// promptLogin asks for the username or email on stderr and reads stdout's
// companion stdin.
func promptLogin() (string, error) {
	fmt.Fprint(os.Stderr, "Login (username or email): ")
	return readLinePlain(os.Stdin)
}

// promptPassword asks for a password on stderr. readPassword is implemented
// per-platform so supported terminals can disable echo.
func promptPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Password: ")
	return readPassword(os.Stdin)
}
