// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

// Package watch decides when ship watch takes a checkpoint (#38): after a
// pause in the edits, never while files keep changing, and never when
// nothing changed. It sees the working tree only as a fingerprint that
// changes when a file does.
//
// Allowed imports: standard library without I/O, other core packages.
package watch

import "time"

// Label is the label of the checkpoints ship watch takes.
const Label = "watch"

// Pauser follows the fingerprints of the working tree.
type Pauser struct {
	// Pause is how long the files must stay still before a checkpoint.
	Pause time.Duration

	seen       bool
	last       string
	changedAt  time.Time
	pending    bool
	checkpoint string // the fingerprint of the last checkpoint taken
}

// Observe reads one fingerprint taken at now and reports whether to take a
// checkpoint. The first fingerprint is the starting point: the files as
// they are when watching starts are not a change.
func (p *Pauser) Observe(fingerprint string, now time.Time) bool {
	if !p.seen {
		p.seen, p.last, p.checkpoint = true, fingerprint, fingerprint
		return false
	}
	if fingerprint != p.last {
		p.last, p.changedAt, p.pending = fingerprint, now, true
		return false
	}
	if !p.pending || now.Sub(p.changedAt) < p.Pause {
		return false
	}
	p.pending = false
	if fingerprint == p.checkpoint {
		return false // edited, then put back as it was
	}
	p.checkpoint = fingerprint
	return true
}
