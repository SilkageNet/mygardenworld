# mygardenworld

Personal local automation prototype. Daemon + CLI architecture.

## Build

```sh
make build        # → bin/gardend, bin/gardenctl
make test         # go test ./...
make lint         # golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
make proto-gen    # buf generate (requires buf CLI)
```

## Architecture

```
cmd/gardend      — Long-running daemon (gRPC server on 127.0.0.1:50051)
cmd/gardenctl    — Operator CLI (talks to gardend via Connect/gRPC)
internal/
  babigame/      — Protocol layer: HTTP login, WS client, envelope crypto
  state/         — In-memory land + inventory + water drops + cultivation tracker
  automation/    — Pure decision engine (Plan → PlannedOp)
  runner/        — Per-account lifecycle: login → WS → state → automation loop
  store/         — SQLite persistence (accounts, sessions, policies, op log)
  apiserver/     — gRPC/Connect service implementations
proto/           — Protobuf definitions (6 services + Channel enum)
gen/             — Generated code (do not edit)
```

## Key conventions

- Observed protocol behavior is source of truth. See `doc.go` in the protocol package.
- Channel-scoped config: no global defaults. Each account resolves its own Config via ConfigForChannel.
- State is fed namespace fragments from WS responses. Namespaces 7, 100, 101, 109, 114 are tracked.
- Automation runs every 4s (configurable). Priority: harvest > plant > water. Misc ops (orders, tasks, cultivation) run every 60s.
- Water drops are checked before watering. Each land costs 1 drop.

## Testing

E2E tests only run when credentials are provided via env vars:
```sh
E2E_USERNAME=<game-account> E2E_PASSWORD=<game-password> go test -v -run "E2E" ./internal/babigame/
```

## Protocol reference

See `internal/babigame/doc.go` for the complete namespace/RPC reference integrated into Go doc comments.
