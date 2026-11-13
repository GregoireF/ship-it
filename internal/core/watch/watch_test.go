// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package watch

import (
	"testing"
	"time"
)

func TestPauser(t *testing.T) {
	start := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return start.Add(time.Duration(s) * time.Second) }
	p := &Pauser{Pause: 10 * time.Second}
	steps := []struct {
		second int
		print  string
		take   bool
	}{
		{0, "a", false},  // the starting point
		{2, "a", false},  // nothing changes: no checkpoint, ever
		{30, "a", false}, //
		{32, "b", false}, // an edit
		{34, "c", false}, // still editing
		{40, "c", false}, // 6 s of quiet only
		{44, "c", true},  // 10 s: a checkpoint
		{60, "c", false}, // and only one
		{62, "d", false}, // edited...
		{64, "c", false}, // ...and put back
		{80, "c", false}, // same as the last checkpoint: none
	}
	for _, s := range steps {
		if got := p.Observe(s.print, at(s.second)); got != s.take {
			t.Errorf("at %ds with %q: take = %v, want %v", s.second, s.print, got, s.take)
		}
	}
}
