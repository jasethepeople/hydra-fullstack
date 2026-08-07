# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-05-14

### Added
- **Infrastructure Layer**
  - Structured logging with Zerolog and correlation IDs
  - Prometheus metrics export with 15+ custom metrics
  - Environment-based configuration with validation
  - Graceful shutdown with 30-second timeout
  - SQLite persistence with WAL mode

- **Orchestration Layer**
  - Redis-backed priority job queue with sorted sets
  - Goroutine worker pool with configurable concurrency
  - Circuit breaker pattern for external service resilience
  - gRPC API with Protocol Buffers (RenderService, AlertService)
  - NATS/JetStream event bus for cross-service communication
  - Dead letter queue for failed jobs

- **Alerting Layer**
  - Multi-channel alert dispatch (Discord, Slack, PagerDuty, SMTP)
  - Severity-based routing rules (INFO → Discord, CRITICAL → all)
  - Alert acknowledgment with audit trail
  - Persistent alert history (1000-entry limit)
  - Real-time web dashboard with auto-refresh
  - Manual alert firing via HTTP API
  - Discord rich embeds with color-coded severity

- **Application Layer — Jackie Memorial**
  - Seasonal canvas effects engine (spring/summer/fall/winter)
  - Auto-detected season based on current date
  - Manual season override with click interaction
  - Photo/video/text upload with drag-and-drop
  - Real-time WebSocket community chat
  - Collaborative pixel canvas with live sync
  - Responsive design for mobile and desktop

- **Deployment**
  - Multi-stage Dockerfile with Alpine Linux
  - Fly.io deployment configuration
  - Automated deploy script with health checks
  - Verification script for endpoint testing

- **Documentation**
  - Comprehensive README with architecture diagrams
  - API reference for HTTP and gRPC endpoints
  - Configuration reference with all environment variables
  - Academic citation in BibTeX format

### Security
- Input validation on all API endpoints
- XSS protection via HTML escaping in frontend
- File type validation on uploads
- CORS middleware for cross-origin requests

## [Unreleased]

### Planned
- Kubernetes deployment manifests
- PostgreSQL backend option
- OAuth2 authentication
- Rate limiting per user
- Distributed tracing with OpenTelemetry
- WebRTC video streaming for memorial
- Machine learning-based anomaly detection
