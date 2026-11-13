// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package intent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/core/snapshot"
)

// render writes groups as "type key [path#index …]" lines, to compare.
func render(groups []Group) string {
	var lines []string
	for _, g := range groups {
		var refs []string
		for _, h := range g.Hunks {
			refs = append(refs, fmt.Sprintf("%s#%d", h.Path, h.Index))
		}
		params := ""
		if g.Params["from"] != "" {
			params = " " + g.Params["from"] + "->" + g.Params["to"]
		}
		lines = append(lines, g.Type+" "+g.Key+params+" ["+strings.Join(refs, " ")+"]")
	}
	return strings.Join(lines, "\n")
}

func hunk(removed, added string) snapshot.Hunk {
	split := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.Split(s, "\n")
	}
	return snapshot.Hunk{Removed: split(removed), Added: split(added)}
}

func TestGroups(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files []snapshot.FileDiff
		want  string
	}{
		{
			"a rename across files, a feature with its test, the rest",
			[]snapshot.FileDiff{
				{Path: "user.go", Hunks: []snapshot.Hunk{
					hunk("func getUser(id int) User {", "func fetchUser(id int) User {"),
					hunk("", "func Delete(id int) error {\n\treturn nil\n}"),
				}},
				{Path: "api.go", Hunks: []snapshot.Hunk{
					hunk("\tu := getUser(id)", "\tu := fetchUser(id)"),
					hunk("\tlimit := 10", "\tlimit := 20"),
				}},
				{Path: "user_test.go", Hunks: []snapshot.Hunk{hunk("", "func TestDelete(t *testing.T) {}")}},
			},
			"refactor intent.rename getUser->fetchUser [user.go#0 api.go#0]\n" +
				"feat intent.feature [user.go#1 user_test.go#0]\n" +
				"fix intent.rest [api.go#1]",
		},
		{
			"formatting apart",
			[]snapshot.FileDiff{{Path: "vat.go", Hunks: []snapshot.Hunk{
				hunk("x:=1", "x := 1"),
				hunk("rate := 0.2", "rate := 0.21"),
			}}},
			"style intent.format [vat.go#0]\nfix intent.rest [vat.go#1]",
		},
		{
			"a word changed in one hunk only is no rename",
			[]snapshot.FileDiff{{Path: "vat.go", Hunks: []snapshot.Hunk{hunk("return a + b", "return a - b")}}},
			"fix intent.rest [vat.go#0]",
		},
		{
			"a file renamed and edited: the move apart from the edit",
			[]snapshot.FileDiff{{Path: "pkg/new.go", OriginalPath: "pkg/old.go", Status: snapshot.Renamed,
				Hunks: []snapshot.Hunk{hunk("package old", "package fresh\n\nfunc Added() {}")}}},
			"refactor intent.move [pkg/new.go#-1]\nfeat intent.feature [pkg/new.go#0]",
		},
		{
			"a test without its code changed",
			[]snapshot.FileDiff{{Path: "vat_test.go", Hunks: []snapshot.Hunk{hunk("want := 1", "want := 2")}}},
			"test intent.test [vat_test.go#0]",
		},
		{
			"two words changed on one line is no rename",
			[]snapshot.FileDiff{
				{Path: "a.go", Hunks: []snapshot.Hunk{hunk("f(x, y)", "g(x, z)")}},
				{Path: "b.go", Hunks: []snapshot.Hunk{hunk("f(x, y)", "g(x, z)")}},
			},
			"fix intent.rest [a.go#0 b.go#0]",
		},
		{
			"binary files have no hunks to group",
			[]snapshot.FileDiff{{Path: "logo.png", Binary: true}},
			"",
		},
	} {
		if got := render(Groups(tt.files, "fix")); got != tt.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tt.name, got, tt.want)
		}
	}
}

// TestOneHugeHunk: a thousand lines in one hunk stay one hunk, in one group.
func TestOneHugeHunk(t *testing.T) {
	var removed, added []string
	for i := range 1000 {
		removed = append(removed, fmt.Sprintf("line %d", i))
		added = append(added, fmt.Sprintf("line %d changed", i))
	}
	groups := Groups([]snapshot.FileDiff{{Path: "big.go", Hunks: []snapshot.Hunk{{Removed: removed, Added: added}}}}, "fix")
	if len(groups) != 1 || len(groups[0].Hunks) != 1 || groups[0].Key != "intent.rest" {
		t.Errorf("groups = %+v", groups)
	}
}

// TestHunksFromAUnifiedDiff: the snapshot keeps each "@@" block apart.
func TestHunksFromAUnifiedDiff(t *testing.T) {
	diff := "diff --git a/user.go b/user.go\n--- a/user.go\n+++ b/user.go\n" +
		"@@ -3 +3 @@\n-func getUser() {}\n+func fetchUser() {}\n" +
		"@@ -10 +10,2 @@\n-x\n+y\n+z\n"
	files := snapshot.ParseUnifiedDiff(diff)
	if len(files) != 1 || len(files[0].Hunks) != 2 || files[0].Hunks[1].OldStart != 10 || files[0].Hunks[1].NewStart != 10 ||
		strings.Join(files[0].Hunks[1].Added, ",") != "y,z" || len(files[0].Added) != 3 {
		t.Fatalf("files = %+v", files)
	}
}

func FuzzGroups(f *testing.F) {
	f.Add("func getUser() {}", "func fetchUser() {}", "a := getUser()", "a := fetchUser()")
	f.Fuzz(func(t *testing.T, r1, a1, r2, a2 string) {
		files := []snapshot.FileDiff{
			{Path: "a.go", Hunks: []snapshot.Hunk{{Removed: strings.Split(r1, "\n"), Added: strings.Split(a1, "\n")}}},
			{Path: "a_test.go", Hunks: []snapshot.Hunk{{Removed: strings.Split(r2, "\n"), Added: strings.Split(a2, "\n")}}},
		}
		total := 0
		for _, g := range Groups(files, "fix") {
			total += len(g.Hunks)
		}
		if total != 2 {
			t.Fatalf("%d hunks grouped, want 2 (each exactly once)", total)
		}
	})
}
