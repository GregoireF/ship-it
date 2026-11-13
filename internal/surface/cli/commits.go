// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"slices"
	"strconv"
	"strings"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// commitDraft is one commit of a split by intent as the human arranges it
// (#36), on the review screen or line by line.
type commitDraft struct {
	title  string
	reason app.ReasonView
	hunks  []app.HunkView
}

// commitDrafts are the commits in order. Every change returns a new list,
// and a commit left without hunks goes away.
type commitDrafts []commitDraft

func draftsOf(views []app.CommitView) commitDrafts {
	d := make(commitDrafts, len(views))
	for i, v := range views {
		d[i] = commitDraft{title: v.Title, reason: v.Reason, hunks: slices.Clone(v.Hunks)}
	}
	return d
}

func (d commitDrafts) clone() commitDrafts {
	c := slices.Clone(d)
	for i := range c {
		c[i].hunks = slices.Clone(c[i].hunks)
	}
	return c
}

// moveHunk moves hunk h of commit from to the end of commit to. ok is false
// when an index is out of range or from is to.
func (d commitDrafts) moveHunk(from, h, to int) (commitDrafts, bool) {
	if from == to || from < 0 || from >= len(d) || to < 0 || to >= len(d) || h < 0 || h >= len(d[from].hunks) {
		return d, false
	}
	c := d.clone()
	moved := c[from].hunks[h]
	c[from].hunks = slices.Delete(c[from].hunks, h, h+1)
	c[to].hunks = append(c[to].hunks, moved)
	return c.withoutEmpty(), true
}

// swap exchanges commits i and j.
func (d commitDrafts) swap(i, j int) (commitDrafts, bool) {
	if i == j || i < 0 || j < 0 || i >= len(d) || j >= len(d) {
		return d, false
	}
	c := d.clone()
	c[i], c[j] = c[j], c[i]
	return c, true
}

// merge puts the hunks of commit from into commit into, which keeps its
// title.
func (d commitDrafts) merge(into, from int) (commitDrafts, bool) {
	if into == from || into < 0 || from < 0 || into >= len(d) || from >= len(d) {
		return d, false
	}
	c := d.clone()
	c[into].hunks = append(c[into].hunks, c[from].hunks...)
	c[from].hunks = nil
	return c.withoutEmpty(), true
}

func (d commitDrafts) withoutEmpty() commitDrafts {
	return slices.DeleteFunc(d, func(c commitDraft) bool { return len(c.hunks) == 0 })
}

// specs are the commits to deliver.
func (d commitDrafts) specs() []app.CommitSpec {
	specs := make([]app.CommitSpec, len(d))
	for i, c := range d {
		specs[i].Title = strings.TrimSpace(c.title)
		for _, h := range c.hunks {
			specs[i].Hunks = append(specs[i].Hunks, h.Ref())
		}
	}
	return specs
}

// hunkLabel names a part of the work on one line: the file, where the hunk
// starts and what it changes, or that the file goes whole.
func hunkLabel(lang i18n.Language, h app.HunkView) string {
	if h.Index == app.WholeFile {
		return i18n.Render(lang, "cli.split.whole_file", map[string]string{"path": h.Path})
	}
	at := h.NewStart
	if len(h.Added) == 0 {
		at = h.OldStart
	}
	label := h.Path + ":" + strconv.Itoa(at)
	first := func(lines []string) string {
		if len(lines) == 0 {
			return ""
		}
		return strings.TrimSpace(lines[0])
	}
	switch removed, added := first(h.Removed), first(h.Added); {
	case len(h.Removed) > 0 && len(h.Added) > 0:
		return label + "  " + removed + " → " + added
	case len(h.Added) > 0:
		return label + "  + " + added
	default:
		return label + "  - " + removed
	}
}

// applyChange reads one line of the plain review: "1.2 2" moves change 2
// of commit 1 to commit 2, "up 2" and "down 2" move commit 2, "merge 1 2"
// puts commit 2 into commit 1. Numbers start at 1. ok is false when the
// line is not one of these or names nothing.
func (d commitDrafts) applyChange(line string) (commitDrafts, bool) {
	words := strings.Fields(strings.ToLower(line))
	number := func(s string) int {
		n, err := strconv.Atoi(s)
		if err != nil {
			return -1
		}
		return n - 1
	}
	switch {
	case len(words) == 2 && words[0] == "up":
		i := number(words[1])
		return d.swap(i, i-1)
	case len(words) == 2 && words[0] == "down":
		i := number(words[1])
		return d.swap(i, i+1)
	case len(words) == 3 && words[0] == "merge":
		return d.merge(number(words[1]), number(words[2]))
	case len(words) == 2:
		c, h, found := strings.Cut(words[0], ".")
		if !found {
			return d, false
		}
		return d.moveHunk(number(c), number(h), number(words[1]))
	}
	return d, false
}
