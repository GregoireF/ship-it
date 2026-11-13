// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import (
	"strconv"
	"time"

	"github.com/GregoireF/ship-it/internal/app"
	"github.com/GregoireF/ship-it/internal/i18n"
)

// watch takes a checkpoint after each pause in the edits until Ctrl+C
// (#38): the working tree is only looked at between checkpoints, nothing
// is written while nothing changes.
func (r runner) watch(args []string) int {
	fs, _ := r.flags("watch")
	if code, ok := r.parse(fs, args); !ok {
		return code
	}
	if fs.NArg() > 0 {
		r.status(i18n.Render(r.lang, "cli.error.unexpected_argument_of", map[string]string{"argument": fs.Arg(0), "command": commandLine(fs.Name())}))
		return ExitUsage
	}
	t := func(key string, params map[string]string) string { return i18n.Render(r.lang, key, params) }
	pause := app.WatchPause(r.ctx, r.deps)
	r.status(t("cli.watch.start", map[string]string{"seconds": strconv.Itoa(int(pause / time.Second))}))
	taken, err := app.Watch(r.ctx, r.deps, app.WatchOptions{Pause: pause, Taken: func(v app.CheckpointView) {
		r.status(t("cli.watch.taken", map[string]string{"time": v.CreatedAt.Local().Format("15:04:05"), "id": v.ShortID}))
	}})
	if err != nil {
		return r.fail(err)
	}
	r.note(t("cli.watch.stopped", map[string]string{"count": strconv.Itoa(taken)}))
	return ExitOK
}
