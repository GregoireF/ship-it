// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

// Package atomic turns the work in progress into several commits, one per
// intent (#34): which hunks each commit holds, and what to stage for each
// so that the last one leaves exactly the work in the repository.
//
// A modified text file can be split by hunk: an intermediate commit holds
// its HEAD version with the hunks of that commit and of the ones before
// applied (Compose). Any other file (added, deleted, renamed, binary) goes
// whole into one commit. The last commit that holds part of a file stages
// the file as it is in the working tree, so that nothing is lost or
// rewritten whatever Compose makes of line endings.
//
// Allowed imports: standard library without I/O, other core packages.
package atomic

import (
	"slices"
	"strings"

	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/core/intent"
	"github.com/GregoireF/ship-it/internal/core/snapshot"
)

// Whole is the Index of a HunkRef that stands for a whole file, one that
// cannot be split by hunk.
const Whole = intent.MoveHunk

// Stage is what a commit stages for one file.
type Stage struct {
	Path, OriginalPath string
	// Whole stages the file as it is in the working tree (OriginalPath too,
	// for a rename). Otherwise Hunks are the indices of the hunks applied
	// to the HEAD version of the file.
	Whole bool
	Hunks []int
}

// Splittable reports whether the hunks of f can go to different commits: a
// modified text file that keeps its name.
func Splittable(f snapshot.FileDiff) bool {
	return f.Status == snapshot.Modified && !f.Binary && f.OriginalPath == "" && len(f.Hunks) > 0
}

// Assign returns the groups of intent as commits, in their order: the
// hunks of a file that cannot be split go together, as one Whole ref, with
// the first group that holds one of them; the files no group holds (binary
// files) go with the last group. Groups left empty are dropped.
func Assign(files []snapshot.FileDiff, groups []intent.Group) []intent.Group {
	byPath := filesByPath(files)
	placed := map[string]bool{} // whole files already assigned
	var commits []intent.Group
	for _, g := range groups {
		var refs []intent.HunkRef
		for _, ref := range g.Hunks {
			f, ok := byPath[ref.Path]
			switch {
			case !ok:
			case Splittable(f):
				refs = append(refs, ref)
			case !placed[ref.Path]:
				placed[ref.Path] = true
				refs = append(refs, intent.HunkRef{Path: ref.Path, Index: Whole})
			}
		}
		if len(refs) > 0 {
			g.Hunks = refs
			commits = append(commits, g)
		}
	}
	for _, f := range files {
		if Splittable(f) || placed[f.Path] {
			continue
		}
		if len(commits) == 0 {
			commits = append(commits, intent.Group{Key: "intent.rest"})
		}
		last := &commits[len(commits)-1]
		last.Hunks = append(last.Hunks, intent.HunkRef{Path: f.Path, Index: Whole})
	}
	return commits
}

// Stages returns what each commit stages, in the order of files. Each hunk
// of a splittable file, and each other file whole, must be in exactly one
// commit, and no commit may be empty: CommitsInvalid otherwise.
func Stages(files []snapshot.FileDiff, commits [][]intent.HunkRef) ([][]Stage, error) {
	byPath := filesByPath(files)
	owner := map[intent.HunkRef]int{} // unit → its commit
	last := map[string]int{}          // file → the last commit holding part of it
	for k, refs := range commits {
		if len(refs) == 0 {
			return nil, invalid("empty")
		}
		for _, ref := range refs {
			f, ok := byPath[ref.Path]
			valid := ok && (Splittable(f) && ref.Index >= 0 && ref.Index < len(f.Hunks) || !Splittable(f) && ref.Index == Whole)
			if !valid {
				return nil, invalid("unknown", "path", ref.Path)
			}
			if _, twice := owner[ref]; twice {
				return nil, invalid("twice", "path", ref.Path)
			}
			owner[ref] = k
			last[ref.Path] = max(last[ref.Path], k)
		}
	}
	for _, f := range files {
		units := []intent.HunkRef{{Path: f.Path, Index: Whole}}
		if Splittable(f) {
			units = units[:0]
			for i := range f.Hunks {
				units = append(units, intent.HunkRef{Path: f.Path, Index: i})
			}
		}
		for _, u := range units {
			if _, ok := owner[u]; !ok {
				return nil, invalid("missing", "path", f.Path)
			}
		}
	}

	stages := make([][]Stage, len(commits))
	for k := range commits {
		for _, f := range files {
			if !holds(commits[k], f.Path) {
				continue
			}
			if !Splittable(f) || k == last[f.Path] {
				stages[k] = append(stages[k], Stage{Path: f.Path, OriginalPath: f.OriginalPath, Whole: true})
				continue
			}
			var hunks []int
			for i := range f.Hunks {
				if owner[intent.HunkRef{Path: f.Path, Index: i}] <= k {
					hunks = append(hunks, i)
				}
			}
			stages[k] = append(stages[k], Stage{Path: f.Path, Hunks: hunks})
		}
	}
	return stages, nil
}

// Compose applies hunks of a zero-context diff (git diff -U0) to old, the
// HEAD version of a file. The hunks are independent: each one says where
// it starts in old. Added lines take the line ending old uses.
func Compose(old string, hunks []snapshot.Hunk) string {
	eol := "\n"
	if strings.Contains(old, "\r\n") {
		eol = "\r\n"
	}
	lines := strings.SplitAfter(old, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	ordered := slices.Clone(hunks)
	slices.SortStableFunc(ordered, func(a, b snapshot.Hunk) int { return b.OldStart - a.OldStart })
	for _, h := range ordered {
		at := h.OldStart // a pure addition goes after line OldStart
		if len(h.Removed) > 0 {
			at = h.OldStart - 1
		}
		at = min(max(at, 0), len(lines))
		if n := len(lines); at == n && n > 0 && len(h.Added) > 0 && !strings.HasSuffix(lines[n-1], "\n") {
			lines[n-1] += eol // lines added after a last line without an end
		}
		end := min(at+len(h.Removed), len(lines))
		added := make([]string, len(h.Added))
		for i, a := range h.Added {
			added[i] = a + eol
		}
		lines = slices.Concat(lines[:at], added, lines[end:])
	}
	return strings.Join(lines, "")
}

func filesByPath(files []snapshot.FileDiff) map[string]snapshot.FileDiff {
	m := make(map[string]snapshot.FileDiff, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func holds(refs []intent.HunkRef, path string) bool {
	return slices.ContainsFunc(refs, func(r intent.HunkRef) bool { return r.Path == path })
}

func invalid(reason string, params ...string) *fault.Fault {
	return fault.New(fault.CommitsInvalid, "atomic.invalid."+reason, params...)
}
