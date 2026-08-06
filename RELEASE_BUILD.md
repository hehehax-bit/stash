# Release Build

## Prerequisites

- **Node.js >= 20.19** (required by Vite 7). Check with `node --version`.
- **pnpm** (pinned via `corepack` in `ui/v2.5/package.json`).

If your local Node.js is too old (`v18.x`), Vite will fail with `crypto.hash is not a function`. Use the Podman workaround below.

## Standard (Linux/macOS native)

```bash
make pre-ui          # Install UI dependencies
make generate        # Generate Go + GraphQL types
make ui              # Build production frontend
make build-release   # Build release binaries (stash + phasher)
```

Or all at once:

```bash
make release
```

### Podman workaround (Node.js < 20.19)

```bash
make pre-ui
make generate
podman run --rm -v "$(pwd):/src" -v ~/.pnpm-store:/root/.pnpm-store \
  -w /src/ui/v2.5 node:24-slim bash -c "corepack enable && pnpm run build"
make build-release
```

## Cross-compilation

Run the first 3 steps natively, then inside the compiler container:

```bash
make build-cc-all                          # All platforms
make build-cc-linux                        # Linux amd64
make build-cc-windows                      # Windows amd64
make build-cc-macos                        # macOS universal binary
make build-cc-linux-arm64v8                # Linux arm64
make build-cc-linux-arm32v7                # Linux arm v7
make build-cc-linux-arm32v6                # Linux arm v6
make build-cc-freebsd                      # FreeBSD amd64
```

Output goes to `dist/`.

## Docker image

```bash
make docker-build
```

Tags the image as `stash/build`.
