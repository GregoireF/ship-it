// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// The commits section of the review screen (#36): the commits of a split
// by intent, each with its title and its changes, arranged with the keys
// of the stack review screen.

// commitRow is one row of the section: a commit (hunk -1) or one of its
// changes.
type commitRow struct{ commit, hunk int }

// splitting reports whether the work ships as several commits.
func (m reviewModel) splitting() bool { return m.split && len(m.drafts) > 1 }

func (m reviewModel) commitRows() []commitRow {
	var rows []commitRow
	for i, c := range m.drafts {
		rows = append(rows, commitRow{i, -1})
		for j := range c.hunks {
			rows = append(rows, commitRow{i, j})
		}
	}
	return rows
}

// rowOf is the row of change ref, or of commit i when ref is nil.
func (m reviewModel) rowOf(i int, ref *app.HunkRef) int {
	for n, row := range m.commitRows() {
		switch {
		case ref == nil && row.commit == i && row.hunk < 0:
			return n
		case ref != nil && row.hunk >= 0 && m.drafts[row.commit].hunks[row.hunk].Ref() == *ref:
			return n
		}
	}
	return 0
}

func (m reviewModel) commitsKey(k string) (tea.Model, tea.Cmd) {
	if k == "s" {
		m.split = !m.split
		return m, nil
	}
	if k == "?" {
		m.help = true
		return m, nil
	}
	if !m.splitting() {
		return m, nil
	}
	rows := m.commitRows()
	row := rows[m.commitRow]
	var ref *app.HunkRef
	if row.hunk >= 0 {
		r := m.drafts[row.commit].hunks[row.hunk].Ref()
		ref = &r
	}
	// moved arranges the drafts, then puts the cursor back on what it was on.
	moved := func(next commitDrafts, ok bool, commit int) (tea.Model, tea.Cmd) {
		if !ok {
			return m, nil
		}
		m.drafts = next
		if len(next) == 1 {
			// Merged back into one: an ordinary commit with that title.
			m.inputs[fieldTitle].SetValue(next[0].title)
			m.proposeBranch(false)
			return m.focusOn(fieldTitle)
		}
		m.commitRow = m.rowOf(min(commit, len(next)-1), ref)
		return m, nil
	}
	switch k {
	case "up", "k":
		m.commitRow = max(m.commitRow-1, 0)
	case "down", "j":
		m.commitRow = min(m.commitRow+1, len(rows)-1)
	case "shift+up", "K":
		if ref == nil {
			next, ok := m.drafts.swap(row.commit, row.commit-1)
			return moved(next, ok, row.commit-1)
		}
		next, ok := m.drafts.moveHunk(row.commit, row.hunk, row.commit-1)
		return moved(next, ok, 0)
	case "shift+down", "J":
		if ref == nil {
			next, ok := m.drafts.swap(row.commit, row.commit+1)
			return moved(next, ok, row.commit+1)
		}
		next, ok := m.drafts.moveHunk(row.commit, row.hunk, row.commit+1)
		return moved(next, ok, 0)
	case "m":
		if ref != nil {
			return m, nil
		}
		if row.commit == len(m.drafts)-1 {
			m.notice = i18n.Render(m.cfg.lang, "cli.review.merge_last", nil)
			return m, nil
		}
		next, ok := m.drafts.merge(row.commit, row.commit+1)
		return moved(next, ok, row.commit)
	default:
		if n, err := strconv.Atoi(k); err == nil && ref != nil && n >= 1 && n <= 9 {
			next, ok := m.drafts.moveHunk(row.commit, row.hunk, n-1)
			return moved(next, ok, 0)
		}
	}
	return m, nil
}

// startEditing puts the title of commit i in the commit title input.
func (m reviewModel) startEditing(i int) (tea.Model, tea.Cmd) {
	m.editing = i
	m.commitTitle = lineInput{focused: true, width: max(m.width-10, 10)}
	m.commitTitle.SetValue(m.drafts[i].title)
	return m, nil
}

// editKey edits the title of a commit: Enter, Tab or Esc keeps it, Ctrl+C
// cancels the review, Ctrl+Enter keeps it and ships.
func (m reviewModel) editKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m.quit(reviewOutcome{action: reviewCancel})
	case "enter", "tab", "shift+tab", "esc":
		m.stopEditing()
		return m, nil
	case "ctrl+enter", "ctrl+j", "f2":
		m.stopEditing()
		return m.finish(reviewShip)
	case "ctrl+l":
		m.stopEditing()
		return m.finish(reviewLocal)
	}
	m.commitTitle.update(msg)
	return m, nil
}

func (m *reviewModel) stopEditing() {
	m.drafts = m.drafts.clone()
	m.drafts[m.editing].title = strings.TrimSpace(m.commitTitle.Value())
	m.editing = -1
}

// finishSplit ships the commits once each title follows the conventions.
func (m reviewModel) finishSplit(action reviewAction, branch string) (tea.Model, tea.Cmd) {
	for i, c := range m.drafts {
		if !m.cfg.conventions.CheckTitle(c.title).Valid() {
			m.notice = i18n.Render(m.cfg.lang, "cli.review.blocked.commit_title", map[string]string{"position": strconv.Itoa(i + 1)})
			m.commitRow = m.rowOf(i, nil)
			return m.focusOn(fieldCommits)
		}
	}
	if !m.plan.CreatesBranch {
		branch = m.plan.Branch
	}
	return m.quit(splitOutcome(reviewOutcome{
		action: action, title: m.plan.Title, branch: branch,
		why: strings.TrimSpace(m.inputs[fieldWhy].Value()), tested: strings.TrimSpace(m.inputs[fieldTested].Value()),
	}, m.drafts, m.plan.HunksDigest))
}

// renderCommits shows the choice between the commits and one commit, then
// each commit: its number, its title (checked), what it does and its
// changes.
func (m reviewModel) renderCommits(line func(string), label string) {
	t := func(key string, params map[string]string) string { return i18n.Render(m.cfg.lang, key, params) }
	splitMark, oneMark := "(•)", "( )"
	if !m.splitting() {
		splitMark, oneMark = oneMark, splitMark
	}
	line(label + splitMark + " " + t("cli.review.commits_split", map[string]string{"count": strconv.Itoa(len(m.drafts))}) +
		"  " + oneMark + " " + t("cli.review.commits_one", nil) + "  " + styleFaint.Render(t("cli.review.commits_switch", nil)))
	if !m.splitting() {
		line("")
		return
	}
	focused := m.focus == fieldCommits
	pointer := func(n int) string {
		if focused && m.commitRow == n && m.editing < 0 {
			return "› "
		}
		return "  "
	}
	n := 0
	for i, c := range m.drafts {
		number := strconv.Itoa(i + 1)
		text, title := c.title, fit(c.title, m.width-8)
		if m.editing == i {
			text, title = strings.TrimSpace(m.commitTitle.Value()), m.commitTitle.view()
		}
		line("  " + pointer(n) + number + "  " + title)
		n++
		// The text is cut before it is styled: a cut style would leak.
		room := m.width - 7
		status, style := "✓ "+t("cli.review.commit_length", map[string]string{
			"length": strconv.Itoa(utf8.RuneCountInString(text)), "max": strconv.Itoa(m.cfg.conventions.MaxTitleLength()),
		}), styleOK
		if check := m.cfg.conventions.CheckTitle(text); !check.Valid() {
			f := check.Faults[0]
			status, style = "✗ "+t(f.Key, f.Params), styleBad
		}
		status = fit(status, room)
		reason := fit(" · "+t(c.reason.Key, c.reason.Params), max(room-lipgloss.Width(status), 0))
		line("       " + style.Render(status) + styleFaint.Render(reason))
		for _, h := range c.hunks {
			line(fit("     "+pointer(n)+hunkLabel(m.cfg.lang, h), m.width))
			n++
		}
	}
	line("")
}
