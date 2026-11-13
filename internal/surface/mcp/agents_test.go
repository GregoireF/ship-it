// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package mcp

import (
	"slices"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/testkit"
)

// TestAgentRules: #48. ship.agents hides and refuses the tools an agent may
// not call, and refuses a delivery that touches a protected path.
func TestAgentRules(t *testing.T) {
	h := newHarness()
	h.ws.Config = &testkit.FakeConfigFiles{Repository: `{"ship": {"agents": {"*": {
		"tools": ["ship_status", "ship_plan", "ship_deliver"],
		"protectedPaths": ["internal/app/**"]}}}}`}

	r := h.exchange(t,
		modern(1, "tools/list", ""),
		modern(2, "tools/call", `"name":"ship_publish","arguments":{}`),
		modern(3, "tools/call", `"name":"ship_deliver","arguments":{"title":"fix(app): stop the runner leak"}`),
	)
	var names []string
	for _, tool := range result(t, r[0])["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	slices.Sort(names)
	if strings.Join(names, ",") != "ship_deliver,ship_plan,ship_status" {
		t.Errorf("tools/list = %v", names)
	}
	text := func(res map[string]any) string {
		return res["content"].([]any)[0].(map[string]any)["text"].(string)
	}
	if res := result(t, r[1]); res["isError"] != true || !strings.Contains(text(res), "SHIP-E0802") {
		t.Errorf("ship_publish: %v", res)
	}
	if res := result(t, r[2]); res["isError"] != true || !strings.Contains(text(res), "SHIP-E0803") ||
		!strings.Contains(text(res), "internal/app/run.go") {
		t.Errorf("ship_deliver: %v", res)
	}
	for _, c := range *h.calls {
		if strings.HasPrefix(c, "commit") {
			t.Errorf("a protected file was committed: %q", *h.calls)
		}
	}
}
