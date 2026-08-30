# mygardenworld

Personal local automation prototype. One `gardend` daemon owns game sessions, automation, persistence, Connect commands, the Protobuf workspace WebSocket, and the embedded Web UI.

## Toolchain

- Use system Go 1.27.0 for every Go build, test, lint, generation, and release command.
- The Web UI uses Node.js 22, pnpm 10, Next.js 16.3, React 19, and Tailwind CSS 4.
- Generated files under `gen/` and `web/src/gen/` must not be edited by hand.

```sh
make build            # bin/gardend
make test             # go test -count=1 ./...
make test-race        # go test -race -count=1 ./...
make lint             # golangci-lint v2.13.0
make proto-gen        # Go protobuf/connect code
make proto-gen-web    # TypeScript protobuf code
make frontend:test
make frontend:lint
make frontend:build
make check
```

## Architecture

```text
cmd/
  gardend/       daemon, CLI, embedded Web entrypoint
  gardencap/     protocol capture and inspection utility
  gardencatalog/ catalog and protocol artifact generator
internal/
  babigame/      channel login, HTTP/WS client, envelope crypto, observed RPCs
  state/         in-memory authoritative facts and typed domain views
  automation/    pure planning and resource-gated PlannedOp selection
  runner/        per-account session, sync, automation, execution, recovery
  store/         SQLite accounts, encrypted credentials/sessions, policies, logs
  apiserver/     auth, Connect commands, workspace WebSocket, read models
proto/           current mygardenworld.v1 command, policy, and workspace schemas
gen/             generated Go protobuf/connect code
web/             embedded Next.js control panel
```

`internal/auth`, `internal/updater`, `internal/captureanalysis`, `internal/cataloggen`, and `internal/webui` contain bounded supporting services; keep executable entrypoints thin and place reusable behavior in `internal/` packages.

## Product boundaries

- Supported game channels are only iOS and Alipay. Alipay login is QR-driven and must not ask for a manual game username.
- The Web product has eight top-level workspaces: basic, garden, orders, union, activities, warehouse, statistics, and logs.
- Each business workspace owns its status and settings. Warehouse is inventory-only; statistics contains aggregated history; logs contains structured execution/runtime records and no settings.
- Union land, construction, and race behavior require authoritative current-cycle membership evidence. Never plan union work from stale snapshots for an account that is not confirmed as a member.
- Supported activities are 花笺集芳 (`tmpType 4002`) and 莳花纪闻 (`tmpType 4003`). Removed activities must not remain in policy, state, API, or UI compatibility paths.

## Protocol and runtime invariants

- Observed game behavior is the source of truth. Keep the namespace/RPC reference in `internal/babigame/doc.go` aligned with captures, implementation, and tests.
- Each account resolves channel-scoped configuration through `ConfigForChannel`; there are no global channel defaults.
- The runner owns the only game connection for an account. Workspace reads reuse its Session and in-memory state; they must not open a second game connection or fetch snapshots through SQLite.
- Read-side account status, views, patches, log pagination, and Alipay progress use the authenticated binary Protobuf WebSocket at `/api/workspace`. Connect is for explicit commands such as account mutation, lifecycle actions, and policy saves.
- Workspace frames are strictly sequenced and versioned. `mygardenworld.v1` is the current protocol, not a compatibility alias.
- State consumes namespace fragments, preserves raw observations for protocol gaps, and exposes typed domain views. Do not move automation decisions into API or Web presentation code.
- Policy, planner, runner events, and Web filters share `basic`, `plant`, `order`, `water`, `union`, `race`, and `activity`; operational events use `account` and `system`.
- Automation normally evaluates every 4 seconds. Hard state/resource gates precede harvest, planting/order deficits, watering, orders/flower art, cultivation/upgrades, basic rewards, union, secondary systems, and activities.
- Every mutating operation with gold, diamond, item, water-drop, or count cost must pass observed-state resource gates. Diamond-cost operations remain blocked unless explicitly and safely implemented. Watering one land consumes one drop.

## Breaking changes and persistence

- This prototype does not carry runtime backward compatibility. Do not add deprecated fields, Protobuf `reserved` declarations, legacy decoders, old policy aliases, or parallel API versions unless explicitly requested.
- Breaking schema work stays in `mygardenworld.v1`. Regenerate both Go and TypeScript outputs and update all callers atomically.
- SQLite uses transactional, ordered `PRAGMA user_version` migrations and currently targets schema v6. A database schema change requires a one-way migration and tests; unversioned legacy databases remain rejected.
- Policy is one strict protojson document in `account_policies.policy_json`. Public replace/import/export/copy operations handle the whole current policy.
- Credentials and recoverable Sessions are encrypted with `garden.db.key`. Session restore is preferred; invalid server sessions fall back to the channel login flow.

## Web UI

- Read `web/AGENTS.md` before changing Next.js code; it contains version-specific framework rules maintained by Next.js.
- Keep domain modules under `web/src/features/workspace/` and shared primitives under `web/src/components/` or `workspace/shared/`.
- Preserve the compact cloud UI language, shared setting rows, responsive bounds, dark theme, keyboard access, and `prefers-reduced-motion` behavior.
- Use ambient effects only for navigation, overview cards, dialogs, and empty states. Logs, tables, and dense settings must remain stable and easy to scan.
- Do not add heavy animation dependencies for interactions that CSS can implement.

## Testing

- Add table-driven tests beside state, automation, runner recovery, store migration, and API behavior changes.
- Frontend changes must pass `pnpm --dir web lint`, `pnpm --dir web test`, and `pnpm --dir web build`.
- Full E2E game tests run only with explicit credentials:

```sh
E2E_USERNAME=<game-account> E2E_PASSWORD=<game-password> \
  go test -v -run E2E ./internal/babigame/
```

## Repository documentation

- Do not create `docs/` directories or standalone implementation plans. Current behavior belongs in code, Go doc comments, Protobuf comments, and tests.
- Keep the root `README.md` concise and user-facing. Keep durable contributor constraints here.
- `CLAUDE.md` imports this file and must not duplicate its contents.
