// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/surface/mcp"
)

// mcpCall sends one tools/call to a server on the scenario's repository, as
// a coding agent would through ship mcp, and returns its result.
func (s *scenario) mcpCall(tool, arguments string) (structured map[string]any, text string, isError bool) {
	s.t.Helper()
	const meta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"claude-code","version":"1"}}`
	in := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s,%s}}`, tool, arguments, meta)
	var out bytes.Buffer
	srv := &mcp.Server{Workspace: s.deps, Version: "test"}
	if err := srv.Serve(context.Background(), strings.NewReader(in+"\n"), &out); err != nil {
		s.t.Fatal(err)
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent map[string]any `json:"structuredContent"`
			IsError           bool           `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || len(response.Result.Content) == 0 {
		s.t.Fatalf("%s: unexpected response %q (%v)", tool, out.String(), err)
	}
	return response.Result.StructuredContent, response.Result.Content[0].Text, response.Result.IsError
}

// TestAgentProposesTheTitle follows the Agent Skill: the agent reads the
// conventions from ship_plan, proposes a title that breaks some of them, gets
// every fault with a corrected proposal, and delivers with it (#33).
func TestAgentProposesTheTitle(t *testing.T) {
	s := newScenario(t, nil)
	s.write("README.md", "# shop\n")
	s.sh("add", "--all")
	s.sh("commit", "--quiet", "-m", "🎉 chore: init")
	s.write("cart.go", "package shop\n\nfunc EmptyCart() bool { return true }\n")

	plan, _, isError := s.mcpCall("ship_plan", `{}`)
	conventions, _ := plan["conventions"].(map[string]any)
	if isError || conventions == nil {
		t.Fatalf("ship_plan gives no conventions: %v", plan)
	}
	if types, _ := conventions["types"].([]any); len(types) == 0 || !strings.Contains(fmt.Sprint(types[0]), "description:") {
		t.Errorf("conventions without described types: %v", conventions["types"])
	}
	if conventions["title_format"] != "<gitmoji> <type>(<scope>): <subject>" || conventions["subject_max"] != float64(50) {
		t.Errorf("conventions = %v", conventions)
	}

	_, text, isError := s.mcpCall("ship_deliver", `{"title":"Feature(Cart): Handle an empty cart."}`)
	if !isError || !strings.HasPrefix(text, "SHIP-E0103") ||
		!strings.Contains(text, "\n- SHIP-E0109") || !strings.Contains(text, "\n- SHIP-E0106") || !strings.Contains(text, "\n- SHIP-E0107") ||
		!strings.Contains(text, "Proposed title, yours corrected: ✨ feat(cart): handle an empty cart") ||
		!strings.Contains(text, "Proposed branch: feat/") {
		t.Fatalf("refusal = %q", text)
	}
	if n := s.sh("rev-list", "--count", "HEAD"); n != "1" {
		t.Fatalf("a refused proposal committed: %s commits", n)
	}

	delivered, text, isError := s.mcpCall("ship_deliver", `{"title":"✨ feat(cart): handle an empty cart"}`)
	if isError {
		t.Fatalf("the corrected proposal is refused: %s", text)
	}
	if got := s.sh("log", "-1", "--format=%s"); got != "✨ feat(cart): handle an empty cart" {
		t.Errorf("commit subject = %q", got)
	}
	if branch := s.sh("branch", "--show-current"); branch != delivered["branch"] || !strings.HasPrefix(branch, "feat/") {
		t.Errorf("branch = %q, result %v", branch, delivered["branch"])
	}
}
