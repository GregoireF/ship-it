// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"
	"testing"

	"github.com/GregoireF/ship-it/internal/testkit"
)

// TestAnOrganisationEnforcesAgentRules: #48. A policy sets ship.agents for
// every repository; a repository that sets them too is refused.
func TestAnOrganisationEnforcesAgentRules(t *testing.T) {
	ws, _, _ := workspaceOf(testkit.HealthyRepository("main"))
	files := &testkit.FakeConfigFiles{Policy: `{"organization":"Acme","enforce":{"ship":{"agents":{"*":{"protectedPaths":["secrets/**"]}}}}}`}
	ws.Config = files
	ws = WithAgent(ws, "claude")
	ws.Agent = true
	if _, _, ok := AgentRules(context.Background(), ws).Protected([]string{"secrets/prod.env"}); !ok {
		t.Error("the policy's protected path must apply")
	}
	files.Repository = `{"ship": {"agents": {"claude": {"tools": ["ship_publish"]}}}}`
	if _, err := LoadSettings(context.Background(), ws); !IsPolicyRefusal(err) {
		t.Errorf("a repository overriding the policy: %v", err)
	}
	ws.Agent = false
	if rules := AgentRules(context.Background(), ws); len(rules.ProtectedPaths) != 0 {
		t.Error("no rules apply without an agent (a human, or --attended)")
	}
}
