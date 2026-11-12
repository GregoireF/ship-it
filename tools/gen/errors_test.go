// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GregoireF/ship-it/internal/i18n"
)

// TestEveryFaultCodeHasAMessage keeps docs/reference/errors.md useful: a
// code that the source never pairs with a catalogue key would be listed with
// nothing to read.
func TestEveryFaultCodeHasAMessage(t *testing.T) {
	root := filepath.Join("..", "..")
	codes, err := faultCodes(root)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := faultKeys(root, i18n.Keys())
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) < 50 {
		t.Fatalf("read only %d codes from fault.go", len(codes))
	}
	for _, code := range codes {
		if len(keys[code.Name]) == 0 {
			t.Errorf("%s (%s) is never used with a catalogue key", code.Value, code.Name)
		}
	}
}

// TestEveryFaultCodeHasAGeneratedEntry keeps the generated table of
// internal/i18n complete: every code constant of fault.go has an entry whose
// keys render in both English and French, so `ship explain` can show them.
func TestEveryFaultCodeHasAGeneratedEntry(t *testing.T) {
	root := filepath.Join("..", "..")
	codes, err := faultCodes(root)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := faultKeys(root, i18n.Keys())
	if err != nil {
		t.Fatal(err)
	}
	table, err := faultsTable(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range codes {
		if !strings.Contains(string(table), strconv.Quote(code.Value)+":") {
			t.Errorf("%s has no entry in the generated table", code.Value)
		}
		for _, key := range keys[code.Name] {
			for _, lang := range []i18n.Language{i18n.English, i18n.French} {
				if got := i18n.Render(lang, key, nil); got == key {
					t.Errorf("%s: %s is missing from the %s catalogue", code.Value, key, lang)
				}
			}
		}
	}
}

func TestGroupTitle(t *testing.T) {
	for doc, want := range map[string]string{
		"Codes of the organisation policy (08xx range): they exit with code 5.": "Organisation policy",
		"Codes of delivery (05xx range).":                                       "Delivery",
		"Unexpected wraps an error that has no dedicated code yet.":             "General",
	} {
		if got := groupTitle(doc); got != want {
			t.Errorf("groupTitle(%q) = %q, want %q", doc, got, want)
		}
	}
}
