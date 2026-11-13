// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package retention

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func at(id string, daysAgo int, hour int) Item {
	day := now.AddDate(0, 0, -daysAgo)
	return Item{ID: id, CreatedAt: time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, time.UTC)}
}

func TestExpired(t *testing.T) {
	items := []Item{ // newest first
		at("today-2", 0, 11), at("today-1", 0, 9),
		at("d1-late", 1, 18), at("d1-early", 1, 8),
		at("d40", 40, 10),
		at("d50-protected", 50, 10),
	}
	policy := Policy{KeepLatest: 1, DailyWindow: 30 * 24 * time.Hour}
	got := Expired(items, now, policy, map[string]bool{"d50-protected": true})
	// today-2 is the latest; today-1 shares its day; d1-late is that day's
	// newest; d40 is outside the window.
	want := []string{"today-1", "d1-early", "d40"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expired = %v, want %v", got, want)
	}
}

// TestWatchCheckpointsHaveTheirOwnLimit: #38. A long watch never pushes
// the checkpoints of operations out; its own are kept 48 hours, 200 at most.
func TestWatchCheckpointsHaveTheirOwnLimit(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	policy := Policy{KeepLatest: 2, KeepWatchFor: 48 * time.Hour, KeepWatchMax: 3}
	items := []Item{
		{ID: "w1", CreatedAt: now.Add(-1 * time.Minute), Watch: true},
		{ID: "w2", CreatedAt: now.Add(-2 * time.Minute), Watch: true},
		{ID: "op1", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "w3", CreatedAt: now.Add(-4 * time.Minute), Watch: true},
		{ID: "w4", CreatedAt: now.Add(-5 * time.Minute), Watch: true}, // beyond KeepWatchMax
		{ID: "op2", CreatedAt: now.Add(-6 * time.Minute)},             // still among the 2 operations
		{ID: "w5", CreatedAt: now.Add(-72 * time.Hour), Watch: true},  // older than 48 hours
		{ID: "op3", CreatedAt: now.Add(-73 * time.Hour)},              // beyond KeepLatest, no daily window
	}
	got := Expired(items, now, policy, nil)
	if strings.Join(got, ",") != "w4,w5,op3" {
		t.Errorf("Expired = %v, want [w4 w5 op3]", got)
	}
}

func TestDefaultPolicyKeepsRecentWork(t *testing.T) {
	var items []Item
	for i := range 20 {
		items = append(items, at("old", 100, i))
	}
	if got := Expired(items, now, DefaultPolicy, nil); len(got) != 0 {
		t.Fatalf("the 20 latest checkpoints must survive whatever their age, expired %d", len(got))
	}
}
