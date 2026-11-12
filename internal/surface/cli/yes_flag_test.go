// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"strings"
	"testing"
)

// TestWritingCommandsAcceptYes guards issue #122: every command that writes
// accepts --yes/-y, so scripts and agents can pass the same flag everywhere.
// None of them asks a question today, so the flag is accepted and ignored,
// like `stack sync`.
func TestWritingCommandsAcceptYes(t *testing.T) {
	commands := [][]string{
		{"publish"},
		{"tidy"},
		{"init"},
		{"label", "--pr", "12"},
		{"check", "gates"},
	}
	for _, command := range commands {
		name := strings.Join(command, " ")
		code, stdout, stderr := run(append(append([]string{}, command...), "--yes", "--help")...)
		if code != ExitOK || stderr != "" {
			t.Errorf("%s --yes --help: exit %d, stderr %q", name, code, stderr)
		}
		if !strings.Contains(stdout, "--yes") || !strings.Contains(stdout, "-y") {
			t.Errorf("%s --help does not show --yes/-y:\n%s", name, stdout)
		}
	}
}
