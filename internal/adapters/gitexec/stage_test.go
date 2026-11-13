// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStageForAtomicCommits: #34. StageContent stages a version of a file
// the working tree does not hold, with the file's mode; StagePaths stages
// a deletion; both work from a subdirectory.
func TestStageForAtomicCommits(t *testing.T) {
	c := newRepo(t)
	ctx := context.Background()
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(c.Dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := c.Run(ctx, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	write("src/cart.go", "a\nb\n")
	write("old.txt", "x\n")
	run("add", "--all")
	run("update-index", "--chmod=+x", "src/cart.go")
	run("commit", "--quiet", "-m", "init")
	write("src/cart.go", "a\nB\nc\n")
	if err := os.Remove(filepath.Join(c.Dir, "old.txt")); err != nil {
		t.Fatal(err)
	}

	diff, err := c.ZeroContextDiff(ctx)
	if err != nil || !strings.Contains(diff, "@@ -2 +2,2 @@") {
		t.Fatalf("ZeroContextDiff = %q, %v", diff, err)
	}
	sub := New(filepath.Join(c.Dir, "src"))
	if err := sub.StageContent(ctx, "src/cart.go", []byte("a\nB\n")); err != nil {
		t.Fatal(err)
	}
	if err := sub.StagePaths(ctx, []string{"old.txt"}); err != nil {
		t.Fatal(err)
	}
	if got := run("show", ":src/cart.go"); got != "a\nB" {
		t.Errorf("staged cart.go = %q", got)
	}
	if mode := run("ls-files", "-s", "--", "src/cart.go"); !strings.HasPrefix(mode, "100755 ") {
		t.Errorf("the mode was lost: %q", mode)
	}
	if staged := run("diff", "--cached", "--name-status"); staged != "D\told.txt\nM\tsrc/cart.go" {
		t.Errorf("staged = %q", staged)
	}
	if worktree, _ := os.ReadFile(filepath.Join(c.Dir, "src", "cart.go")); string(worktree) != "a\nB\nc\n" {
		t.Errorf("the working tree changed: %q", worktree)
	}
}
