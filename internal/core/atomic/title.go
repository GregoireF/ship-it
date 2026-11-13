// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package atomic

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"

	"github.com/GregoireF/ship-it/internal/core/intent"
	"github.com/GregoireF/ship-it/internal/core/snapshot"
)

// Subject is the subject of the commit of g when its intent says what it
// does (a rename, a move, formatting); otherwise planned, what the planner
// proposes for the changes of g.
func Subject(g intent.Group, planned string) string {
	switch g.Key {
	case "intent.rename":
		return "rename " + g.Params["from"] + " to " + g.Params["to"]
	case "intent.move":
		return "move files"
	case "intent.format":
		return "format code"
	}
	return planned
}

// Restrict returns the files as one commit changes them: a splittable file
// with only the hunks of refs (its added and removed lines recounted), any
// other file whole when refs hold it.
func Restrict(files []snapshot.FileDiff, refs []intent.HunkRef) []snapshot.FileDiff {
	var out []snapshot.FileDiff
	for _, f := range files {
		if !holds(refs, f.Path) {
			continue
		}
		if !Splittable(f) {
			out = append(out, f)
			continue
		}
		part := snapshot.FileDiff{Path: f.Path, Status: f.Status}
		for i, h := range f.Hunks {
			if !slices.Contains(refs, intent.HunkRef{Path: f.Path, Index: i}) {
				continue
			}
			part.Hunks = append(part.Hunks, h)
			part.Removed = append(part.Removed, h.Removed...)
			for j, line := range h.Added {
				part.Added = append(part.Added, snapshot.Line{Number: h.NewStart + j, Text: line})
			}
		}
		out = append(out, part)
	}
	return out
}

// Digest identifies the hunks of files: when the work changes between the
// review and the delivery, the hunks a human placed may no longer be the
// ones the delivery reads.
func Digest(files []snapshot.FileDiff) string {
	h := sha256.New()
	write := func(s string) { h.Write([]byte(s)); h.Write([]byte{0}) }
	for _, f := range files {
		write(f.Path)
		write(f.OriginalPath)
		write(strconv.Itoa(int(f.Status)))
		write(strconv.FormatBool(f.Binary))
		for _, k := range f.Hunks {
			write(strconv.Itoa(k.OldStart) + "," + strconv.Itoa(k.NewStart))
			for _, l := range k.Removed {
				write("-" + l)
			}
			for _, l := range k.Added {
				write("+" + l)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
