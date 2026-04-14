//go:build darwin || linux || freebsd || openbsd || netbsd

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readSecret prompts on stderr and reads a line from stdin with echo
// disabled (via `stty -echo`) when stdin is a TTY. Falls back to a
// plain echoed read when stdin is not a terminal or stty is unavailable.
// Using stty avoids pulling in golang.org/x/term as a dependency.
func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	fi, _ := os.Stdin.Stat()
	isTTY := fi.Mode()&os.ModeCharDevice != 0

	if isTTY {
		if _, err := exec.LookPath("stty"); err == nil {
			cmd := exec.Command("stty", "-echo")
			cmd.Stdin = os.Stdin
			if err := cmd.Run(); err == nil {
				defer func() {
					restore := exec.Command("stty", "echo")
					restore.Stdin = os.Stdin
					restore.Run()
					fmt.Fprintln(os.Stderr)
				}()
			}
		}
	}

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
