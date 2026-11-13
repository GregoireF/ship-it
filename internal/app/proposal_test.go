// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/testkit"
)

func TestCheckProposal(t *testing.T) {
	plan := PlanView{Title: "✨ feat(cart): add cart", Branch: "feat/add-cart", CreatesBranch: true}
	onBranch := plan
	onBranch.CreatesBranch = false
	for _, tc := range []struct {
		name          string
		proposal      Proposal
		codes         string // empty: accepted
		title, branch string
		kept          bool
	}{
		{name: "valid title and branch", proposal: Proposal{Title: "fix(cart): handle an empty cart", Branch: "fix/empty-cart", Plan: plan}},
		{name: "valid title alone", proposal: Proposal{Title: "fix(cart): handle an empty cart", Plan: plan}},
		{
			name:     "a bad branch, the title kept",
			proposal: Proposal{Title: "fix(cart): handle an empty cart", Branch: "Empty Cart", Plan: plan},
			codes:    string(fault.InvalidBranch), title: "fix(cart): handle an empty cart", branch: "fix/handle-empty-cart", kept: true,
		},
		{
			name:     "every fault of the title, corrected",
			proposal: Proposal{Title: "Fix: Handle an empty cart.", Plan: plan},
			codes:    "SHIP-E0106 SHIP-E0107", title: "🐛 fix: handle an empty cart", branch: "fix/handle-empty-cart", kept: true,
		},
		{
			name:     "the agent's branch stays when it is valid",
			proposal: Proposal{Title: "fix: Handle an empty cart", Branch: "fix/cart", Plan: plan},
			codes:    "SHIP-E0106", title: "🐛 fix: handle an empty cart", branch: "fix/cart", kept: true,
		},
		{
			name:     "a subject too long falls back on the plan",
			proposal: Proposal{Title: "fix: " + strings.Repeat("a", 51), Plan: plan},
			codes:    string(fault.SubjectTooLong), title: "✨ feat(cart): add cart", branch: "feat/add-cart",
		},
		{name: "no branch is created: the branch does not count", proposal: Proposal{Title: "fix: handle it", Branch: "Not A Branch", Plan: onBranch}},
		{
			name:     "no branch is created: none is proposed",
			proposal: Proposal{Title: "fix: Handle it", Plan: onBranch},
			codes:    "SHIP-E0106", title: "🐛 fix: handle it", kept: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws, _, _ := workspaceOf(testkit.HealthyRepository("main", "cart.go"))
			err := CheckProposal(context.Background(), ws, tc.proposal)
			refused, ok := ProposalIn(err)
			if tc.codes == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if !ok {
				t.Fatalf("err = %v, want a refused proposal", err)
			}
			var codes []string
			for _, f := range refused.Faults {
				codes = append(codes, string(f.Code))
			}
			if got := strings.Join(codes, " "); got != tc.codes {
				t.Errorf("faults = %s, want %s", got, tc.codes)
			}
			if refused.Title != tc.title || refused.Branch != tc.branch || refused.Kept != tc.kept {
				t.Errorf("proposal = %q %q kept %v, want %q %q kept %v", refused.Title, refused.Branch, refused.Kept, tc.title, tc.branch, tc.kept)
			}
		})
	}
}
