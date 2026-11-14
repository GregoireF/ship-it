# Glossary

The words you meet in ship-it's messages and documentation, in one or two sentences each.

**Base branch** (or default branch): The branch your work goes into once accepted, usually `main`. ship reads it from GitHub.

**Branch**: A separate line of work in git. You work on a branch so that `main` stays clean until your work is reviewed. ship creates one for you when you are on `main`.

**Checkpoint**: A full snapshot of your files that ship takes before changing anything. It stays on your computer, inside `.git`.

**Commit**: A saved step of your work in git, with a message that says what changed. ship writes the message for you, in the [Conventional Commits](https://www.conventionalcommits.org/) format.

**Conflict**: Two changes to the same lines that git cannot combine alone. A human decides which version to keep.

**Conventional Commits**: A standard format for commit messages: `type(scope): subject`, for example `feat(billing): add vat`. Tools read it to build changelogs and version numbers.

**DCO** (Developer Certificate of Origin): A short statement that you wrote the code, or have the right to contribute it. You accept it by signing off your commits (`git commit -s`), which adds a `Signed-off-by:` line.

**Draft pull request**: A pull request marked as not ready for review yet. ship opens drafts when nobody reviewed the plan in a terminal (`--yes`, an AI agent).

**Exit code**: The number a command returns when it ends: `0` means success. ship's codes are listed in the [command reference](reference/cli.md).

**Fork**: Your own copy of someone else's repository on GitHub. You push branches to your fork and open pull requests from it.

**Gate**: A check of your project that ship runs before pushing: lint, type check, tests. A failing gate keeps your work on your computer.

**Gitmoji**: An emoji that shows the kind of change at a glance, such as ✨ for a feature or 🐛 for a fix. ship puts it where your repository's history puts it.

**Issue**: A task, bug or idea tracked on GitHub. A pull request can close one when merged (`Closes #42`).

**Label**: A tag on a pull request or issue, such as `kind/feature` or `size/S`, used to sort and filter them.

**MCP**: The Model Context Protocol: how AI coding agents (Claude Code, VS Code agents…) call tools. `ship mcp` makes ship one of those tools.

**Merge**: Adding the commits of one branch into another. On GitHub, merging a pull request puts your work into the base branch.

**Milestone**: A group of issues with an optional due date, for example a phase of a roadmap.

**Operation log**: ship's append-only record of the operations it ran, and the checkpoint before each. `ship log` reads it, `ship undo` reverts the last one.

**Plan**: What ship would do with your work: the title, the branch, the files. `ship plan` shows it without changing anything.

**Pull request** (PR): A request to merge your branch into the base branch. Teammates review it on GitHub before it is merged.

**Push**: Sending your commits from your computer to GitHub.

**Rebase**: Replaying your commits on top of the latest base branch, as if you had started from it. ship does it before publishing when the base has moved.

**Release**: A published version of the project, usually a tag with notes. ship writes the version and the changelog entry; GitHub publishes the release.

**Remote**: A repository on another computer, usually GitHub, that your commits are pushed to. `origin` is the remote you cloned from.

**Ruleset**: Rules a repository imposes on GitHub, for example a required branch name pattern. ship reads them before pushing.

**Secret**: A password, token or key in your code. ship refuses to commit or push one.

**Sign-off**: The `Signed-off-by:` line `git commit -s` adds to a commit message, by which you accept the DCO.

**Squash merge**: Merging a pull request as a single commit on the base branch, so the pull request title becomes the commit message. ship writes titles that read well that way.

**Stack** (stacked pull requests): Several pull requests that build on each other, each based on the branch before it, so a large change is reviewed in small pieces. `ship stack` plans and publishes them.

**Tag**: A name for one commit, usually a version such as `v1.2.0`. ship reads the latest one to pick the next version.

**Undo**: `ship undo` reverts the last thing ship did (a commit, a publication, a configuration change) and keeps your files.

**Upstream**: The repository a fork was copied from. A pull request from your fork targets its upstream.
