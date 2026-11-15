// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/i18n"
	"github.com/GregoireF/ship-it/internal/testkit"
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

func TestPublishYesPassesUnattended(t *testing.T) {
	var out, errOut bytes.Buffer
	git := testkit.HealthyRepository("feat/42-add-vat")
	git.CommitMessages = []string{"✨ feat: add vat"}
	forge := &testkit.FakeForge{}
	deps := Deps{
		Git:             git,
		Checkpoints:     &testkit.FakeCheckpointStore{},
		Operations:      &testkit.FakeOperationLog{},
		Forge:           forge,
		OperatingSystem: "linux",
	}
	code := Run(context.Background(), []string{"publish", "--yes"}, Streams{Out: &out, Err: &errOut}, i18n.English, testBuild, deps)
	if code != ExitOK {
		t.Fatalf("publish --yes: exit %d, stderr %q", code, errOut.String())
	}
	if len(forge.Created) == 0 {
		t.Fatal("publish --yes did not create a pull request")
	}
	if !forge.Created[0].Draft {
		t.Fatal("publish --yes must open a draft (Unattended)")
	}
}
