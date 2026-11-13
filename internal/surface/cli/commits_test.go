// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// splitPlan is a work of two intents: a rename in two hunks of cart.go, a
// feature in two new files.
func splitPlan() []app.CommitView {
	return []app.CommitView{
		{Title: "♻️ refactor: rename total to sum", Reason: app.ReasonView{Key: "intent.rename", Params: map[string]string{"from": "total", "to": "sum"}},
			Hunks: []app.HunkView{
				{Path: "cart.go", Index: 0, OldStart: 3, NewStart: 3, Removed: []string{"func total() int {"}, Added: []string{"func sum() int {"}},
				{Path: "cart.go", Index: 1, OldStart: 8, NewStart: 8, Removed: []string{"\treturn total()"}, Added: []string{"\treturn sum()"}},
			}},
		{Title: "✨ feat: add vat", Reason: app.ReasonView{Key: "intent.feature"},
			Hunks: []app.HunkView{{Path: "vat.go", Index: app.WholeFile}, {Path: "vat_test.go", Index: app.WholeFile}}},
	}
}

// layout writes drafts as "title: path#index, …" per commit.
func layout(d commitDrafts) string {
	var parts []string
	for _, c := range d {
		var hunks []string
		for _, h := range c.hunks {
			hunks = append(hunks, fmt.Sprintf("%s#%d", h.Path, h.Index))
		}
		parts = append(parts, c.title+": "+strings.Join(hunks, ", "))
	}
	return strings.Join(parts, " | ")
}

func TestCommitDraftChanges(t *testing.T) {
	start := draftsOf(splitPlan())
	for _, tc := range []struct {
		change string
		want   string // empty: refused, nothing changes
	}{
		{"1.2 2", "♻️ refactor: rename total to sum: cart.go#0 | ✨ feat: add vat: vat.go#-1, vat_test.go#-1, cart.go#1"},
		{"2.1 1", "♻️ refactor: rename total to sum: cart.go#0, cart.go#1, vat.go#-1 | ✨ feat: add vat: vat_test.go#-1"},
		{"down 1", "✨ feat: add vat: vat.go#-1, vat_test.go#-1 | ♻️ refactor: rename total to sum: cart.go#0, cart.go#1"},
		{"UP 2", "✨ feat: add vat: vat.go#-1, vat_test.go#-1 | ♻️ refactor: rename total to sum: cart.go#0, cart.go#1"},
		{"merge 2 1", "✨ feat: add vat: vat.go#-1, vat_test.go#-1, cart.go#0, cart.go#1"},
		{"up 1", ""},
		{"down 2", ""},
		{"1.3 2", ""},
		{"1.1 1", ""},
		{"1.1 3", ""},
		{"merge 1 1", ""},
		{"1 2", ""},
		{"move it", ""},
	} {
		got, ok := start.applyChange(tc.change)
		if tc.want == "" {
			if ok || layout(got) != layout(start) {
				t.Errorf("%q: accepted, %s", tc.change, layout(got))
			}
			continue
		}
		if !ok || layout(got) != tc.want {
			t.Errorf("%q = %s (ok %v)\nwant %s", tc.change, layout(got), ok, tc.want)
		}
	}
	if layout(start) != layout(draftsOf(splitPlan())) {
		t.Error("a change modified the drafts it started from")
	}
}

func TestEmptyCommitGoesAway(t *testing.T) {
	d := draftsOf(splitPlan())
	d, _ = d.applyChange("2.1 1")
	d, _ = d.applyChange("2.1 1")
	if len(d) != 1 || len(d[0].hunks) != 4 {
		t.Errorf("drafts = %s", layout(d))
	}
	specs := d.specs()
	if len(specs) != 1 || specs[0].Hunks[3] != (app.HunkRef{Path: "vat_test.go", Index: app.WholeFile}) {
		t.Errorf("specs = %+v", specs)
	}
}

func TestHunkLabel(t *testing.T) {
	plan := splitPlan()
	cases := []struct {
		hunk app.HunkView
		want string
	}{
		{plan[0].Hunks[0], "cart.go:3  func total() int { → func sum() int {"},
		{plan[1].Hunks[0], "vat.go (whole file)"},
		{app.HunkView{Path: "a.go", Index: 0, NewStart: 7, Added: []string{"  x := 1"}}, "a.go:7  + x := 1"},
		{app.HunkView{Path: "a.go", Index: 1, OldStart: 9, Removed: []string{"y"}}, "a.go:9  - y"},
	}
	for _, c := range cases {
		if got := hunkLabel(i18n.English, c.hunk); got != c.want {
			t.Errorf("hunkLabel = %q, want %q", got, c.want)
		}
	}
}
