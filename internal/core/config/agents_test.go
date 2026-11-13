// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/core/fault"
)

// TestAgentsLayers: #48. The repository sets "*" and one agent; the user's
// layer adds another; an agent's table completes "*".
func TestAgentsLayers(t *testing.T) {
	eff, err := Merge([]Layer{
		{Name: "user", Path: "~/.config/ship-it/config.json", Data: []byte(`{"ship": {"agents": {"cursor": {"tools": ["ship_status"]}}}}`)},
		{Name: "repository", Path: ".github/ship.json", Data: []byte(`{"ship": {"agents": {
			"*": {"protectedPaths": [".github/workflows/**"]},
			"vscode": {"tools": ["ship_status", "ship_plan", "ship_deliver"], "protectedPaths": ["go.mod"]}}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	agents := eff.Settings.Ship.Agents
	vscode := agents.For("vscode")
	if !reflect.DeepEqual(vscode.ProtectedPaths, []string{".github/workflows/**", "go.mod"}) || !vscode.Allows("ship_deliver") || vscode.Allows("ship_publish") {
		t.Errorf("vscode = %+v", vscode)
	}
	if cursor := agents.For("cursor"); cursor.Allows("ship_plan") || !cursor.Allows("ship_status") {
		t.Errorf("cursor = %+v", cursor)
	}
	unknown := agents.For("unknown")
	if !unknown.Allows("ship_publish") {
		t.Error(`an agent without a table follows "*", which allows every tool`)
	}
	if path, pattern, ok := unknown.Protected([]string{"main.go", ".github/workflows/ci.yml"}); !ok || path != ".github/workflows/ci.yml" || pattern != ".github/workflows/**" {
		t.Errorf("Protected = %q %q %v", path, pattern, ok)
	}
	if _, _, ok := unknown.Protected([]string{"main.go"}); ok {
		t.Error("main.go is not protected")
	}
}

func TestAgentsRefusals(t *testing.T) {
	for _, tt := range []struct {
		json string
		code fault.Code
		key  string
	}{
		{`{"Claude": {}}`, fault.ConfigInvalidValue, "ship.agents.Claude"},
		{`{"claude": {"tools": ["status"]}}`, fault.ConfigInvalidValue, "ship.agents.claude.tools"},
		{`{"claude": {"protectedPaths": ["[a"]}}`, fault.ConfigInvalidValue, "ship.agents.claude.protectedPaths"},
		{`{"claude": {"paths": []}}`, fault.ConfigUnknownKey, "ship.agents.claude.paths"},
		{`{"claude": {"tools": "ship_status"}}`, fault.ConfigWrongType, "ship.agents.claude.tools"},
	} {
		_, err := Merge([]Layer{{Name: "repository", Path: ".github/ship.json",
			Data: []byte(`{"ship": {"agents": ` + tt.json + `}}`)}})
		var f *fault.Fault
		if !errors.As(err, &f) || f.Code != tt.code || !strings.HasPrefix(f.Params["key"], tt.key) {
			t.Errorf("%s: %v, want %s on %s", tt.json, err, tt.code, tt.key)
		}
	}
}
