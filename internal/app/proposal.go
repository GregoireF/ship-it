// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"
	"errors"
)

// Proposal is the title and branch a coding agent proposes for a delivery,
// with the plan they replace.
type Proposal struct {
	Title, Branch string
	Only          []string
	Issue         int
	// Plan is the deterministic plan of the same work: its title is the
	// fallback when the agent's cannot be corrected safely.
	Plan PlanView
}

// ProposalRefusedError refuses the title or branch an agent proposed. It
// carries every rule they break and a title and branch that follow them,
// which the agent can pass back as they are.
type ProposalRefusedError struct {
	Faults        []*Fault
	Title, Branch string
	// Kept reports whether Title keeps the agent's wording (corrected), or
	// is the plan's.
	Kept bool
}

func (e *ProposalRefusedError) Error() string { return e.Unwrap().Error() }

// Unwrap exposes the first fault: its code is the refusal's.
func (e *ProposalRefusedError) Unwrap() error { return e.Faults[0] }

// ProposalIn returns the refusal an error carries, if it refuses an agent's
// proposal.
func ProposalIn(err error) (*ProposalRefusedError, bool) {
	var refused *ProposalRefusedError
	ok := errors.As(err, &refused)
	return refused, ok
}

// CheckProposal checks an agent's title and branch against the repository's
// conventions before anything changes, and returns every rule they break
// with a corrected proposal (ProposalRefusedError). The branch counts only
// when the delivery creates one. The style rules the project's commitlint
// decides are left to it, as for a human's title.
func CheckProposal(ctx context.Context, ws Workspace, p Proposal) error {
	if p.Title == "" && p.Branch == "" {
		return nil
	}
	eff, err := LoadSettingsFor(ctx, ws, p.Only)
	if err != nil {
		return err
	}
	rules := eff.Settings.Conventions()
	var faults []*Fault
	if p.Title != "" {
		faults = rules.ValidateTitle(p.Title)
	}
	// The project's commitlint is read only when it could clear a fault.
	if len(faults) > 0 {
		if lint := projectCommitlint(ctx, ws); lint != nil {
			faults = withoutFaults(faults, givingWayTo(lint, eff))
		}
	}
	titleFault := len(faults) > 0
	branchFault := false
	if p.Branch != "" && p.Plan.CreatesBranch {
		if err := rules.ValidateBranch(p.Branch, eff.Settings.Ship.Issues.TrackerKeys); err != nil {
			faults, branchFault = append(faults, asFault(err)), true
		}
	}
	if len(faults) == 0 {
		return nil
	}

	refused := &ProposalRefusedError{Faults: faults, Title: p.Title, Branch: p.Branch, Kept: true}
	switch {
	case p.Title == "":
		refused.Title, refused.Kept = p.Plan.Title, false
	case titleFault:
		fallback, err := rules.ParseHeader(p.Plan.Title)
		if err != nil {
			refused.Title, refused.Kept = p.Plan.Title, false
			break
		}
		corrected, kept := rules.Correct(p.Title, fallback)
		refused.Title, refused.Kept = rules.Format(corrected, placementWith(ctx, ws, eff, nil)), kept
	}
	if p.Plan.CreatesBranch && (branchFault || p.Branch == "") {
		refused.Branch = p.Plan.Branch
		if name, err := conventionsOf(eff).BranchForTitleOf(refused.Title, BranchRequest{Issue: p.Issue}); err == nil {
			refused.Branch = name
		}
	}
	return refused
}
