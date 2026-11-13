// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Grégoire Favreau

package gitexec

import (
	"context"
	"errors"
	"strings"
)

// ZeroContextDiff implements ports.Git.
func (c *Client) ZeroContextDiff(ctx context.Context) (string, error) {
	out, err := c.Run(ctx, "-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff", "--find-renames", "-U0", "HEAD", "--")
	return string(out), err
}

// StagePaths implements ports.Git. The pathspecs are literal and from the
// top of the repository, wherever ship runs.
func (c *Client) StagePaths(ctx context.Context, paths []string) error {
	args := []string{"add", "--all", "--"}
	for _, p := range paths {
		args = append(args, ":(top,literal)"+p)
	}
	_, err := c.Run(ctx, args...)
	return err
}

// StageContent implements ports.Git: the blob is written as it is (no
// filter: content is already what git stores), then put in the index.
func (c *Client) StageContent(ctx context.Context, path string, content []byte) error {
	top, err := c.TopLevel(ctx)
	if err != nil {
		return err
	}
	entry, err := c.Run(ctx, "-C", top, "ls-tree", "HEAD", "--", path)
	if err != nil {
		return err
	}
	mode, _, ok := strings.Cut(string(entry), " ")
	if !ok {
		return errors.New("gitexec: " + path + " is not in HEAD")
	}
	blob, err := c.RunWithInput(ctx, nil, content, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return err
	}
	_, err = c.Run(ctx, "-C", top, "update-index", "--cacheinfo", mode+","+strings.TrimSpace(string(blob))+","+path)
	return err
}
