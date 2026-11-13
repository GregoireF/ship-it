// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWorkingFingerprint: #38. It changes when a file is edited again,
// created or deleted, and not for an ignored file.
func TestWorkingFingerprint(t *testing.T) {
	c := newRepo(t)
	ctx := context.Background()
	put := func(name, content string, mtime time.Time) {
		t.Helper()
		path := filepath.Join(c.Dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	fingerprint := func() string {
		t.Helper()
		f, err := c.WorkingFingerprint(ctx)
		if err != nil || f == "" {
			t.Fatalf("fingerprint %q, %v", f, err)
		}
		return f
	}
	base := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	put(".gitignore", "build/\n", base)
	put("a.txt", "one\n", base)
	start := fingerprint()
	if again := fingerprint(); again != start {
		t.Fatalf("nothing changed, yet %s became %s", start, again)
	}
	// Still untracked: only the size and time say it changed.
	put("a.txt", "two!\n", base.Add(time.Second))
	edited := fingerprint()
	if edited == start {
		t.Error("an edit must change the fingerprint")
	}
	put("build/out.bin", "x", base)
	if fingerprint() != edited {
		t.Error("an ignored file must not change the fingerprint")
	}
	if err := os.Remove(filepath.Join(c.Dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if fingerprint() == edited {
		t.Error("a deletion must change the fingerprint")
	}
}
