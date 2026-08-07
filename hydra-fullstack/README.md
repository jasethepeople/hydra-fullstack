# HYDRA Fullstack v1.0

> **A Production-Grade Distributed Systems Platform with Integrated Alerting, Orchestration, and Living Memorial Technology**

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/docker-supported-blue.svg)](Dockerfile)
[![Fly.io](https://img.shields.io/badge/deploy-fly.io-purple.svg)](https://fly.io)

---

## Table of Contents

- [Abstract](#abstract)
- [Architecture](#architecture)
- [System Components](#system-components)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [API Reference](#api-reference)
- [Deployment](#deployment)
- [Development](#development)
- [Testing](#testing)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgments](#acknowledgments)

---

## Abstract

HYDRA Fullstack is a horizontally-scalable, production-grade distributed systems platform built in Go. It integrates four distinct architectural layers into a cohesive system:

1. **Infrastructure Layer** — Structured logging, Prometheus metrics, circuit breakers, graceful shutdown
2. **Orchestration Layer** — Redis-backed priority job queues, goroutine worker pools, gRPC APIs, NATS event bus
3. **Alerting Layer** — Multi-channel alert routing (Discord, Slack, PagerDuty, SMTP), SQLite persistence, real-time dashboard
4. **Application Layer** — Living memorial site with seasonal theming, collaborative canvas, WebSocket chat, media uploads

The system is designed for academic research in distributed systems, cloud-native architecture, and real-time collaborative applications. It demonstrates production patterns including backpressure, circuit breaking, structured observability, and event-driven architecture.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           HYDRA FULLSTACK v1.0                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Layer 4: Application — Jackie Memorial Site                         │   │
│  │  • Seasonal canvas effects (WebGL/Canvas 2D)                        │   │
│  │  • Photo/video/text upload with drag-and-drop                       │   │
│  │  • Real-time WebSocket community chat                               │   │
│  │  • Collaborative pixel canvas (CRDT-like)                           │   │
│  │  • Auto-detected + manual season switching                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Layer 3: Alerting — Multi-Channel Notification System               │   │
│  │  • Discord/Slack/PagerDuty/SMTP multiplexing                        │   │
│  │  • Severity-based routing rules (CRITICAL → all channels)           │   │
│  │  • Alert acknowledgment + persistent history (SQLite)               │   │
│  │  • Real-time web dashboard with auto-refresh                        │   │
│  │  • Prometheus metrics integration                                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Layer 2: Orchestration — Distributed Job Processing                 │   │
│  │  • Redis-backed priority job queue (sorted sets)                    │   │
│  │  • Goroutine worker pool with configurable concurrency              │   │
│  │  • Circuit breaker pattern for external services                    │   │
│  │  • gRPC API for inter-service communication                         │   │
│  │  • NATS/JetStream event bus for cross-service events                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Layer 1: Infrastructure — Hardened Foundation                       │   │
│  │  • Structured logging (Zerolog) with correlation IDs                │   │
│  │  • Prometheus metrics export (/metrics endpoint)                    │   │
│  │  • Environment-based configuration with validation                  │   │
│  │  • Graceful shutdown with 30s timeout + drain                       │   │
│  │  • SQLite with WAL mode for concurrent access                       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Language | Go 1.22+ | Core runtime |
| Database | SQLite 3 (WAL mode) | Persistent storage |
| Queue | Redis | Job queue backend |
| Events | NATS/JetStream | Event bus |
| RPC | gRPC + Protocol Buffers | Inter-service API |
| Metrics | Prometheus | Observability |
| Logging | Zerolog | Structured logging |
| Frontend | Vanilla JS + Canvas 2D | Memorial site |
| Deployment | Docker + Fly.io | Container orchestration |

---

## System Components

### 1. Alert Manager (`internal/alerts/`)

The alert manager provides enterprise-grade alerting with multi-channel dispatch:

- **Severity Levels**: INFO, WARNING, CRITICAL, RESOLVED
- **Categories**: System, Performance, Error, Security
- **Channels**: Discord (rich embeds), Slack, PagerDuty, SMTP
- **Routing Rules**: Configurable severity→channel mapping
- **Persistence**: SQLite with 1000-entry history limit
- **Acknowledgment**: User acknowledgment with audit trail

**Key Files:**
- `internal/alerts/types.go` — Core alert types and enums
- `internal/alerts/manager_v2.go` — Alert creation, routing, dispatch
- `internal/alerts/handlers_v2.go` — HTTP API with Prometheus metrics

### 2. Health Monitor (`internal/monitor/`)

Continuous system health monitoring with configurable thresholds:

| Metric | Warning | Critical | Check Interval |
|--------|---------|----------|----------------|
| Memory Usage | 80% | 92% | 5s |
| Render Latency (p95) | 3s | 5s | 5s |
| Error Rate | 5% | 15% | 5s |

**Key Files:**
- `internal/monitor/health_v2.go` — Monitoring loop, percentile calculation

### 3. Worker Pool (`internal/orchestrator/`)

Distributed job processing with resilience patterns:

- **Redis Queue**: Priority-based sorted sets with dead letter queue
- **Worker Pool**: Configurable goroutine count (default: 4)
- **Circuit Breaker**: Automatic failover for external services
- **Retry Logic**: Exponential backoff with 3-attempt limit

**Key Files:**
- `internal/orchestrator/worker_pool.go` — Worker goroutines, job execution
- `internal/orchestrator/circuit_breaker.go` — Circuit breaker pattern

### 4. Memorial Site (`web/memorial/`)

A living digital memorial with seasonal theming:

- **Seasonal Engine**: Canvas-based particle effects (spring petals, summer fireflies, fall leaves, winter snow)
- **Auto-Detection**: Season determined by current date
- **Manual Override**: Click season badge to switch
- **Media Upload**: Drag-and-drop with type validation
- **WebSocket Chat**: Real-time community messaging
- **Collaborative Canvas**: Pixel placement with live sync

**Key Files:**
- `web/memorial/index.html` — Main memorial page
- `web/memorial/static/js/seasonal.js` — Seasonal effects engine
- `web/memorial/static/js/memorial.js` — Interactivity (canvas, chat, upload)
- `web/memorial/static/css/seasonal.css` — Seasonal animations

---

## Installation

### Prerequisites

- Go 1.22 or later
- Docker (optional, for containerized deployment)
- Redis 6+ (optional, for distributed job queue)
- NATS Server 2+ (optional, for event bus)
- Make (optional, for build automation)

### Quick Start

```bash
# Clone the repository
git clone https://github.com/yourusername/hydra-fullstack.git
cd hydra-fullstack

# Download dependencies
go mod download

# Build the server
make build

# Run locally
make run
```

The server will start on `http://localhost:8080` with the following endpoints:

| Endpoint | Description |
|----------|-------------|
| `http://localhost:8080/` | Jackie Memorial Site |
| `http://localhost:8080/dashboard/view` | Alert Dashboard |
| `http://localhost:8080/health` | Health Check |
| `http://localhost:8080/alerts/status` | Alert System Status |
| `http://localhost:8080/metrics` | Prometheus Metrics |

---

## Configuration

All configuration is via environment variables. See `internal/config/config.go` for full schema.

### Required Variables

```bash
export PORT=8080                    # HTTP server port
export ENV=development              # environment: development | production
```

### Optional Variables

```bash
# Alert Channels
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/..."
export SLACK_WEBHOOK="https://hooks.slack.com/services/..."
export PAGERDUTY_KEY="your-pagerduty-key"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT=587
export SMTP_USER="your-email"
export SMTP_PASS="your-password"
export ALERT_EMAIL="alerts@example.com"

# Queue (Redis)
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD=""
export REDIS_DB=0
export WORKER_COUNT=4

# Event Bus (NATS)
export NATS_ENABLED=true
export NATS_URL="nats://localhost:4222"

# Storage
export SQLITE_PATH="./hydra.db"
export DB_WAL_ENABLED=true

# Memorial
export MEMORIAL_SEASONAL=true
export MEMORIAL_MAX_UPLOAD_MB=50
export MEMORIAL_WEBSOCKET=true
export MEMORIAL_CANVAS=true

# Metrics
export METRICS_ENABLED=true
export METRICS_PORT=9090
```

### Threshold Configuration

```bash
# Alert Thresholds (percentages or seconds)
export MEMORY_WARNING_THRESHOLD=80.0
export MEMORY_CRITICAL_THRESHOLD=92.0
export LATENCY_WARNING_THRESHOLD=3.0
export LATENCY_CRITICAL_THRESHOLD=5.0
export ERROR_WARNING_THRESHOLD=5.0
export ERROR_CRITICAL_THRESHOLD=15.0
```

---

## Usage

### Running the Server

```bash
# Basic run
make run

# With Discord alerts
DISCORD_WEBHOOK=https://discord.com/api/webhooks/xxx make run

# With Redis queue
REDIS_ADDR=localhost:6379 make run

# Production mode
ENV=production make run
```

### API Examples

**Fire an Alert:**
```bash
curl -X POST http://localhost:8080/api/v1/alerts/fire \
  -H "Content-Type: application/json" \
  -d '{
    "alert_type": "High Memory Usage",
    "severity": "WARNING",
    "category": "System",
    "message": "Memory at 85%",
    "value": 85.0,
    "threshold": 80.0,
    "unit": "%"
  }'
```

**Submit a Render Job:**
```bash
curl -X POST http://localhost:8080/api/render/stress \
  -H "Content-Type: application/json" \
  -d '{
    "width": 3840,
    "height": 2160,
    "complexity": "high",
    "iterations": 1000
  }'
```

**Acknowledge an Alert:**
```bash
curl -X POST http://localhost:8080/alerts/acknowledge \
  -H "Content-Type: application/json" \
  -d '{
    "alert_id": "alt-1234567890-High Memory Usage",
    "acked_by": "admin"
  }'
```

**Get Active Alerts:**
```bash
curl http://localhost:8080/alerts/active | jq
```

**Get Alert History:**
```bash
curl "http://localhost:8080/alerts/history?limit=10&offset=0" | jq
```

### gRPC API

```bash
# Check health
grpcurl -plaintext localhost:50051 hydra.RenderService/HealthCheck

# Get worker stats
grpcurl -plaintext localhost:50051 hydra.RenderService/GetWorkerStats

# Submit render job
grpcurl -plaintext -d '{"width":1920,"height":1080,"complexity":"medium","iterations":500}' \
  localhost:50051 hydra.RenderService/SubmitRender

# Stream alerts
grpcurl -plaintext localhost:50051 hydra.AlertService/StreamAlerts
```

---

## API Reference

### HTTP Endpoints

#### Health & Status
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Service health check |
| GET | `/alerts/status` | Alert system configuration status |
| GET | `/api/metrics` | Prometheus metrics (JSON) |

#### Alerts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/alerts/history` | Paginated alert history |
| GET | `/alerts/active` | Non-acknowledged active alerts |
| POST | `/alerts/acknowledge` | Acknowledge an alert by ID |
| POST | `/api/v1/alerts/fire` | Manually fire an alert |

#### Render
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/render/stress` | Submit stress render job |

#### Memorial
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/memorial/entries` | Get memorial entries |
| POST | `/memorial/entries` | Create text entry |
| POST | `/memorial/upload` | Upload photo/video |
| GET | `/memorial/canvas` | Get canvas state |
| POST | `/memorial/canvas` | Place pixel |
| GET | `/memorial/canvas/pixels` | Get all pixels |
| GET | `/memorial/season` | Get current season |
| GET | `/memorial/ws` | WebSocket chat endpoint |

#### Dashboard
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/dashboard/view` | Alert dashboard HTML |

### gRPC Services

See `proto/render.proto` for full service definitions.

**RenderService:**
- `SubmitRender` — Queue a render job
- `GetJobStatus` — Check job status
- `StreamJobLogs` — Stream real-time logs
- `GetWorkerStats` — Get worker pool statistics
- `HealthCheck` — Service health

**AlertService:**
- `FireAlert` — Trigger an alert
- `GetActiveAlerts` — Get active alerts
- `AcknowledgeAlert` — Acknowledge alert
- `GetAlertHistory` — Get alert history
- `StreamAlerts` — Real-time alert stream

---

## Deployment

### Fly.io (Recommended)

```bash
# Install flyctl: https://fly.io/docs/hands-on/install-flyctl/

# Login
flyctl auth login

# Set secrets
flyctl secrets set DISCORD_WEBHOOK="your-webhook-url" --app hydra-fullstack

# Deploy
make deploy

# Or use the deploy script
bash scripts/deploy.sh
```

### Docker

```bash
# Build image
make docker

# Run container
make docker-run

# Or manually:
docker build -t hydra-fullstack:latest .
docker run -p 8080:8080 -p 9090:9090 \
  -e DISCORD_WEBHOOK="..." \
  -e REDIS_ADDR="host.docker.internal:6379" \
  hydra-fullstack:latest
```

### Manual (Binary)

```bash
# Build
make build

# Run binary
./bin/server
```

---

## Development

### Project Structure

```
hydra-fullstack/
├── cmd/
│   └── server/           # Application entry point
│       └── main.go
├── internal/             # Private application code
│   ├── alerts/           # Alert management system
│   ├── config/           # Configuration management
│   ├── dashboard/        # Web dashboard handlers
│   ├── grpcapi/          # gRPC server implementation
│   ├── logging/          # Structured logging
│   ├── memorial/         # Memorial site API
│   ├── metrics/          # Prometheus metrics
│   ├── monitor/          # Health monitoring
│   ├── natsbus/          # NATS event bus
│   ├── orchestrator/     # Worker pool & circuit breaker
│   ├── queue/            # Redis job queue
│   └── storage/          # SQLite persistence
├── web/
│   └── memorial/         # Frontend assets
│       ├── index.html
│       └── static/
│           ├── css/
│           └── js/
├── proto/                # Protocol Buffers definitions
├── scripts/              # Deployment & verification
├── deployments/          # K8s manifests (future)
├── migrations/           # Database migrations
├── Dockerfile
├── fly.toml
├── Makefile
├── go.mod
└── README.md
```

### Adding a New Alert Channel

1. Add channel configuration to `internal/config/config.go`
2. Add dispatch method to `internal/alerts/manager_v2.go`
3. Add routing rule to `defaultRoutingRules()`
4. Update `GetStatus()` to report channel status

### Adding a New Seasonal Effect

1. Create particle class in `web/memorial/static/js/seasonal.js`
2. Add season case to `createParticle()`
3. Add CSS variables to `web/memorial/static/css/seasonal.css`
4. Update `getSeasonEffects()` in `internal/memorial/handlers.go`

---

## Testing

```bash
# Run all tests
make test

# Run with race detection
go test -race ./...

# Generate coverage report
make test
open coverage.html

# Manual verification
bash scripts/verify.sh
```

---

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE) for details.

---

## Acknowledgments

- **Jacquelin Anne Hellenberg (Jackie)** — This project includes a living memorial site dedicated to her memory. The seasonal theming and collaborative canvas are designed to create a space where friends and family can share memories year-round.
- Go Team — For the excellent standard library and concurrency primitives
- Redis — For the reliable job queue backend
- NATS — For the lightweight event streaming platform
- Fly.io — For the developer-friendly deployment platform

---

## Citation

If you use this project in academic research, please cite:

```bibtex
@software{hydra_fullstack_2024,
  title = {HYDRA Fullstack: A Distributed Systems Platform},
  author = {Your Name},
  year = {2024},
  url = {https://github.com/yourusername/hydra-fullstack},
  version = {1.0.0}
}
```

---

<p align="center">
  Built with ❤️ for Jackie and the open-source community.
</p>
