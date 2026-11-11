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

// TestExplainACode: the page of a known code goes to stdout, with its note
// as the title, its exit status and its messages; stderr stays empty.
func TestExplainACode(t *testing.T) {
	code, stdout, stderr := run("explain", "SHIP-E0618")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		"SHIP-E0618 — A request to GitHub got no answer (network, proxy, outage).",
		"Exit code 1.",
		"does not answer",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout misses %q:\n%s", want, stdout)
		}
	}
	// A warning says ship goes on, and a remedy when the catalogue has one.
	if _, stdout, _ := run("explain", "SHIP-W0310"); !strings.Contains(stdout, "Warning: ship goes on.") ||
		!strings.Contains(stdout, "To fix it: ") {
		t.Errorf("the page of SHIP-W0310:\n%s", stdout)
	}
}

// TestExplainNormalisesTheInput: case, surrounding spaces and the SHIP-
// prefix (missing, doubled) do not change the page.
func TestExplainNormalisesTheInput(t *testing.T) {
	_, want, _ := run("explain", "SHIP-E0618")
	for _, given := range []string{"e0618", "  E0618  ", "ship-e0618", "SHIP-SHIP-E0618"} {
		if _, stdout, _ := run("explain", given); stdout != want {
			t.Errorf("%q: page differs from SHIP-E0618's:\n%s", given, stdout)
		}
	}
}

// TestExplainUnknownCode: a code nothing is known about — SHIP-I passing
// checks included — is a usage error (64), said on stderr only.
func TestExplainUnknownCode(t *testing.T) {
	for _, given := range []string{"SHIP-X9999", "SHIP-I0000", "hello"} {
		code, stdout, stderr := run("explain", given)
		if code != ExitUsage || stdout != "" ||
			!strings.Contains(stderr, "Unknown code") || !strings.Contains(stderr, given) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", given, code, stdout, stderr)
		}
	}
}

// TestExplainListsTheCodes: without a code, every explainable code is listed
// with its note, grouped like the errors page.
func TestExplainListsTheCodes(t *testing.T) {
	code, stdout, stderr := run("explain")
	if code != ExitOK || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if n := strings.Count(stdout, "\n  SHIP-"); n != len(i18n.FaultPages()) {
		t.Errorf("listed %d codes, want %d", n, len(i18n.FaultPages()))
	}
	for _, want := range []string{
		"Every error and warning (SHIP-E, SHIP-W)",
		"General:\n  SHIP-E0001",
		"Titles, branch names and sign-offs:",
		"  SHIP-W0411  Undoing a publication found the linked issue's status",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout misses %q", want)
		}
	}
}

// TestExplainJSON: --json prints the page, or the list, as JSON.
func TestExplainJSON(t *testing.T) {
	code, stdout, _ := run("explain", "--json", "SHIP-E0618")
	if code != ExitOK || !strings.HasPrefix(stdout, "{") ||
		!strings.Contains(stdout, `"code": "SHIP-E0618"`) || !strings.Contains(stdout, `"keys"`) ||
		!strings.Contains(stdout, `"messages"`) || !strings.Contains(stdout, "does not answer") {
		t.Errorf("explain --json SHIP-E0618: exit %d\n%s", code, stdout)
	}
	if code, stdout, _ := run("explain", "--json"); code != ExitOK || !strings.HasPrefix(stdout, "[") || !strings.Contains(stdout, `"code": "SHIP-E0001"`) {
		t.Errorf("explain --json: exit %d\n%s", code, stdout)
	}
}

// TestExplainInFrench: the page and the unknown-code message are translated.
func TestExplainInFrench(t *testing.T) {
	deps := Deps{Git: testkit.HealthyRepository("main"), Checkpoints: &testkit.FakeCheckpointStore{},
		Operations: &testkit.FakeOperationLog{}, OperatingSystem: "linux"}
	run := func(args ...string) (int, string, string) {
		var out, errOut bytes.Buffer
		code := Run(context.Background(), args, Streams{Out: &out, Err: &errOut}, i18n.French, testBuild, deps)
		return code, out.String(), errOut.String()
	}
	if code, stdout, _ := run("explain", "SHIP-W0310"); code != ExitOK ||
		!strings.Contains(stdout, "Aucun token GitHub") || !strings.Contains(stdout, "Pour corriger : ") {
		t.Errorf("exit %d\n%s", code, stdout)
	}
	if code, stdout, _ := run("explain", "--json", "SHIP-W0310"); code != ExitOK ||
		!strings.Contains(stdout, `"messages"`) || !strings.Contains(stdout, "Aucun token GitHub") ||
		!strings.Contains(stdout, `"remedies"`) {
		t.Errorf("explain --json in French: exit %d\n%s", code, stdout)
	}
	if code, _, stderr := run("explain", "SHIP-X9999"); code != ExitUsage || !strings.Contains(stderr, "Code inconnu") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

// TestExplainHelpAndExtraArguments: --help prints the usage and exits 0;
// more than one code is a usage error.
func TestExplainHelpAndExtraArguments(t *testing.T) {
	if code, stdout, _ := run("explain", "--help"); code != ExitOK || !strings.HasPrefix(stdout, "Usage: ship explain [flags] [<code>]\n") {
		t.Errorf("explain --help: exit %d\n%s", code, stdout)
	}
	code, stdout, stderr := run("explain", "SHIP-E0618", "SHIP-E0101")
	if code != ExitUsage || stdout != "" || !strings.Contains(stderr, `Unexpected argument "SHIP-E0101"`) {
		t.Errorf("two codes: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
