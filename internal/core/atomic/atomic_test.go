// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package atomic

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/core/intent"
	"github.com/GregoireF/ship-it/internal/core/snapshot"
)

// zeroContext is what `git diff -U0` prints for old → new below: a line
// replaced, one removed, two added after the last line.
const zeroContext = `diff --git a/cart.go b/cart.go
index 1111111..2222222 100644
--- a/cart.go
+++ b/cart.go
@@ -2 +2 @@ package shop
-func Total() int { return 0 }
+func Total() int { return 1 }
@@ -4 +3,0 @@ func Total() int { return 0 }
-// obsolete
@@ -6,0 +6,2 @@ func Empty() bool { return true }
+
+func Count() int { return 0 }
`

const (
	oldCart = "package shop\nfunc Total() int { return 0 }\n\n// obsolete\n\nfunc Empty() bool { return true }\n"
	newCart = "package shop\nfunc Total() int { return 1 }\n\n\nfunc Empty() bool { return true }\n\nfunc Count() int { return 0 }\n"
)

func cartHunks(t *testing.T) []snapshot.Hunk {
	t.Helper()
	files := snapshot.ParseUnifiedDiff(zeroContext)
	if len(files) != 1 || len(files[0].Hunks) != 3 {
		t.Fatalf("parsed %+v", files)
	}
	return files[0].Hunks
}

func TestComposeEveryHunkGivesTheNewVersion(t *testing.T) {
	if got := Compose(oldCart, cartHunks(t)); got != newCart {
		t.Errorf("Compose = %q, want %q", got, newCart)
	}
}

func TestComposeSomeHunks(t *testing.T) {
	h := cartHunks(t)
	for _, tc := range []struct {
		hunks []snapshot.Hunk
		want  string
	}{
		{nil, oldCart},
		{h[:1], "package shop\nfunc Total() int { return 1 }\n\n// obsolete\n\nfunc Empty() bool { return true }\n"},
		{h[1:2], "package shop\nfunc Total() int { return 0 }\n\n\nfunc Empty() bool { return true }\n"},
		{h[2:], oldCart + "\nfunc Count() int { return 0 }\n"},
	} {
		if got := Compose(oldCart, tc.hunks); got != tc.want {
			t.Errorf("Compose(%d hunks) = %q, want %q", len(tc.hunks), got, tc.want)
		}
	}
}

func TestComposeKeepsLineEndings(t *testing.T) {
	old := strings.ReplaceAll(oldCart, "\n", "\r\n")
	want := strings.ReplaceAll(newCart, "\n", "\r\n")
	if got := Compose(old, cartHunks(t)); got != want {
		t.Errorf("Compose = %q, want %q", got, want)
	}
}

func TestComposeAfterALastLineWithoutEnd(t *testing.T) {
	added := snapshot.Hunk{OldStart: 1, NewStart: 2, Added: []string{"b"}}
	if got := Compose("a", []snapshot.Hunk{added}); got != "a\nb\n" {
		t.Errorf("Compose = %q", got)
	}
	if got := Compose("a", nil); got != "a" {
		t.Errorf("no hunk changed the file: %q", got)
	}
}

func work() []snapshot.FileDiff {
	cart := snapshot.ParseUnifiedDiff(zeroContext)[0]
	return []snapshot.FileDiff{
		cart,
		{Path: "logo.png", Status: snapshot.Modified, Binary: true},
		{Path: "vat.go", Status: snapshot.Added, Hunks: []snapshot.Hunk{{NewStart: 1, Added: []string{"package shop"}}}},
		{Path: "new/name.go", OriginalPath: "old/name.go", Status: snapshot.Renamed, Hunks: []snapshot.Hunk{{OldStart: 1, NewStart: 1}}},
	}
}

func ref(path string, index int) intent.HunkRef { return intent.HunkRef{Path: path, Index: index} }

func TestAssign(t *testing.T) {
	groups := []intent.Group{
		{Type: "refactor", Hunks: []intent.HunkRef{ref("new/name.go", intent.MoveHunk), ref("cart.go", 0)}},
		{Type: "feat", Hunks: []intent.HunkRef{ref("vat.go", 0), ref("new/name.go", 0), ref("cart.go", 2)}},
		{Type: "chore", Hunks: []intent.HunkRef{ref("cart.go", 1)}},
		{Type: "style", Hunks: []intent.HunkRef{ref("gone.go", 0)}},
	}
	var got string
	for _, g := range Assign(work(), groups) {
		got += g.Type + fmt.Sprint(g.Hunks)
	}
	want := "refactor[{new/name.go -1} {cart.go 0}]feat[{vat.go -1} {cart.go 2}]chore[{cart.go 1} {logo.png -1}]"
	if got != want {
		t.Errorf("Assign = %s\nwant      %s", got, want)
	}
}

func TestStages(t *testing.T) {
	commits := [][]intent.HunkRef{
		{ref("new/name.go", Whole), ref("cart.go", 0)},
		{ref("vat.go", Whole), ref("cart.go", 2)},
		{ref("cart.go", 1), ref("logo.png", Whole)},
	}
	stages, err := Stages(work(), commits)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprint(stages)
	want := "[[{cart.go  false [0]} {new/name.go old/name.go true []}] [{cart.go  false [0 2]} {vat.go  true []}] [{cart.go  true []} {logo.png  true []}]]"
	if got != want {
		t.Errorf("Stages = %s\nwant      %s", got, want)
	}
}

func TestStagesRefusesWhatDoesNotCoverTheWork(t *testing.T) {
	all := []intent.HunkRef{ref("cart.go", 0), ref("cart.go", 1), ref("cart.go", 2), ref("logo.png", Whole), ref("vat.go", Whole), ref("new/name.go", Whole)}
	for name, commits := range map[string][][]intent.HunkRef{
		"a hunk left out":               {all[:5]},
		"a hunk twice":                  {all, {all[0]}},
		"an empty commit":               {all, nil},
		"an unknown file":               {append(all, ref("other.go", Whole))},
		"a hunk of a whole file":        {append(all[:5:5], ref("new/name.go", 0))},
		"a whole splittable file":       {append(all[3:], ref("cart.go", Whole))},
		"a hunk past the end of a file": {append(all, ref("cart.go", 3))},
	} {
		if _, err := Stages(work(), commits); !errors.Is(err, &fault.Fault{Code: fault.CommitsInvalid}) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func FuzzCompose(f *testing.F) {
	f.Add("a\nb\nc\n", 2, 1, "x")
	f.Add("a", 1, 0, "b")
	f.Add("", 0, 0, "")
	f.Fuzz(func(t *testing.T, old string, start, removed int, added string) {
		h := snapshot.Hunk{OldStart: start % 1000, Removed: make([]string, max(removed%5, 0)), Added: strings.Split(added, "\n")}
		got := Compose(old, []snapshot.Hunk{h})
		if strings.Count(got, "\n") > strings.Count(old, "\n")+len(h.Added)+1 {
			t.Fatalf("Compose(%q, %+v) = %q: more lines than it could add", old, h, got)
		}
	})
}
