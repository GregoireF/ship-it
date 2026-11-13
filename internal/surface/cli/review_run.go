// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// reviewMode is how `ship` asks the human to review the plan.
type reviewMode int

const (
	reviewNone    reviewMode = iota // --yes: no review
	reviewNobody                    // no terminal on stdin: exit 2
	reviewConfirm                   // stderr is not a terminal, or ui.reviewScreen false: one question
	reviewPlain                     // --plain, SHIP_ACCESSIBLE=1, ui.accessible, TERM=dumb: line-by-line prompts
	reviewScreen                    // the editable review screen
)

// accessibleMode reports whether the review must stay line by line: asked
// with --plain, SHIP_ACCESSIBLE=1 or ui.accessible (screen readers follow a
// stream of lines, not a redrawn screen), or forced by a terminal that cannot
// move the cursor (TERM=dumb).
func (r runner) accessibleMode() bool {
	if r.global.plain || r.ui.accessible {
		return true
	}
	accessible, _ := strconv.ParseBool(r.env("SHIP_ACCESSIBLE"))
	return accessible || r.env("TERM") == "dumb"
}

func (r runner) env(name string) string {
	for _, kv := range r.deps.Environ {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v
		}
	}
	return ""
}

func (r runner) reviewModeFor(yes bool) reviewMode {
	switch {
	case yes:
		return reviewNone
	case !r.streams.Interactive:
		return reviewNobody
	case r.accessibleMode():
		return reviewPlain
	case !r.streams.TerminalUI || r.ui.noReviewScreen:
		return reviewConfirm
	}
	return reviewScreen
}

// screenEnvironment is the environment the review screen detects its
// colours from: NO_COLOR, set to any value (no-color.org), always removes
// them, then ui.color decides; auto leaves the detection to the terminal.
func (r runner) screenEnvironment() []string {
	env := make([]string, 0, len(r.deps.Environ)+1)
	for _, kv := range r.deps.Environ {
		name, _, _ := strings.Cut(kv, "=")
		if name != "NO_COLOR" && name != "CLICOLOR_FORCE" {
			env = append(env, kv)
		}
	}
	switch {
	case r.env("NO_COLOR") != "" || r.ui.color == app.ColorNever:
		return append(env, "NO_COLOR=1")
	case r.ui.color == app.ColorAlways:
		return append(env, "CLICOLOR_FORCE=1")
	}
	if force := r.env("CLICOLOR_FORCE"); force != "" {
		env = append(env, "CLICOLOR_FORCE="+force)
	}
	return env
}

// runReviewScreen shows the review screen on stderr until the human ships or
// cancels. ok is false when the screen could not start: the caller falls back
// to line-by-line prompts.
func (r runner) runReviewScreen(cfg reviewConfig) (outcome reviewOutcome, ok bool) {
	final, cancelled, ok := r.runScreen(newReviewModel(cfg))
	switch {
	case !ok:
		return reviewOutcome{}, false
	case cancelled:
		return reviewOutcome{action: reviewCancel}, true
	}
	return final.(reviewModel).outcome, true
}

// runStackReviewScreen is runReviewScreen for the layers of ship stack.
func (r runner) runStackReviewScreen(cfg stackReviewConfig) (outcome stackReviewOutcome, ok bool) {
	final, cancelled, ok := r.runScreen(newStackReviewModel(cfg))
	switch {
	case !ok:
		return stackReviewOutcome{}, false
	case cancelled:
		return stackReviewOutcome{action: reviewCancel}, true
	}
	return final.(stackReviewModel).outcome, true
}

// runScreen runs a review screen on stderr until the human decides;
// cancelled is set when the program was interrupted. ok is false when it
// could not start.
func (r runner) runScreen(model tea.Model) (final tea.Model, cancelled, ok bool) {
	program := tea.NewProgram(model,
		tea.WithContext(r.ctx),
		tea.WithInput(r.streams.In),
		tea.WithOutput(r.streams.Err),
		tea.WithEnvironment(r.screenEnvironment()),
	)
	final, err := program.Run()
	if errors.Is(err, tea.ErrProgramKilled) || errors.Is(err, tea.ErrInterrupted) {
		return nil, true, true
	}
	if err != nil {
		r.journal.Logger.Warn("review.screen", "error", err)
		r.status(i18n.Render(r.lang, "cli.review.unavailable", map[string]string{"error": err.Error()}))
		return nil, false, false
	}
	return final, false, true
}

// plainReview asks for the title, the branch (when one is created), why and
// how it was tested, then whether to ship: one line each, on stderr, so that
// a screen reader reads every prompt once. Enter keeps the value in brackets.
func (r runner) plainReview(cfg reviewConfig) reviewOutcome {
	cancel := reviewOutcome{action: reviewCancel}
	t := func(key string, params map[string]string) string { return i18n.Render(r.lang, key, params) }
	if len(cfg.plan.Commits) > 1 {
		drafts := draftsOf(cfg.plan.Commits)
		r.printDrafts(drafts)
		if r.confirm(t("cli.prompt.split", map[string]string{"count": strconv.Itoa(len(drafts))})) {
			return r.plainSplitReview(cfg, drafts)
		}
	}
	proposed := cfg.plan.Title
	if choices := cfg.plan.TypeChoices; len(choices) > 0 {
		picked, ok := r.askType(choices)
		if !ok {
			return cancel
		}
		proposed, _ = cfg.conventions.RetypeTitle(proposed, picked)
	}
	title, ok := r.askValid(t("cli.prompt.title", map[string]string{"default": proposed}), proposed, func(v string) *app.Fault {
		if check := cfg.conventions.CheckTitle(v); !check.Valid() {
			return check.Faults[0]
		}
		// A retyped title must keep a type the project allows.
		return app.TypeAllowed(v, cfg.plan.TypeChoices)
	})
	if !ok {
		return cancel
	}
	branch := cfg.plan.Branch
	if cfg.plan.CreatesBranch {
		// An edited title proposes its own branch, as on the review screen.
		if title != cfg.plan.Title && cfg.branchFor != nil {
			if b, ok := cfg.branchFor(title); ok {
				branch = b
			}
		}
		if branch, ok = r.askValid(t("cli.prompt.branch", map[string]string{"default": branch}), branch, func(v string) *app.Fault {
			return cfg.checkBranch(v).Fault
		}); !ok {
			return cancel
		}
	}
	why, ok := r.askOptional("cli.prompt.why", cfg.why)
	if !ok {
		return cancel
	}
	tested, ok := r.askOptional("cli.prompt.tested", cfg.tested)
	if !ok || !r.confirm(t("cli.ship.confirm", nil)) {
		return cancel
	}
	return reviewOutcome{action: reviewShip, title: title, branch: branch, why: why, tested: tested, only: cfg.only}
}

// printDrafts lists the commits of a split by intent, with what each does
// and its changes numbered for the plain review (1.1, 1.2, 2.1…).
func (r runner) printDrafts(drafts commitDrafts) {
	r.status(i18n.Render(r.lang, "cli.split.header", map[string]string{"count": strconv.Itoa(len(drafts))}))
	for i, c := range drafts {
		r.status(i18n.Render(r.lang, "cli.split.commit", map[string]string{
			"position": strconv.Itoa(i + 1), "title": c.title, "reason": i18n.Render(r.lang, c.reason.Key, c.reason.Params),
		}))
		for j, h := range c.hunks {
			r.status("     " + strconv.Itoa(i+1) + "." + strconv.Itoa(j+1) + " " + hunkLabel(r.lang, h))
		}
	}
}

// plainSplitReview lets the human move changes between commits, reorder
// and merge them (one command per line, the list shown again after each),
// then asks for the title of each commit, the branch, why and how it was
// tested (they go into the last commit), and whether to ship.
func (r runner) plainSplitReview(cfg reviewConfig, drafts commitDrafts) reviewOutcome {
	cancel := reviewOutcome{action: reviewCancel}
	t := func(key string, params map[string]string) string { return i18n.Render(r.lang, key, params) }
	for len(drafts) > 1 {
		fmt.Fprint(r.streams.Err, t("cli.prompt.split_change", nil))
		answer, ok := r.readLine()
		if !ok {
			fmt.Fprintln(r.streams.Err)
			return cancel
		}
		if strings.TrimSpace(answer) == "" {
			break
		}
		next, ok := drafts.applyChange(answer)
		if !ok {
			r.status(t("cli.split.change_invalid", nil))
			continue
		}
		drafts = next
		r.printDrafts(drafts)
	}
	for i := range drafts {
		title, ok := r.askValid(t("cli.prompt.commit_title", map[string]string{"position": strconv.Itoa(i + 1), "default": drafts[i].title}), drafts[i].title,
			func(v string) *app.Fault {
				if check := cfg.conventions.CheckTitle(v); !check.Valid() {
					return check.Faults[0]
				}
				return nil
			})
		if !ok {
			return cancel
		}
		drafts[i].title = title
	}
	branch := cfg.plan.Branch
	if cfg.plan.CreatesBranch {
		var ok bool
		if branch, ok = r.askValid(t("cli.prompt.branch", map[string]string{"default": branch}), branch, func(v string) *app.Fault {
			return cfg.checkBranch(v).Fault
		}); !ok {
			return cancel
		}
	}
	why, ok := r.askOptional("cli.prompt.why", cfg.why)
	if !ok {
		return cancel
	}
	tested, ok := r.askOptional("cli.prompt.tested", cfg.tested)
	if !ok || !r.confirm(t("cli.ship.confirm", nil)) {
		return cancel
	}
	return splitOutcome(reviewOutcome{action: reviewShip, title: cfg.plan.Title, branch: branch, why: why, tested: tested}, drafts, cfg.plan.HunksDigest)
}

// splitOutcome puts the commits into an outcome; merged back into one, they
// are one ordinary commit with its title.
func splitOutcome(o reviewOutcome, drafts commitDrafts, digest string) reviewOutcome {
	if len(drafts) == 1 {
		o.title = strings.TrimSpace(drafts[0].title)
		return o
	}
	o.commits, o.hunksDigest = drafts.specs(), digest
	return o
}

// plainStackReview asks for the title and the branch of each layer, bottom
// first, then whether to ship: one line each, as plainReview does. The files
// of the layers are chosen with --layer.
func (r runner) plainStackReview(cfg stackReviewConfig) stackReviewOutcome {
	cancel := stackReviewOutcome{action: reviewCancel}
	t := func(key string, params map[string]string) string { return i18n.Render(r.lang, key, params) }
	var layers []app.StackLayerSpec
	for i, l := range cfg.plan.Layers {
		position := strconv.Itoa(i + 1)
		title, ok := r.askValid(t("cli.prompt.layer_title", map[string]string{"position": position, "default": l.Title}), l.Title,
			func(v string) *app.Fault {
				if check := cfg.conventions.CheckTitle(v); !check.Valid() {
					return check.Faults[0]
				}
				return nil
			})
		if !ok {
			return cancel
		}
		branch := l.Branch
		if title != l.Title && cfg.branchFor != nil {
			if b, ok := cfg.branchFor(title); ok {
				branch = b
			}
		}
		branch, ok = r.askValid(t("cli.prompt.layer_branch", map[string]string{"position": position, "default": branch}), branch,
			func(v string) *app.Fault {
				if cfg.checkBranch != nil {
					if f := cfg.checkBranch(v).Fault; f != nil {
						return f
					}
				}
				if slices.ContainsFunc(layers, func(s app.StackLayerSpec) bool { return s.Branch == v }) {
					return app.StackBranchTaken(v)
				}
				return nil
			})
		if !ok {
			return cancel
		}
		layers = append(layers, app.StackLayerSpec{Title: title, Branch: branch, Files: l.Files})
	}
	if !r.confirm(t("cli.stack.confirm", map[string]string{"count": strconv.Itoa(len(layers))})) {
		return cancel
	}
	return stackReviewOutcome{action: reviewShip, layers: layers}
}

// askType asks for one of the commit types the project's commitlint allows,
// by number or by name; there is no default: the human decides.
func (r runner) askType(choices []string) (string, bool) {
	numbered := make([]string, len(choices))
	for i, c := range choices {
		numbered[i] = strconv.Itoa(i+1) + ") " + c
	}
	prompt := i18n.Render(r.lang, "cli.prompt.type", map[string]string{"choices": strings.Join(numbered, "  ")})
	for {
		fmt.Fprint(r.streams.Err, prompt)
		answer, ok := r.readLine()
		if !ok {
			fmt.Fprintln(r.streams.Err)
			return "", false
		}
		answer = strings.TrimSpace(answer)
		if n, err := strconv.Atoi(answer); err == nil && n >= 1 && n <= len(choices) {
			return choices[n-1], true
		}
		for _, c := range choices {
			if strings.EqualFold(answer, c) {
				return c, true
			}
		}
		r.status(i18n.Render(r.lang, "cli.prompt.type_invalid", map[string]string{"choices": strings.Join(choices, ", ")}))
	}
}

// askValid asks until the answer passes check; Enter keeps def. ok is false
// at the end of the input.
func (r runner) askValid(prompt, def string, check func(string) *app.Fault) (string, bool) {
	for {
		fmt.Fprint(r.streams.Err, prompt)
		answer, ok := r.readLine()
		if !ok {
			fmt.Fprintln(r.streams.Err)
			return "", false
		}
		if answer = strings.TrimSpace(answer); answer == "" {
			answer = def
		}
		f := check(answer)
		if f == nil {
			return answer, true
		}
		r.status(r.renderFault(f))
	}
}

// askOptional asks for an optional value: key for an empty default,
// key+"_default" to show the one given on the command line.
func (r runner) askOptional(key, def string) (string, bool) {
	prompt := i18n.Render(r.lang, key, nil)
	if def != "" {
		prompt = i18n.Render(r.lang, key+"_default", map[string]string{"default": def})
	}
	fmt.Fprint(r.streams.Err, prompt)
	answer, ok := r.readLine()
	if !ok {
		fmt.Fprintln(r.streams.Err)
		return "", false
	}
	if answer = strings.TrimSpace(answer); answer == "" {
		return def, true
	}
	return answer, true
}

// readLine reads one line from stdin a byte at a time, so that nothing past
// the line is consumed. ok is false at the end of the input with nothing read.
func (r runner) readLine() (string, bool) {
	if r.streams.In == nil {
		return "", false
	}
	var line []byte
	buf := make([]byte, 1)
	for {
		n, err := r.streams.In.Read(buf)
		if n == 1 {
			if buf[0] == '\n' {
				return strings.TrimSuffix(string(line), "\r"), true
			}
			line = append(line, buf[0])
		}
		if err == io.EOF {
			return strings.TrimSuffix(string(line), "\r"), len(line) > 0
		}
		if err != nil {
			return "", false
		}
	}
}
