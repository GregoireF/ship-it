# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Grégoire Favreau
#
# Dev container of ship-it contributors (CONTRIBUTING.md), also used by
# GitHub Codespaces. The image is the Microsoft Go dev container
# (https://github.com/devcontainers/images/tree/main/src/go): Go 1.27,
# Debian trixie, git from the devcontainers git feature (newer than the
# git 2.43 that `ship doctor` requires), nvm and the non-root user `vscode`.
#
# It lives in a Dockerfile rather than in devcontainer.json "image" so that
# Dependabot (docker ecosystem, .github/dependabot.yml) keeps the digest fresh.
# Tag 2.3.3-1.27-trixie, index digest checked on mcr.microsoft.com 2026-10-23.
FROM mcr.microsoft.com/devcontainers/go:2.3.6-1.27-trixie@sha256:c83c320b68e27dc36903cd5ee07394870c5330dfd77b0897e78c7e1aa00ddfd0
