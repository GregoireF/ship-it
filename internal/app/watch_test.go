// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"
	"testing"
	"time"

	"github.com/GregoireF/ship-it/internal/testkit"
)

// TestWatchTakesACheckpointAfterAPause: #38. Nothing while nothing
// changes, one checkpoint once the edits pause, labelled "watch".
func TestWatchTakesACheckpointAfterAPause(t *testing.T) {
	prints := []string{"a", "a", "b", "c"} // then "c" from then on
	git := testkit.HealthyRepository("main")
	git.Fingerprints = &prints
	ws, store, _ := workspaceOf(git)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	var seen []CheckpointView
	taken, err := Watch(ctx, ws, WatchOptions{Interval: 5 * time.Millisecond, Pause: 40 * time.Millisecond,
		Taken: func(v CheckpointView) { seen = append(seen, v) }})
	if err != nil || taken != 1 || len(seen) != 1 || len(store.Checkpoints) != 1 || store.Checkpoints[0].Label != "watch" {
		t.Fatalf("taken %d, %v, seen %+v, store %+v", taken, err, seen, store.Checkpoints)
	}
}
