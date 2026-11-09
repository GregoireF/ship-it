// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"fmt"
	"strings"

	"github.com/GregoireF/ship-it/internal/i18n"
)

// explain prints the page of a SHIP-E or SHIP-W code from the generated
// table (the text of docs/reference/errors.md, offline, in the language of
// the run), or every code with its note when no code is given. Nothing here
// reaches the network: the table was made by go run ./tools/gen.
func (r runner) explain(args []string) int {
	fs, asJSON := r.flags("explain")
	if code, ok := r.parse(fs, args); !ok {
		return code
	}
	switch {
	case fs.NArg() > 1:
		r.status(i18n.Render(r.lang, "cli.error.unexpected_argument_of", map[string]string{"argument": fs.Arg(1), "command": commandLine(fs.Name())}))
		return ExitUsage
	case fs.NArg() == 1:
		return r.explainCode(fs.Arg(0), *asJSON)
	default:
		return r.explainList(*asJSON)
	}
}

// explainCode prints the page of one code. The input is forgiving: spaces
// are trimmed, case is ignored, and the SHIP- prefix may be left out or
// doubled ("e0618", "ship-e0618" and "SHIP-SHIP-E0618" all work). A code
// nothing is known about — SHIP-I passing checks included — is a usage
// error, said on stderr as logs.md asks.
func (r runner) explainCode(arg string, asJSON bool) int {
	name := strings.ToUpper(strings.TrimSpace(arg))
	for strings.HasPrefix(name, "SHIP-") {
		name = strings.TrimPrefix(name, "SHIP-")
	}
	page, ok := i18n.FaultPageOf("SHIP-" + name)
	if !ok {
		r.status(i18n.Render(r.lang, "cli.explain.unknown", map[string]string{"code": strings.TrimSpace(arg)}))
		return ExitUsage
	}
	if asJSON {
		return r.writeJSON(page)
	}
	out := r.streams.Out
	if page.Note != "" {
		fmt.Fprintf(out, "%s — %s\n", page.Code, page.Note)
	} else {
		fmt.Fprintln(out, page.Code)
	}
	fmt.Fprintln(out, statusLine(page.Status))
	// The same structure as the errors page: what ship prints first, then
	// what it suggests, a doctor remedy or what ship fix does.
	var messages, remedies, fixes []string
	for _, key := range page.Keys {
		switch {
		case strings.HasPrefix(key, "doctor.remedy."):
			remedies = append(remedies, strings.Join(strings.Fields(i18n.Render(r.lang, key, nil)), " "))
		case strings.HasPrefix(key, "fix."):
			fixes = append(fixes, i18n.Render(r.lang, key, nil))
		default:
			messages = append(messages, strings.ReplaceAll(i18n.Render(r.lang, key, nil), "\n", " "))
		}
	}
	if len(messages) > 0 {
		fmt.Fprintln(out)
		for _, message := range messages {
			fmt.Fprintln(out, message)
		}
	}
	if len(remedies) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, i18n.Render(r.lang, "cli.explain.remedy", map[string]string{"remedies": strings.Join(remedies, " · ")}))
	}
	for _, fix := range fixes {
		fmt.Fprintln(out, i18n.Render(r.lang, "cli.explain.fix", map[string]string{"fix": strings.TrimSuffix(fix, ".")}))
	}
	return ExitOK
}

// statusLine renders an exit status the way the errors page does: a capital
// first letter and a period.
func statusLine(status string) string {
	return strings.ToUpper(status[:1]) + status[1:] + "."
}

// explainList prints every explainable code with its note, grouped like the
// errors page.
func (r runner) explainList(asJSON bool) int {
	pages := i18n.FaultPages()
	if asJSON {
		return r.writeJSON(pages)
	}
	out := r.streams.Out
	fmt.Fprintln(out, i18n.Render(r.lang, "cli.explain.list", nil))
	group := ""
	for _, page := range pages {
		if page.Group != group {
			group = page.Group
			fmt.Fprintf(out, "\n%s:\n", group)
		}
		if page.Note != "" {
			fmt.Fprintf(out, "  %s  %s\n", page.Code, page.Note)
		} else {
			fmt.Fprintln(out, "  "+page.Code)
		}
	}
	return ExitOK
}
