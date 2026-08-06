#!/usr/bin/env bash

set -e

make -j 6 pre-ui
make -j 6 generate
podman run --rm -v "$(pwd):/src" -v ~/.pnpm-store:/root/.pnpm-store \
  -w /src/ui/v2.5 node:24-slim bash -c "corepack enable && pnpm run build"
make -j 6 build-release

