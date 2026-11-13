// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package app

import (
	"context"

	"github.com/GregoireF/ship-it/internal/core/config"
	"github.com/GregoireF/ship-it/internal/core/fault"
)

// AgentRules are the rules of ship.agents for the agent of ws (#48); none
// when no agent drives it, or when the settings cannot be read (the
// delivery then fails on them anyway).
func AgentRules(ctx context.Context, ws Workspace) config.AgentSettings {
	if !ws.Agent {
		return config.AgentSettings{}
	}
	eff, err := LoadSettings(ctx, ws)
	if err != nil {
		return config.AgentSettings{}
	}
	return eff.Settings.Ship.Agents.For(ws.AgentName)
}

// AgentToolRefused is the refusal of a tool ship.agents does not allow the
// agent, nil when it may call it.
func AgentToolRefused(ctx context.Context, ws Workspace, tool string) error {
	if AgentRules(ctx, ws).Allows(tool) {
		return nil
	}
	return fault.New(fault.AgentToolNotAllowed, "agents.tool_not_allowed", "tool", tool, "agent", ws.AgentName)
}

// checkAgentPaths refuses work that touches a path ship.agents protects
// from the agent: a human delivers it.
func checkAgentPaths(ctx context.Context, ws Workspace, paths []string) error {
	if path, pattern, ok := AgentRules(ctx, ws).Protected(paths); ok {
		return fault.New(fault.AgentPathProtected, "agents.path_protected", "path", path, "pattern", pattern, "agent", ws.AgentName)
	}
	return nil
}
