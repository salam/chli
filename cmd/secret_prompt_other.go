//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Fallback: Windows and any platform where we don't attempt to disable
// terminal echo. Input is echoed; users who want no-echo can pass
// --password or pipe from a password manager.
func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
