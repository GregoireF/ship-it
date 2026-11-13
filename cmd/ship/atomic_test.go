// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/i18n"
	"github.com/GregoireF/ship-it/internal/surface/cli"
)

// answer runs the CLI with stdin typed by a human in a terminal.
func (s *scenario) answer(stdin string, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	streams := cli.Streams{In: strings.NewReader(stdin), Out: &out, Err: &errOut, Interactive: true}
	code := cli.Run(context.Background(), args, streams, i18n.English, cli.BuildInfo{}, s.deps)
	return code, out.String(), errOut.String()
}

// TestShipSplitsTheWorkByIntent: #34. A rename in two places, then a new
// function with its test: the review proposes two commits, the second
// holds the feature only, each commit holds what it says, and one undo
// takes both back.
func TestShipSplitsTheWorkByIntent(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	renamed := "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n"
	s.write("cart.go", renamed)
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 20 }\n")
	s.write("vat_test.go", "package shop\n\nimport \"testing\"\n\nfunc TestVAT(t *testing.T) { _ = VAT() }\n")

	// Yes to the split, no change, every default, then ship.
	code, _, stderr := s.answer(strings.Repeat("\n", 8), "--plain", "--local")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	for _, want := range []string{"This work does 2 things", "renames total to sum\n     1.1 cart.go:3  func total() int { → func sum() int {", "Make 2 commits?", "✓ 2 commits on "} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	subjects := s.sh("log", "--format=%s", "main..HEAD")
	if subjects != "✨ feat: add vat\n♻️ refactor: rename total to sum" {
		t.Fatalf("commits:\n%s", subjects)
	}
	if got := s.sh("show", "HEAD~1:cart.go"); got != strings.TrimSpace(renamed) {
		t.Errorf("the rename commit holds cart.go as:\n%s", got)
	}
	if files := s.sh("show", "--name-only", "--format=", "HEAD~1"); files != "cart.go" {
		t.Errorf("the rename commit holds %q", files)
	}
	if files := s.sh("show", "--name-only", "--format=", "HEAD"); files != "vat.go\nvat_test.go" {
		t.Errorf("the feature commit holds %q", files)
	}
	if status := s.sh("status", "--porcelain"); status != "" {
		t.Errorf("work left behind:\n%s", status)
	}

	if code, _, stderr := s.ship("undo", "--yes"); code != 0 {
		t.Fatalf("undo: exit %d\n%s", code, stderr)
	}
	if head := s.sh("log", "--format=%s", "-1"); head != "🎉 chore: init" || s.sh("branch", "--show-current") != "main" {
		t.Errorf("after undo: %s on %s", head, s.sh("branch", "--show-current"))
	}
	if status := s.sh("status", "--porcelain", "--untracked-files=all"); status != "M cart.go\n?? vat.go\n?? vat_test.go" {
		t.Errorf("after undo, the work is:\n%s", status)
	}
}

// TestShipSplitsOneFileByHunk: #34. Two intents in one file: the first
// commit holds the file with its own hunk only.
func TestShipSplitsOneFileByHunk(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	final := "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n\nfunc Discount() int { return 5 }\n"
	s.write("cart.go", final)

	code, _, stderr := s.answer(strings.Repeat("\n", 8), "--plain", "--local")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if n := s.sh("rev-list", "--count", "main..HEAD"); n != "2" {
		t.Fatalf("%s commits\n%s", n, stderr)
	}
	first := "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}"
	if got := s.sh("show", "HEAD~1:cart.go"); got != first {
		t.Errorf("the first commit holds cart.go as:\n%s", got)
	}
	if got := s.sh("show", "HEAD:cart.go"); got != strings.TrimSpace(final) {
		t.Errorf("the last commit holds cart.go as:\n%s", got)
	}
}

// TestShipOneCommitWhenTheSplitIsDeclined: n keeps today's single commit.
func TestShipOneCommitWhenTheSplitIsDeclined(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	s.write("cart.go", "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n")
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 20 }\n")

	code, _, stderr := s.answer("n\n"+strings.Repeat("\n", 5), "--plain", "--local")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if n := s.sh("rev-list", "--count", "main..HEAD"); n != "1" {
		t.Errorf("%s commits", n)
	}
}

// TestSplitRefusedWhenTheWorkChanged: #34. The hunks a human placed are
// those of the work reviewed: an edit since stops the delivery before
// anything changes.
func TestSplitRefusedWhenTheWorkChanged(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	s.write("cart.go", "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n")
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 20 }\n")
	ctx := context.Background()
	view, err := app.ProposePlan(ctx, s.deps, app.PlanRequest{Atomic: true})
	if err != nil || len(view.Commits) != 2 {
		t.Fatalf("plan: %v, %d commits", err, len(view.Commits))
	}
	req := app.DeliveryRequest{Title: view.Title, Branch: view.Branch, HunksDigest: view.HunksDigest}
	for _, c := range view.Commits {
		spec := app.CommitSpec{Title: c.Title}
		for _, h := range c.Hunks {
			spec.Hunks = append(spec.Hunks, h.Ref())
		}
		req.Commits = append(req.Commits, spec)
	}
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 21 }\n")

	_, err = app.DeliverLocally(ctx, s.deps, req)
	if !errors.Is(err, &fault.Fault{Code: fault.CommitsInvalid}) {
		t.Fatalf("err = %v", err)
	}
	if n := s.sh("rev-list", "--count", "HEAD"); n != "1" || s.sh("branch", "--show-current") != "main" {
		t.Errorf("something changed: %s commits on %s", n, s.sh("branch", "--show-current"))
	}
}

// TestPlainReviewMovesAChange: #36. Line by line, the test file moves to
// the rename commit, then the commits swap: the commits follow.
func TestPlainReviewMovesAChange(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	s.write("cart.go", "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n")
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 20 }\n")
	s.write("vat_test.go", "package shop\n\nimport \"testing\"\n\nfunc TestVAT(t *testing.T) { _ = VAT() }\n")

	// Yes to the split, an invalid change, 2.2 to commit 1, swap, done.
	code, _, stderr := s.answer("\n9.9 1\n2.2 1\ndown 1\n\n"+strings.Repeat("\n", 6), "--plain", "--local")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "✗ Not a change") {
		t.Errorf("the invalid change was not explained:\n%s", stderr)
	}
	if subjects := s.sh("log", "--format=%s", "main..HEAD"); subjects != "♻️ refactor: rename total to sum\n✨ feat: add vat" {
		t.Fatalf("commits:\n%s", subjects)
	}
	if files := s.sh("show", "--name-only", "--format=", "HEAD~1"); files != "vat.go" {
		t.Errorf("the first commit holds %q", files)
	}
	if files := s.sh("show", "--name-only", "--format=", "HEAD"); files != "cart.go\nvat_test.go" {
		t.Errorf("the last commit holds %q", files)
	}
}

// TestPlainReviewMergesBackIntoOne: merged into one, the work is one
// ordinary commit with the title kept.
func TestPlainReviewMergesBackIntoOne(t *testing.T) {
	s := newScenario(t, nil)
	s.write("cart.go", "package shop\n\nfunc total() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn total()\n}\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	s.write("cart.go", "package shop\n\nfunc sum() int {\n\treturn 1\n}\n\nfunc checkout() int {\n\treturn sum()\n}\n")
	s.write("vat.go", "package shop\n\nfunc VAT() int { return 20 }\n")

	code, _, stderr := s.answer("\nmerge 2 1\n"+strings.Repeat("\n", 5), "--plain", "--local")
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, stderr)
	}
	if subjects := s.sh("log", "--format=%s", "main..HEAD"); subjects != "✨ feat: add vat" {
		t.Errorf("commits:\n%s", subjects)
	}
	if status := s.sh("status", "--porcelain"); status != "" {
		t.Errorf("work left behind:\n%s", status)
	}
}
