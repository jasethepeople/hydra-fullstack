# HYDRA Fullstack

A Go distributed-systems platform combining multi-channel alerting, Redis/NATS-backed job orchestration, and a real-time "living memorial" web app with seasonal theming. Codebase only — the entry point is missing (see Status).

## Features

- **Alerting** (`internal/alerts/`) — severity levels (INFO/WARNING/CRITICAL/RESOLVED) and categories (System/Performance/Error/Security); multi-channel dispatch (Discord, Slack, PagerDuty, SMTP) with severity-based routing rules; alert acknowledgment with audit trail; SQLite-backed history (1000-entry cap) and HTTP API
- **Orchestration** (`internal/orchestrator/`, `internal/queue/`, `internal/natsbus/`, `internal/grpcapi/`) — Redis-backed priority job queue (sorted sets + dead-letter queue), goroutine worker pool with configurable concurrency, circuit breaker, gRPC services (`RenderService`, `AlertService` per `proto/render.proto`), and a NATS/JetStream event bus
- **Infrastructure** (`internal/config/`, `internal/logging/`, `internal/metrics/`, `internal/monitor/`, `internal/storage/`) — environment-based config with validation, Zerolog structured logging with correlation IDs, Prometheus metrics, a health monitor with configurable warning/critical thresholds, and SQLite (WAL mode) persistence
- **Memorial site** (`web/memorial/`, `internal/memorial/`, `internal/dashboard/`) — a canvas-based memorial page with auto-detected/manual seasonal particle effects (spring petals, summer fireflies, fall leaves, winter snow), drag-and-drop media upload, WebSocket chat, a collaborative pixel canvas, and a real-time alert dashboard

## Tech stack

Go 1.22 (module `hydra-fullstack`), SQLite (CGO), Redis, NATS/JetStream, gRPC + Protocol Buffers, Prometheus, Zerolog, vanilla JS + Canvas 2D frontend, Docker, Fly.io (`fly.toml`).

## Getting started

> **Caveat:** the committed tree has no `cmd/` directory — `cmd/server/main.go` does not exist — so the `Makefile`/`Dockerfile` build targets fail as-is. There is also no `migrations/` directory despite the documented project layout.

If the entry point is restored, the documented flow is:

```bash
go mod download
make build   # go build -o bin/server cmd/server/main.go
make run     # starts on http://localhost:8080
```

Configuration is entirely via environment variables (`PORT`, `ENV`, `DISCORD_WEBHOOK`, `SLACK_WEBHOOK`, `PAGERDUTY_KEY`, SMTP settings, `REDIS_ADDR`, `NATS_URL`, `SQLITE_PATH`, threshold overrides — see `internal/config/config.go`). Deployment helpers: `Dockerfile`, `fly.toml`, `scripts/deploy.sh` (Fly.io), `scripts/verify.sh`.

## HTTP API (per the code)

- `GET /health`, `GET /metrics`, `GET /alerts/status`, `GET /dashboard/view`
- `GET /alerts/active`, `GET /alerts/history`, `POST /alerts/acknowledge`, `POST /api/v1/alerts/fire`
- `POST /api/render/stress`
- Memorial: `GET/POST /memorial/entries`, `POST /memorial/upload`, `GET/POST /memorial/canvas`, `GET /memorial/canvas/pixels`, `GET /memorial/season`, `GET /memorial/ws`

## Project structure

```
hydra-fullstack/          (repo nests the project one level down)
  internal/               alerts, config, dashboard, grpcapi, logging, memorial,
                          metrics, monitor, natsbus, orchestrator, queue, storage
  proto/render.proto      RenderService + AlertService definitions
  web/                    memorial/ (index.html + seasonal.js/memorial.js/seasonal.css),
                          dashboard/index.html
  scripts/                deploy.sh (Fly.io), verify.sh
  Dockerfile, fly.toml, Makefile, go.mod
  CHANGELOG.md, CONTRIBUTING.md, LICENSE (MIT)
```

## Status

Substantial but incomplete: all internal packages, the proto definition, web assets, deployment scripts, CHANGELOG, and docs are committed, but **the application entry point (`cmd/server/main.go`) is absent**, so the project does not build or run from this repo. The repo's own README is also partly aspirational (it describes `migrations/` and `cmd/server/main.go`, and links a placeholder `github.com/yourusername` URL). The memorial site is dedicated to Jacquelin Anne Hellenberg (Jackie), per the repo's acknowledgments.
