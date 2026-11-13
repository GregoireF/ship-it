// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"

	"github.com/GregoireF/ship-it/internal/core/atomic"
	"github.com/GregoireF/ship-it/internal/core/convention"
	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/core/intent"
	"github.com/GregoireF/ship-it/internal/core/plan"
	"github.com/GregoireF/ship-it/internal/core/snapshot"
	"github.com/GregoireF/ship-it/internal/ports"
)

// HunkRef names one part of the work: a hunk of a file, or a whole file
// (WholeFile).
type HunkRef = intent.HunkRef

// WholeFile is the Index of a HunkRef that stands for a whole file: one
// that cannot be split by hunk (added, deleted, renamed or binary).
const WholeFile = atomic.Whole

// CommitView is one commit of a delivery split by intent, as the review
// shows it (#34).
type CommitView struct {
	Title string `json:"title"`
	// Reason says what the commit does (intent.* keys).
	Reason ReasonView `json:"reason"`
	Hunks  []HunkView `json:"hunks"`
}

// HunkView is one part of the work a human can move between commits.
type HunkView struct {
	Path string `json:"path"`
	// Index is the hunk's position in its file, WholeFile for a whole file.
	Index int `json:"index"`
	// OldStart and NewStart are where the hunk starts in each version.
	OldStart int      `json:"old_start,omitempty"`
	NewStart int      `json:"new_start,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Added    []string `json:"added,omitempty"`
}

// Ref names the part.
func (h HunkView) Ref() HunkRef { return HunkRef{Path: h.Path, Index: h.Index} }

// CommitSpec is one commit of a delivery split by intent, as reviewed.
type CommitSpec struct {
	Title string
	Hunks []HunkRef
}

// CommittedView is one commit a delivery made.
type CommittedView struct {
	Commit string `json:"commit"`
	Title  string `json:"title"`
}

// atomicReadLimit bounds the HEAD version of a file split by hunk.
const atomicReadLimit = 64 << 20

// atomicWork is the work in progress without context lines: each hunk can
// be committed apart. An untracked text file is one hunk, so that its
// intent can be read.
func atomicWork(ctx context.Context, git ports.Git, status ports.WorkingTreeStatus) ([]snapshot.FileDiff, error) {
	raw, err := git.ZeroContextDiff(ctx)
	if err != nil {
		return nil, err
	}
	files := snapshot.ParseUnifiedDiff(raw)
	for _, f := range status.Files {
		if f.State != ports.Untracked {
			continue
		}
		d := newFileDiff(ctx, git, f.Path)
		if !d.Binary && len(d.Added) > 0 {
			h := snapshot.Hunk{NewStart: 1}
			for _, l := range d.Added {
				h.Added = append(h.Added, l.Text)
			}
			d.Hunks = []snapshot.Hunk{h}
		}
		files = append(files, d)
	}
	return files, nil
}

// proposeCommits splits the work by intent; nil when it holds one intent
// only. Each commit gets the title the planner proposes for its changes,
// with the type and, when it tells, the subject of its intent.
func proposeCommits(files []snapshot.FileDiff, planned convention.Header, opts plan.Options, generated map[string]bool, placement convention.GitmojiPlacement) []CommitView {
	groups := atomic.Assign(files, intent.Groups(files, string(planned.Type)))
	if len(groups) < 2 {
		return nil
	}
	byPath := map[string]snapshot.FileDiff{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	rules := opts.Rules
	commits := make([]CommitView, 0, len(groups))
	for _, g := range groups {
		part := atomic.Restrict(files, g.Hunks)
		header := planned
		if p, err := plan.Propose(snapshot.New(part, snapshot.Options{GeneratedPaths: generated}), opts); err == nil {
			header = p.Header
		}
		subject := header.Subject
		if t, ok := rules.LookupCommitType(convention.CommitType(g.Type)); ok {
			header.Type = t.Type
		} else if g.Type != "" {
			header.Type, _ = rules.Closest(convention.CommitType(g.Type))
		}
		header.Gitmoji, header.Breaking = "", false
		header.Subject = atomic.Subject(g, subject)
		header = rules.FitToMaxLength(header)
		if len(rules.Validate(header)) > 0 {
			header.Subject = subject // a rename of long names: the planner's subject
		}
		view := CommitView{Title: rules.Format(header, placement), Reason: ReasonView{Key: g.Key, Params: g.Params}}
		for _, ref := range g.Hunks {
			h := HunkView{Path: ref.Path, Index: ref.Index}
			if f := byPath[ref.Path]; ref.Index >= 0 && ref.Index < len(f.Hunks) {
				k := f.Hunks[ref.Index]
				h.OldStart, h.NewStart, h.Removed, h.Added = k.OldStart, k.NewStart, k.Removed, k.Added
			}
			view.Hunks = append(view.Hunks, h)
		}
		commits = append(commits, view)
	}
	return commits
}

// atomicStages checks the reviewed commits against the work as it is now,
// before anything changes: the work must be the one reviewed (digest), and
// the commits must hold each of its parts once.
func atomicStages(ctx context.Context, ws Workspace, status ports.WorkingTreeStatus, req DeliveryRequest) ([]snapshot.FileDiff, [][]atomic.Stage, error) {
	files, err := atomicWork(ctx, ws.Git, status)
	if err != nil {
		return nil, nil, err
	}
	if atomic.Digest(files) != req.HunksDigest {
		return nil, nil, fault.New(fault.CommitsInvalid, "atomic.invalid.changed")
	}
	refs := make([][]HunkRef, len(req.Commits))
	for i, c := range req.Commits {
		refs[i] = c.Hunks
	}
	stages, err := atomic.Stages(files, refs)
	return files, stages, err
}

// commitAtomically makes one commit per reviewed commit, from head: the
// index starts from head, then each commit stages its parts (a file split
// by hunk as head's version with the hunks so far, a whole file as it is
// in the working tree). The working tree is never touched.
func commitAtomically(ctx context.Context, ws Workspace, head string, files []snapshot.FileDiff, stages [][]atomic.Stage, messages []string, opts ports.CommitOptions) ([]CommittedView, error) {
	byPath := map[string]snapshot.FileDiff{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	if err := ws.Git.ResetIndex(ctx, head); err != nil {
		return nil, err
	}
	var done []CommittedView
	for k, commit := range stages {
		var whole []string
		for _, s := range commit {
			if s.Whole {
				whole = append(whole, s.Path)
				if s.OriginalPath != "" {
					whole = append(whole, s.OriginalPath)
				}
				continue
			}
			old, err := ws.Git.ReadFileAt(ctx, head, s.Path, atomicReadLimit)
			if err != nil {
				return done, err
			}
			f := byPath[s.Path]
			hunks := make([]snapshot.Hunk, 0, len(s.Hunks))
			for _, i := range s.Hunks {
				hunks = append(hunks, f.Hunks[i])
			}
			if err := ws.Git.StageContent(ctx, s.Path, []byte(atomic.Compose(string(old), hunks))); err != nil {
				return done, err
			}
		}
		if len(whole) > 0 {
			if err := ws.Git.StagePaths(ctx, whole); err != nil {
				return done, err
			}
		}
		id, err := ws.Git.Commit(ctx, messages[k], opts)
		if err != nil {
			return done, err
		}
		done = append(done, CommittedView{Commit: id, Title: firstLine(messages[k])})
	}
	return done, nil
}
