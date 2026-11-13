// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package config

import (
	"regexp"
	"slices"

	"github.com/GregoireF/ship-it/internal/core/fault"
	"github.com/GregoireF/ship-it/internal/core/glob"
)

// AnyAgent is the table of ship.agents that applies to every agent.
const AnyAgent = "*"

// AgentSettings are the rules of one coding agent working through ship mcp
// (#48). The agent is named by `ship mcp install --client`, or by itself
// when it connects; one that gives no name follows "*".
type AgentSettings struct {
	// Tools are the ship_* tools it may call; empty allows them all.
	Tools []string `json:"tools,omitempty"`
	// ProtectedPaths are glob patterns it may not deliver: ship_deliver and
	// ship_stack refuse work that touches them; a human delivers it.
	ProtectedPaths []string `json:"protectedPaths,omitempty"`
}

// Agents are the tables of ship.agents, by agent name, "*" for every agent.
type Agents map[string]AgentSettings

var _ mapSetting = Agents{}

// For returns the rules of one agent: its own table over "*". Its own tools
// replace those of "*"; protected paths add up.
func (a Agents) For(agent string) AgentSettings {
	any := a[AnyAgent]
	rules := AgentSettings{Tools: any.Tools, ProtectedPaths: slices.Clone(any.ProtectedPaths)}
	if own, ok := a[agent]; ok && agent != AnyAgent {
		if len(own.Tools) > 0 {
			rules.Tools = own.Tools
		}
		rules.ProtectedPaths = append(rules.ProtectedPaths, own.ProtectedPaths...)
	}
	return rules
}

// Allows reports whether the rules let the agent call tool.
func (s AgentSettings) Allows(tool string) bool {
	return len(s.Tools) == 0 || slices.Contains(s.Tools, tool)
}

// Protected returns the first path of paths a protected pattern matches.
func (s AgentSettings) Protected(paths []string) (path, pattern string, ok bool) {
	for _, p := range paths {
		for _, pattern := range s.ProtectedPaths {
			if glob.Match(pattern, p) {
				return p, pattern, true
			}
		}
	}
	return "", "", false
}

var (
	agentName = regexp.MustCompile(`^(\*|[a-z0-9][a-z0-9._-]*)$`)
	toolName  = regexp.MustCompile(`^ship_[a-z_]+$`)
)

const agentsExpected = `an object: agent name or "*" → {"tools": [...], "protectedPaths": [...]}`

// mergedWith applies a layer agent by agent: an agent the layer names gets
// the layer's table, the others keep theirs.
func (a Agents) mergedWith(value any, key string, layer Layer) (any, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, wrongTypeAt(layer, key, agentsExpected, value)
	}
	invalid := func(key, value, allowed string) error {
		return fault.New(fault.ConfigInvalidValue, "config.invalid_value",
			"file", layer.Path, "key", key, "value", value, "allowed", allowed)
	}
	out := Agents{}
	for name, rules := range a {
		out[name] = rules
	}
	for _, name := range sortedKeys(raw) {
		agentKey := key + "." + name
		if !agentName.MatchString(name) {
			return nil, invalid(agentKey, name, `"*" or an agent name: lowercase letters, digits, ".", "_" and "-", such as claude`)
		}
		table, ok := raw[name].(map[string]any)
		if !ok {
			return nil, wrongTypeAt(layer, agentKey, `an object with "tools" and "protectedPaths"`, raw[name])
		}
		var rules AgentSettings
		for _, field := range sortedKeys(table) {
			fieldKey := agentKey + "." + field
			items, isArray := table[field].([]any)
			if !isArray {
				return nil, wrongTypeAt(layer, fieldKey, "array of strings", table[field])
			}
			var list []string
			for _, item := range items {
				s, isString := item.(string)
				if !isString {
					return nil, wrongTypeAt(layer, fieldKey, "array of strings", table[field])
				}
				list = append(list, s)
			}
			switch field {
			case "tools":
				for _, t := range list {
					if !toolName.MatchString(t) {
						return nil, invalid(fieldKey, t, "tool names such as ship_status, ship_deliver")
					}
				}
				rules.Tools = list
			case "protectedPaths":
				for _, p := range list {
					if !glob.Valid(p) {
						return nil, invalid(fieldKey, p, "glob patterns such as .github/workflows/**")
					}
				}
				rules.ProtectedPaths = list
			default:
				return nil, fault.New(fault.ConfigUnknownKey, "config.unknown_key",
					"file", layer.Path, "key", fieldKey, "known", agentKey+".tools, "+agentKey+".protectedPaths")
			}
		}
		out[name] = rules
	}
	return out, nil
}

func (Agents) jsonSchema() map[string]any {
	list := map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"type": "string"}}
	tools := map[string]any{"type": "array", "uniqueItems": true, "items": map[string]any{"type": "string", "pattern": toolName.String()}}
	return map[string]any{
		"type":          "object",
		"propertyNames": map[string]any{"pattern": agentName.String()},
		"additionalProperties": map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{"tools": tools, "protectedPaths": list},
		},
	}
}
func (Agents) typeLabel() string { return `object: agent name or "*" → {tools, protectedPaths}` }
func (Agents) expected() string  { return agentsExpected }
