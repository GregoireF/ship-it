// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

// Package retention decides which checkpoints `ship gc` may remove.
//
// The rule comes from ADR-0005: keep the N most recent checkpoints, plus the
// newest one of each day over a window, plus anything an undo still needs.
// Days are UTC days (docs/design/undo-storage-optimizations.md, M10).
//
// Allowed imports: standard library without I/O, other core packages.
package retention

import "time"

// Policy is how many checkpoints survive a garbage collection.
type Policy struct {
	// KeepLatest checkpoints are always kept, whatever their age.
	KeepLatest int
	// DailyWindow keeps the newest checkpoint of each day younger than this.
	DailyWindow time.Duration
	// Watch checkpoints (ship watch, #38) do not count in KeepLatest, so
	// that an hour of watching never pushes the checkpoints of operations
	// out: the KeepWatchMax newest of them younger than KeepWatchFor stay.
	KeepWatchFor time.Duration
	KeepWatchMax int
}

// DefaultPolicy follows ADR-0005. The values are defaults meant to be
// recalibrated after the storage benchmark (U1b).
var DefaultPolicy = Policy{KeepLatest: 20, DailyWindow: 30 * 24 * time.Hour, KeepWatchFor: 48 * time.Hour, KeepWatchMax: 200}

// Item is a checkpoint as the policy sees it.
type Item struct {
	ID        string
	CreatedAt time.Time
	// Watch: taken by ship watch.
	Watch bool
}

// Expired returns the ids to remove. items are ordered newest first;
// protected ids are always kept.
func Expired(items []Item, now time.Time, policy Policy, protected map[string]bool) []string {
	var expired []string
	keptDays := map[string]bool{}
	operations, watched := 0, 0
	for _, item := range items {
		day := item.CreatedAt.UTC().Format(time.DateOnly)
		recent := false
		if item.Watch {
			recent = watched < policy.KeepWatchMax && now.Sub(item.CreatedAt) <= policy.KeepWatchFor
			watched++
		} else {
			recent = operations < policy.KeepLatest
			operations++
		}
		switch {
		case recent, protected[item.ID]:
			keptDays[day] = true
		case now.Sub(item.CreatedAt) <= policy.DailyWindow && !keptDays[day]:
			keptDays[day] = true
		default:
			expired = append(expired, item.ID)
		}
	}
	return expired
}
