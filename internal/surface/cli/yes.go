// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package cli

import "flag"

// acceptsYes declares --yes/-y on a command that writes, so scripts and agents
// can pass the same flag to every writing command (issue #122).
func acceptsYes(fs *flag.FlagSet) *bool {
	value := new(bool)
	fs.BoolVar(value, "yes", false, "accept without asking")
	fs.BoolVar(value, "y", false, "shorthand for --yes")
	return value
}
