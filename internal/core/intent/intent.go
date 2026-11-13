// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

// Package intent groups the hunks of a change by what they do (#35), so
// that ship can propose one commit per intent: a rename, formatting, a new
// feature with its tests, the rest. It reads the diff only, never a
// language's syntax: a rename is the same word replaced by another in
// several places; formatting is a hunk equal to what it replaces but for
// spaces.
//
// Each hunk is classified by the first rule that applies: rename, format,
// test (with the code it tests), feature (it declares something new), the
// rest.
//
// Allowed imports: standard library without I/O, other core packages.
package intent

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/GregoireF/ship-it/internal/core/plan"
	"github.com/GregoireF/ship-it/internal/core/snapshot"
	"github.com/GregoireF/ship-it/internal/core/split"
)

// MoveHunk is the Index of a HunkRef that stands for a file's rename (the
// move itself), apart from its edits.
const MoveHunk = -1

// HunkRef names one hunk: a file and its index in FileDiff.Hunks, or
// MoveHunk for the move of a renamed file.
type HunkRef struct {
	Path  string `json:"path"`
	Index int    `json:"index"`
}

// Group is one intent: a commit type, a reason key with its parameters
// (internal/i18n renders it), and its hunks in diff order.
type Group struct {
	Type   string            `json:"type"`
	Key    string            `json:"key"`
	Params map[string]string `json:"params,omitempty"`
	Hunks  []HunkRef         `json:"hunks"`
}

// Groups classifies every hunk of files, in this order: renames (one group
// per word pair, and one for files moved), formatting, features, the rest;
// a test hunk joins the group of the code it tests. The rest takes
// restType, the type the plan proposes for the whole work. A word pair seen
// in a single hunk is no rename: it stays with the rest.
func Groups(files []snapshot.FileDiff, restType string) []Group {
	type classified struct {
		ref   HunkRef
		kind  string // "rename", "format", "test", "feature", "rest"
		pair  [2]string
		path  string
		tests string // the file a test hunk tests
		hunk  snapshot.Hunk
	}
	// after classifies a hunk by the rules that follow rename and format.
	after := func(c *classified) {
		switch {
		case split.Subject(c.path) != "":
			c.kind, c.tests = "test", split.Subject(c.path)
		case declares(c.hunk):
			c.kind = "feature"
		default:
			c.kind = "rest"
		}
	}
	var hunks []classified
	pairs := map[[2]string]int{}
	var moved []HunkRef
	for _, f := range files {
		if f.Binary {
			continue
		}
		if f.Status == snapshot.Renamed && f.OriginalPath != "" {
			moved = append(moved, HunkRef{Path: f.Path, Index: MoveHunk})
		}
		for i, h := range f.Hunks {
			c := classified{ref: HunkRef{Path: f.Path, Index: i}, path: f.Path, hunk: h}
			switch pair, ok := renamePair(h); {
			case ok:
				c.kind, c.pair = "rename", pair
				pairs[pair]++
			case formatOnly(h):
				c.kind = "format"
			default:
				after(&c)
			}
			hunks = append(hunks, c)
		}
	}
	// A pair seen in one hunk only is an edit, not a rename.
	for i := range hunks {
		if hunks[i].kind == "rename" && pairs[hunks[i].pair] < 2 {
			after(&hunks[i])
		}
	}

	var groups []Group
	index := map[string]int{} // group id → position in groups
	add := func(id string, g Group, ref HunkRef) {
		at, ok := index[id]
		if !ok {
			at = len(groups)
			index[id] = at
			groups = append(groups, g)
		}
		groups[at].Hunks = append(groups[at].Hunks, ref)
	}
	if len(moved) > 0 {
		groups = append(groups, Group{Type: "refactor", Key: "intent.move", Params: map[string]string{"count": strconv.Itoa(len(moved))}, Hunks: moved})
		index["move"] = 0
	}
	ids := map[string]string{} // path → id of the group holding its code
	for _, c := range hunks {
		switch c.kind {
		case "rename":
			id := "rename\x00" + c.pair[0] + "\x00" + c.pair[1]
			add(id, Group{Type: "refactor", Key: "intent.rename", Params: map[string]string{"from": c.pair[0], "to": c.pair[1]}}, c.ref)
		case "format":
			add("format", Group{Type: "style", Key: "intent.format"}, c.ref)
		case "feature":
			add("feature", Group{Type: "feat", Key: "intent.feature"}, c.ref)
			ids[c.path] = "feature"
		case "rest":
			add("rest", Group{Type: restType, Key: "intent.rest"}, c.ref)
			if _, ok := ids[c.path]; !ok {
				ids[c.path] = "rest"
			}
		}
	}
	// Tests go with the code they test, or in a group of their own.
	for _, c := range hunks {
		if c.kind != "test" {
			continue
		}
		if id, ok := ids[c.tests]; ok {
			add(id, Group{}, c.ref)
			continue
		}
		add("test", Group{Type: "test", Key: "intent.test"}, c.ref)
	}
	return groups
}

// word is an identifier, in most languages.
var word = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// renamePair reports the one word a hunk replaces by another everywhere it
// changes a line: as many lines removed as added, each pair equal once the
// word is replaced, and at least one line changed.
func renamePair(h snapshot.Hunk) ([2]string, bool) {
	if len(h.Removed) == 0 || len(h.Removed) != len(h.Added) {
		return [2]string{}, false
	}
	var pair [2]string
	changed := false
	for i, old := range h.Removed {
		now := h.Added[i]
		if old == now {
			continue
		}
		oldWords, nowWords := word.FindAllStringIndex(old, -1), word.FindAllStringIndex(now, -1)
		if len(oldWords) != len(nowWords) || word.ReplaceAllString(old, "w") != word.ReplaceAllString(now, "w") {
			return [2]string{}, false // the line changed beyond its words
		}
		for j := range oldWords {
			a, b := old[oldWords[j][0]:oldWords[j][1]], now[nowWords[j][0]:nowWords[j][1]]
			if a == b {
				continue
			}
			switch {
			case !changed && pair == [2]string{}:
				pair = [2]string{a, b}
			case pair != [2]string{a, b}:
				return [2]string{}, false // two different words changed
			}
		}
		changed = true
	}
	return pair, changed && pair != [2]string{}
}

// formatOnly reports a hunk that changes nothing but spaces and line breaks.
func formatOnly(h snapshot.Hunk) bool {
	strip := func(lines []string) string {
		return strings.Join(strings.Fields(strings.Join(lines, " ")), "")
	}
	return (len(h.Removed) > 0 || len(h.Added) > 0) && strip(h.Removed) == strip(h.Added) &&
		!slices.Equal(h.Removed, h.Added)
}

// declares reports a hunk that adds a declaration no removed line had.
func declares(h snapshot.Hunk) bool {
	before := map[string]bool{}
	for _, l := range h.Removed {
		if name := plan.DeclaredName(l); name != "" {
			before[name] = true
		}
	}
	for _, l := range h.Added {
		if name := plan.DeclaredName(l); name != "" && !before[name] {
			return true
		}
	}
	return false
}
