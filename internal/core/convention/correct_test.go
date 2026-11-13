// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package convention

import (
	"strings"
	"testing"
)

var plannedHeader = Header{Type: Feature, Scope: "cart", Subject: "add cart"}

func TestCorrect(t *testing.T) {
	withoutFeat := Rules{Types: WithTypes([]TypeChange{{Type: Feature, Remove: true}})}
	for _, tc := range []struct {
		name  string
		rules Rules
		title string
		want  string // formatted with GitmojiOmitted
		kept  bool
	}{
		{"valid stays as it is", Rules{}, "fix(cart): handle an empty cart", "fix(cart): handle an empty cart", true},
		{"case and final period", Rules{}, "Fix(Cart): Handle an empty cart.", "fix(cart): handle an empty cart", true},
		{"type alias", Rules{}, "feature: add VAT", "feat: add VAT", true},
		{"acronym keeps its case", Rules{}, "✨ feat: API keys rotate", "feat: API keys rotate", true},
		{"scope not listed is dropped", Rules{Scopes: []string{"api"}}, "feat(cart): add x", "feat: add x", true},
		{"scope too long for the header is dropped", Rules{}, "feat(configuration): " + strings.Repeat("a", 50), "feat: " + strings.Repeat("a", 50), true},
		{"not a conventional title", Rules{}, "update stuff", "feat(cart): add cart", false},
		{"unknown type", Rules{}, "yolo: add x", "feat(cart): add cart", false},
		{"alias of a type the repository removed", withoutFeat, "feature: add x", "feat(cart): add cart", false},
		{"subject too long is never cut", Rules{}, "fix: " + strings.Repeat("a", 51), "feat(cart): add cart", false},
		{"breaking change the type does not allow", Rules{}, "docs!: rewrite", "feat(cart): add cart", false},
		{"required scope not listed", Rules{Scopes: []string{"api"}, ScopeRequired: true}, "feat(cart): add x", "feat(cart): add cart", false},
		{"empty subject", Rules{}, "fix: .", "feat(cart): add cart", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, kept := tc.rules.Correct(tc.title, plannedHeader)
			if got := Format(h, GitmojiOmitted); got != tc.want || kept != tc.kept {
				t.Errorf("Correct(%q) = %q, kept %v; want %q, kept %v", tc.title, got, kept, tc.want, tc.kept)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	for title, want := range map[string]string{
		"fix(cart): handle an empty cart":      "",
		"Feature(Cart): Handle an empty cart.": "SHIP-E0103 SHIP-E0109 SHIP-E0106 SHIP-E0107",
		"update stuff":                         "SHIP-E0102",
		"":                                     "SHIP-E0101",
	} {
		var codes []string
		for _, f := range (Rules{}).ValidateTitle(title) {
			codes = append(codes, string(f.Code))
		}
		if got := strings.Join(codes, " "); got != want {
			t.Errorf("ValidateTitle(%q) = %q, want %q", title, got, want)
		}
	}
}

func FuzzCorrect(f *testing.F) {
	for _, s := range []string{"Fix(Cart): Handle it.", "feature: add VAT", "yolo: x", "docs!: y", "feat(): z", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		h, kept := Rules{}.Correct(s, plannedHeader)
		if faults := (Rules{}).Validate(h); len(faults) > 0 {
			t.Fatalf("Correct(%q) = %+v, which breaks %s", s, h, faults[0].Code)
		}
		if !kept && h != plannedHeader {
			t.Fatalf("Correct(%q) dropped the wording but did not return the fallback: %+v", s, h)
		}
	})
}
