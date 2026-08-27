# mygardenworld

Personal local automation prototype. Daemon + embedded Web UI architecture.

## Build

```sh
make build        # → bin/gardend
make test         # go test ./...
make lint         # golangci-lint v2.13.0, built by system Go 1.27
make proto-gen    # buf generate (requires buf CLI)
```

All Go build, test, lint, generation, and release commands use system Go 1.27.0.

## Architecture

```
cmd/
  gardend        — Long-running daemon (Connect + workspace WS on 127.0.0.1:50051)
  gardencap      — Protocol capture and inspection utility
  gardencatalog  — Catalog generation utility
internal/
  babigame/      — Protocol layer: HTTP login, WS client, envelope crypto
  state/         — In-memory land + inventory + water drops + cultivation tracker
  automation/    — Pure decision engine (Plan → PlannedOp)
  runner/        — Per-account lifecycle: login → WS → state → automation loop
  store/         — SQLite persistence (accounts, sessions, policies, op log)
  apiserver/     — Connect commands + Protobuf workspace WebSocket/read models
proto/           — Protobuf definitions (5 command services + workspace frames)
gen/             — Generated code (do not edit)
web/src/features/workspace/
                — Basic/garden/orders/union/activities/warehouse/statistics/logs UI modules
```

## Key conventions

- Observed protocol behavior is source of truth. See `doc.go` in the protocol package.
- Channel-scoped config: no global defaults. Each account resolves its own Config via ConfigForChannel.
- State is fed namespace fragments from WS responses. Known typed state is domain-oriented and keeps raw snapshots for protocol gaps.
- Policy, planner, runner events, and Web filters share one category set: `basic`, `plant`, `order`, `water`, `union`, `race`, `activity`, plus operational `account` and `system`.
- Policy is stored as one protojson blob in `account_policies.policy_json`; public policy APIs replace/import/export/copy the whole policy.
- Read-side UI state uses one authenticated binary Protobuf WebSocket at `/api/workspace`; Connect is reserved for explicit commands. Workspace frames are versioned exactly, snapshots and domain patches are sequenced, and no compatibility reservations are retained.
- Automation runs every 4s (configurable). Priority: hard state/resource gates > harvest > plant/order deficits > water > orders/flower art > cultivate/upgrade > basic rewards > union > market/pearl/shop/zoo > activity.
- Every mutating action with gold, diamond, item, or count cost must pass observed-state resource gates. Diamond-cost operations are blocked by default unless explicitly implemented. Each land watering costs 1 drop.

## Testing

E2E tests only run when credentials are provided via env vars:
```sh
E2E_USERNAME=<game-account> E2E_PASSWORD=<game-password> go test -v -run "E2E" ./internal/babigame/
```

## Protocol reference

See `internal/babigame/doc.go` for the complete namespace/RPC reference integrated into Go doc comments.
