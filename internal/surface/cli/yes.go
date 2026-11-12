// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import "flag"

// acceptsYes declares --yes/-y on a command that writes, so scripts and agents
// can pass the same flag to every writing command (issue #122). These commands
// ask nothing today, so the flag changes nothing yet: it is accepted, then
// ignored, which is what `stack sync` does too.
func acceptsYes(fs *flag.FlagSet) {
	value := new(bool)
	fs.BoolVar(value, "yes", false, "accept without asking (no prompt today)")
	fs.BoolVar(value, "y", false, "shorthand for --yes")
}
