// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package i18n

import (
	"slices"
	"testing"
)

// TestEveryCodeCanBeExplained: the generated table explains every SHIP-E and
// SHIP-W code of internal/core/fault, its keys render in both languages, and
// the pages come in the order of the errors page.
func TestEveryCodeCanBeExplained(t *testing.T) {
	pages := FaultPages()
	if len(pages) < 50 {
		t.Fatalf("read only %d pages from the generated table", len(pages))
	}
	known := Keys()
	for _, page := range pages {
		if page.Group == "" || page.Status == "" {
			t.Errorf("%s: empty group or status", page.Code)
		}
		if len(page.Keys) == 0 {
			t.Errorf("%s has no message to show", page.Code)
		}
		for _, key := range page.Keys {
			if !slices.Contains(known, key) {
				t.Errorf("%s: %s is not in the catalogue", page.Code, key)
			}
			for _, lang := range []Language{English, French} {
				if Render(lang, key, nil) == key {
					t.Errorf("%s: %s is missing from the %s catalogue", page.Code, key, lang)
				}
			}
		}
	}
	for i := 1; i < len(pages); i++ {
		if pages[i-1].Code[6:] >= pages[i].Code[6:] {
			t.Errorf("pages are not in the order of the errors page: %s then %s", pages[i-1].Code, pages[i].Code)
		}
	}
	page, ok := FaultPageOf("SHIP-E0618")
	if !ok || page.Note == "" || page.Status != "exit code 1" || len(page.Keys) == 0 {
		t.Errorf("SHIP-E0618: %+v (ok %v)", page, ok)
	}
	// A SHIP-I passing check and a code that does not exist stay unknown.
	for _, code := range []string{"SHIP-I0000", "SHIP-X9999", "SHIP-E06180"} {
		if _, ok := FaultPageOf(code); ok {
			t.Errorf("%s must not be explainable", code)
		}
	}
}
