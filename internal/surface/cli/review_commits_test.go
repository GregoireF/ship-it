// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// splitReviewPlan is a plan whose work holds two intents (#36).
func splitReviewPlan() app.PlanView {
	p := vatPlan()
	p.Files = []string{"cart.go", "vat.go", "vat_test.go"}
	p.Commits, p.HunksDigest = splitPlan(), "d1"
	return p
}

func key(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func TestReviewCommitsGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		lang i18n.Language
		keys []tea.Msg
	}{
		{"commits", i18n.English, nil},
		{"commits.fr", i18n.French, nil},
		{"commits-moved", i18n.English, []tea.Msg{keyDown, keyDown, key('J')}},
		{"commits-editing", i18n.English, []tea.Msg{keyEnter, keyCtrlU}},
		{"commits-one", i18n.English, []tea.Msg{key('s')}},
		{"commits-help", i18n.English, []tea.Msg{keyHelp}},
		{"commits-help.fr", i18n.French, []tea.Msg{keyHelp}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := send(t, newTestReview(tc.lang, splitReviewPlan(), &replanner{}), tc.keys...)
			got := screen(m)
			assertFits(t, got)
			golden(t, tc.name, got)
		})
	}
}

// arranged is the layout of the commits of an outcome.
func arranged(o reviewOutcome) string {
	var parts []string
	for _, c := range o.commits {
		var hunks []string
		for _, h := range c.Hunks {
			hunks = append(hunks, h.Path+"#"+string(rune('0'+h.Index+1)))
		}
		parts = append(parts, c.Title+": "+strings.Join(hunks, ", "))
	}
	return strings.Join(parts, " | ")
}

func TestReviewCommitsKeys(t *testing.T) {
	rename, feat := "♻️ refactor: rename total to sum", "✨ feat: add vat"
	for _, tc := range []struct {
		name  string
		keys  []tea.Msg
		want  string // the commits shipped
		title string // one commit: its title
	}{
		{"as proposed", []tea.Msg{keyCtrlEnter}, rename + ": cart.go#1, cart.go#2 | " + feat + ": vat.go#0, vat_test.go#0", ""},
		{"a change down", []tea.Msg{keyDown, keyDown, key('J'), keyCtrlEnter},
			rename + ": cart.go#1 | " + feat + ": vat.go#0, vat_test.go#0, cart.go#2", ""},
		{"a change up", []tea.Msg{keyDown, keyDown, keyDown, keyDown, keyShiftUp, keyCtrlEnter},
			rename + ": cart.go#1, cart.go#2, vat.go#0 | " + feat + ": vat_test.go#0", ""},
		{"a change to commit 2", []tea.Msg{keyDown, key('2'), keyCtrlEnter},
			rename + ": cart.go#2 | " + feat + ": vat.go#0, vat_test.go#0, cart.go#1", ""},
		{"a commit down", []tea.Msg{keyShiftDown, keyCtrlEnter},
			feat + ": vat.go#0, vat_test.go#0 | " + rename + ": cart.go#1, cart.go#2", ""},
		{"no commit 9", []tea.Msg{keyDown, key('9'), keyCtrlEnter},
			rename + ": cart.go#1, cart.go#2 | " + feat + ": vat.go#0, vat_test.go#0", ""},

		{"merged into one", []tea.Msg{key('m'), keyCtrlEnter}, "", rename},
		{"one commit", []tea.Msg{key('s'), keyEnter}, "", "✨ feat(cart): add vat rate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := send(t, newTestReview(i18n.English, splitReviewPlan(), &replanner{}), tc.keys...)

			if !m.done || m.outcome.action != reviewShip {
				t.Fatalf("not shipped: %+v\n%s", m.outcome, screen(m))
			}
			if got := arranged(m.outcome); got != tc.want {
				t.Errorf("commits = %s\nwant      %s", got, tc.want)
			}
			if tc.title != "" && m.outcome.title != tc.title {
				t.Errorf("title = %q, want %q", m.outcome.title, tc.title)
			}
			if tc.want != "" && m.outcome.hunksDigest != "d1" {
				t.Errorf("digest = %q", m.outcome.hunksDigest)
			}
		})
	}
}

func TestReviewEditsACommitTitle(t *testing.T) {
	keys := append([]tea.Msg{keyEnter, keyCtrlU}, typed("♻️ refactor(cart): rename total")...)
	m := send(t, newTestReview(i18n.English, splitReviewPlan(), &replanner{}), append(keys, keyEnter)...)
	if m.editing >= 0 || m.drafts[0].title != "♻️ refactor(cart): rename total" {
		t.Fatalf("editing %d, title %q", m.editing, m.drafts[0].title)
	}
	// Enter on a commit edits it again; on a change, it ships.
	m = send(t, m, keyDown, keyEnter)
	if !m.done || m.outcome.commits[0].Title != "♻️ refactor(cart): rename total" {
		t.Fatalf("outcome = %+v", m.outcome)
	}
}

func TestReviewBlocksAnInvalidCommitTitle(t *testing.T) {
	keys := append([]tea.Msg{keyDown, keyDown, keyDown, keyEnter, keyCtrlU}, typed("Add VAT.")...)
	m := send(t, newTestReview(i18n.English, splitReviewPlan(), &replanner{}), append(keys, keyEnter, keyUp, keyUp, keyUp, keyCtrlEnter)...)
	if m.done || !strings.Contains(screen(m), "Fix the title of commit 2 first") || m.commitRow != 3 {
		t.Fatalf("done %v, row %d\n%s", m.done, m.commitRow, screen(m))
	}
}

func TestReviewMergeOfTheLastCommitSaysWhy(t *testing.T) {
	m := send(t, newTestReview(i18n.English, splitReviewPlan(), &replanner{}), keyDown, keyDown, keyDown, key('m'))
	if !strings.Contains(screen(m), "no commit below it") || len(m.drafts) != 2 {
		t.Errorf("drafts %d\n%s", len(m.drafts), screen(m))
	}
}

func TestReviewNewSelectionDropsTheSplit(t *testing.T) {
	rp := &replanner{plan: func(only []string) app.PlanView {
		p := vatPlan()
		p.Files = []string{"cart.go", "vat.go", "vat_test.go"}
		if only == nil {
			p.Commits, p.HunksDigest = splitPlan(), "d2"
		}
		return p
	}}
	m := newTestReview(i18n.English, splitReviewPlan(), rp)
	m = send(t, m, keyTab, keyTab, keyTab, keyTab) // to the files
	if m.focus != fieldFiles {
		t.Fatalf("focus %v", m.focus)
	}
	m = send(t, m, keyDown, keySpace)
	if m.drafts != nil || m.splitting() || strings.Contains(screen(m), "Commits") {
		t.Fatalf("the split stayed for a part of the work:\n%s", screen(m))
	}
	m = send(t, m, keySpace)
	if !m.splitting() || m.plan.HunksDigest != "d2" {
		t.Fatalf("the whole work is split again: %v\n%s", m.splitting(), screen(m))
	}
}
