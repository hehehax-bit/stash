# AGENTS.md

## Build & Verify Pipeline

Sequential order matters. Generated files are gitignored — you cannot build or lint without running generation first.

```bash
make pre-ui          # pnpm install --frozen-lockfile (once, or when deps change)
make generate        # generates backend Go code + frontend GraphQL types
make build           # builds stash + phasher binaries (requires generated files + ui/v2.5/build/index.html to exist)
make lint            # golangci-lint (requires generate-backend first — CI does this)
make it              # integration tests: go test -tags "$(GO_BUILD_TAGS) integration" ./... (requires generated files)
make validate        # everything: validate-ui + validate-backend (lint + integration tests)
```

Frontend-only verification:
```bash
cd ui/v2.5
pnpm run validate    # biome check + stylelint + tsc --noEmit
```

Single test run (Go, package-scoped):
```bash
go test ./pkg/sqlite/...               # single package
go test -run TestSceneFind ./pkg/sqlite/...  # single test
# integration tests for a package — sqlite_math_functions is required (random sort uses mod())
go test -tags "integration sqlite_stat4 sqlite_math_functions" ./pkg/sqlite/...
```

## Code Generation

Generation is multi-step and order-dependent:

1. `make generate-backend` — runs `touch-ui` (creates `ui/v2.5/build/index.html` if missing), then `go generate ./cmd/stash` (gqlgen) and dataloaden loaders
2. `make generate-ui` — runs `pnpm run gqlgen` in `ui/v2.5/` (produces `src/core/generated-graphql.ts`)

Generated outputs (all gitignored):
- `internal/api/generated_exec.go`, `internal/api/generated_models.go` — backend GraphQL resolvers/models
- `ui/v2.5/src/core/generated-graphql.ts` — frontend TypeScript types
- `ui/login/locales/*` — login page locales
- `internal/api/loaders/*_generated.go` — dataloaden loaders
- `pkg/stashbox/graphql/generated_*.go` — stash-box client (separate: `make generate-stash-box-client`)

Mock generation: `make generate-test-mocks` → outputs to `pkg/models/mocks/`

## Architecture Quick-Reference

- `cmd/stash/main.go` — main entrypoint; has `//go:generate go run github.com/99designs/gqlgen`
- `cmd/phasher/` — standalone perceptual hash utility
- `internal/api/` — GraphQL resolvers (`resolver_*.go`), HTTP server, generated code
- `internal/manager/` — core application services and task implementations
- `pkg/models/` — interface definitions (`Repository`, `SceneReaderWriter`, etc.) and data structs
- `pkg/sqlite/` — SQLite implementations of `pkg/models/` interfaces; uses goqu + custom query builder
- `graphql/schema/` — GraphQL schema files (types in `types/` subdirectory)
- `ui/v2.5/` — React 17 + TypeScript + Vite frontend, Apollo Client, Bootstrap 4, SCSS

Layering: Resolver → Service (complex entities) or Validation (simple entities) → Repository interface (`pkg/models/`) → SQLite impl (`pkg/sqlite/`)

Full architecture details: `docs/ARCHITECTURE.md`

## Adding a GraphQL Field

1. Define field in `graphql/schema/schema.graphql` or `graphql/schema/types/*.graphql`
2. Run `make generate-backend`
3. Implement resolver in `internal/api/resolver_*.go`
4. If query requires new repository method: add interface to `pkg/models/repository_*.go`
5. Implement in `pkg/sqlite/*.go`
6. Add frontend query in `ui/v2.5/graphql/`
7. Run `make generate-ui`

Update `gqlgen.yml` when adding new types or fields that need explicit model mappings.

## Adding a Database Migration

1. Create `pkg/sqlite/migrations/{version}_description.up.sql`
2. Optionally create `{version}_premigrate.go` and/or `{version}_postmigrate.go` in the same directory
3. Update `appSchemaVersion` in `pkg/sqlite/database.go` to the new version number

Migrations are embedded via `//go:embed migrations/*.sql` and use `golang-migrate/migrate`.

## Testing

- **Unit tests**: `make test` — `go test ./...` (excludes integration tests)
- **Integration tests**: `make it` — adds `integration` build tag via `-tags "integration"`
- **Full PR validation**: `make validate` — runs `validate-ui` then `validate-backend` (lint + integration tests)
- **Frontend**: `cd ui/v2.5 && pnpm run validate` — biome lint + biome format check + stylelint + tsc

## Frontend

Package manager is **pnpm** (not npm). Version pinned in `ui/v2.5/package.json` `packageManager` field.

**Node.js version**: Vite 7 requires Node.js >= 20.19. The local system may have an older version (e.g. 18). A Node.js 24 image is available in Podman (`node:24-slim`). Use it for UI builds when local Node is too old:
```bash
podman run --rm -v "$(pwd):/src" -v ~/.pnpm-store:/root/.pnpm-store -w /src/ui/v2.5 node:24-slim bash -c "corepack enable && pnpm run build"
```
Replace `pnpm run build` with `pnpm run validate` for lint/type-checking, or any other pnpm command as needed. The `~/.pnpm-store` volume mount caches packages across container runs (~687MB).

Toolchain: Vite (build), biome (lint + format), stylelint (SCSS), tsc (type check)

Dev servers:
- `make server-start` — Go backend on `:9999`, runs from `.local/` dir with `STASH_CONFIG_FILE=config.yml`
- `make ui-start` — Vite dev server on `:3000`, requires running backend. Auth cookies don't work cross-origin; disable credentials on the server when developing.

Quick UI validation on changed files only: `make validate-ui-quick` (experimental)

## Key Gotchas

- **CGO_ENABLED=1** is always set in the Makefile — required for SQLite CGO driver
- **Build tags** `sqlite_stat4 sqlite_math_functions` are always appended; static builds add `sqlite_omit_load_extension osusergo netgo`
- **`make generate-backend`** includes `touch-ui` — it creates `ui/v2.5/build/index.html` as a placeholder if missing (needed for `go build` to embed UI assets)
- **`make release`** is sequential: `pre-ui` → `generate` → `ui` → `build-release`. If local Node is too old for Vite 7, run the `ui` step in Podman (see Frontend section) then `make build-release`.
- **golangci-lint** config is in `.golangci.yml` (v2 format). CI runs `golangci-lint-action` v2.11.4
- **AI policy**: See `docs/AI_POLICY.md`. AI-assisted code contributions require disclosure in PR description; fully AI-generated contributions are closed without comment.
- **Mockery** config is in `.mockery.yml` — generates mocks for `.*ReaderWriter` interfaces into `pkg/models/mocks/`

## 🛠 TDD Development Guidelines

### Workflow (TDD Red-Green-Refactor)

1. **Write a failing test first** for the behavior you want.
2. **Run the test** and confirm it fails (RED).
3. **Write the minimal implementation** to make the test pass.
4. **Run the test again** and confirm it passes (GREEN).
5. **Refactor** for clarity, keeping the test green.

### AI Behavioral Rules (must-follow)

- Run tests **after every change**, never skip or guess.
- If a test fails, **stop immediately** — read the failure message, fix the root cause, then re-run.
- Never mark a test as passing unless you have actually run it and seen it pass.
- Never claim a test was fixed without providing the test output.

### Pro Tips

- **Define the run command**: To verify tests, always use the command: `npm test -- [filename]` or `pytest [path]`.
- **Chain-of-Thought Phase Announcement**: Before writing code, state your current phase: **[RED]**, **[GREEN]**, or **[REFACTOR]** and explain what specific behavior you are testing/implementing.
- **Anti-Pattern Warning**: If you provide a test and the implementation in the same response, you are violating the TDD protocol.
