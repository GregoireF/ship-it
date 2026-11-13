// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"
	"time"

	"github.com/GregoireF/ship-it/internal/core/config"
	"github.com/GregoireF/ship-it/internal/core/watch"
)

// WatchInterval is how often ship watch looks at the files.
const WatchInterval = 2 * time.Second

// WatchOptions are how ship watch runs; zero values take the settings.
type WatchOptions struct {
	Interval time.Duration
	Pause    time.Duration
	// Taken is told of each checkpoint, as it is taken.
	Taken func(CheckpointView)
}

// WatchPause is the pause ship watch waits for (ship.watch.pauseSeconds).
func WatchPause(ctx context.Context, ws Workspace) time.Duration {
	seconds := config.DefaultWatchPauseSeconds
	if eff, err := LoadSettings(ctx, ws); err == nil {
		seconds = eff.Settings.Ship.Watch.PauseSeconds
	}
	return time.Duration(seconds) * time.Second
}

// Watch takes a checkpoint after each pause in the edits until ctx ends
// (#38), and returns how many it took. It only reads the files' metadata
// between checkpoints: nothing is written while nothing changes. A failed
// look is tried again at the next tick; a failed checkpoint stops it.
func Watch(ctx context.Context, ws Workspace, opts WatchOptions) (int, error) {
	if _, err := ws.Git.TopLevel(ctx); err != nil {
		return 0, err
	}
	if opts.Interval <= 0 {
		opts.Interval = WatchInterval
	}
	if opts.Pause <= 0 {
		opts.Pause = WatchPause(ctx, ws)
	}
	pauser := &watch.Pauser{Pause: opts.Pause}
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()
	taken := 0
	for {
		if fingerprint, err := ws.Git.WorkingFingerprint(ctx); err == nil && pauser.Observe(fingerprint, time.Now()) {
			cp, err := ws.Checkpoints.Create(ctx, watch.Label)
			if err != nil {
				if ctx.Err() != nil {
					return taken, nil // Ctrl+C while it was being taken
				}
				return taken, err
			}
			taken++
			if opts.Taken != nil {
				opts.Taken(viewOf(cp))
			}
		}
		select {
		case <-ctx.Done():
			return taken, nil
		case <-ticker.C:
		}
	}
}
