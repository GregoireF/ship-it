// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package convention

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/GregoireF/ship-it/internal/core/fault"
)

// typeAliases are spellings of a conventional type that people and agents
// often write instead of it.
var typeAliases = map[CommitType]CommitType{
	"feature": Feature, "features": Feature,
	"bug": BugFix, "bugfix": BugFix, "hotfix": BugFix,
	"doc": Documentation, "documentation": Documentation,
	"tests": Testing, "testing": Testing,
	"refacto": Refactoring, "refactoring": Refactoring,
	"performance": Performance,
	"styles":      Styling,
	"chores":      Maintenance,
}

// ValidateTitle returns every rule a written title breaks, an unknown type
// included: the grammar first, then Validate. nil means valid.
func (r Rules) ValidateTitle(title string) []*fault.Fault {
	h, err := parseGrammar(title)
	if err != nil {
		return []*fault.Fault{err.(*fault.Fault)}
	}
	return r.Validate(h)
}

// Correct returns a valid header for a title someone proposed (an agent,
// through ship_deliver), keeping its wording when the only faults have a
// correction that does not change its meaning: a type alias ("feature"), the
// case of the scope or of the first letter of the subject, a final period, a
// scope the repository does not list (dropped when none is required), a
// header too long because of its scope. Anything else (no type ship knows,
// a subject too long, a breaking change the type does not allow) returns
// fallback: ship never cuts or rewrites a subject. kept reports whether the
// result keeps the proposed wording.
func (r Rules) Correct(title string, fallback Header) (h Header, kept bool) {
	h, err := parseGrammar(title)
	if err != nil {
		return fallback, false
	}
	if _, ok := r.LookupCommitType(h.Type); !ok {
		alias, isAlias := typeAliases[h.Type]
		if _, known := r.LookupCommitType(alias); !isAlias || !known {
			return fallback, false
		}
		h.Type = alias
	}
	if h.Scope != "" {
		h.Scope = strings.ToLower(strings.TrimSpace(h.Scope))
		if !r.ScopeAllowed(h.Scope) && !r.ScopeRequired {
			h.Scope = ""
		}
	}
	h.Subject = strings.TrimRight(strings.TrimSpace(h.Subject), ". ")
	if first, size := utf8.DecodeRuneInString(h.Subject); unicode.IsUpper(first) && !startsWithAcronym(h.Subject) {
		h.Subject = string(unicode.ToLower(first)) + h.Subject[size:]
	}
	h = r.FitToMaxLength(h)
	if len(r.Validate(h)) > 0 {
		return fallback, false
	}
	return h, true
}
