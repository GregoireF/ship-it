// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package i18n

import "sort"

// FaultPage is what ship knows about a SHIP-E or SHIP-W code, generated from
// internal/core/fault and the source (faults_gen.go): the note of its
// constant, the title of the block it belongs to, the exit status ship ends
// with, and the catalogue keys used with the code. docs/reference/errors.md
// shows the same page; `ship explain` renders it, in either language.
type FaultPage struct {
	Code   string   `json:"code"`
	Note   string   `json:"note"`
	Group  string   `json:"group"`
	Status string   `json:"status"`
	Keys   []string `json:"keys"`
}

// FaultPages returns the page of every explainable code, in the order of the
// errors page: by the number after SHIP-.
func FaultPages() []FaultPage {
	pages := make([]FaultPage, 0, len(faultPages))
	for code, page := range faultPages {
		pages = append(pages, FaultPage{Code: code, Note: page.Note, Group: page.Group, Status: page.Status, Keys: page.Keys})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Code[6:] < pages[j].Code[6:] })
	return pages
}

// FaultPageOf returns the page of one code; ok is false for an unknown code,
// SHIP-I included: a passing check, not a problem to look up.
func FaultPageOf(code string) (FaultPage, bool) {
	page, ok := faultPages[code]
	if !ok {
		return FaultPage{}, false
	}
	return FaultPage{Code: code, Note: page.Note, Group: page.Group, Status: page.Status, Keys: page.Keys}, true
}
