# 🔐 Go Auth Service — Production-Ready with Full Observability

A production-ready authentication microservice built with Go, featuring structured logging, Prometheus metrics, distributed tracing, rate limiting, and a pre-configured Grafana dashboard — all runnable with a single command.

## Architecture

```
┌─────────┐     ┌──────────────────────────────────────────────┐
│  Client  │────▶│  Auth Service (:8080)                        │
└─────────┘     │  ┌─────────────────────────────────────────┐ │
                │  │ Middleware Stack                          │ │
                │  │ Request ID → Recovery → Logging →        │ │
                │  │ Tracing → Metrics → Rate Limiter → Auth  │ │
                │  └─────────────────────────────────────────┘ │
                │  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
                │  │ Handlers │→ │ Services │→ │Repository │  │
                │  └──────────┘  └──────────┘  └───────────┘  │
                └──────────┬──────────┬──────────┬─────────────┘
                           │          │          │
                    ┌──────▼──┐ ┌─────▼───┐ ┌───▼────┐
                    │Prometheus│ │  Redis   │ │PostgreSQL│
                    │ (:9090)  │ │ (:6379)  │ │ (:5432)  │
                    └──────┬──┘ └─────────┘ └──────────┘
                           │
                    ┌──────▼──┐    ┌─────────┐
                    │ Grafana  │    │  Jaeger  │
                    │ (:3000)  │    │ (:16686) │
                    └─────────┘    └─────────┘
```

## Quick Start

```bash
# Clone and run — that's it
git clone https://github.com/radityajayantara/go-observability-boilerplate.git
cd go-observability-boilerplate
docker compose up -d --build
```

### Access Points

| Service          | URL                        |
| ---------------- | -------------------------- |
| Auth API         | http://localhost:8080       |
| Grafana Dashboard| http://localhost:3000       |
| Jaeger Tracing   | http://localhost:16686      |
| Prometheus       | http://localhost:9090       |

Grafana credentials: `admin` / `admin`

## API Endpoints

### Authentication

```bash
# Register
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepass123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepass123"}'

# Get Profile (protected)
curl http://localhost:8080/auth/me \
  -H "Authorization: Bearer <access_token>"

# Refresh Token
curl -X POST http://localhost:8080/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "<refresh_token>"}'

# Logout (protected)
curl -X POST http://localhost:8080/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

### Operations

```bash
# Liveness
curl http://localhost:8080/health

# Readiness (checks DB + Redis)
curl http://localhost:8080/ready

# Prometheus Metrics
curl http://localhost:8080/metrics
```

## Observability Stack

### Structured Logging (slog)
Every request produces a JSON log entry:
```json
{
  "time": "2024-01-15T10:30:00.000Z",
  "level": "INFO",
  "msg": "request",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "method": "POST",
  "path": "/auth/login",
  "status": 200,
  "duration_ms": 45,
  "remote_addr": "172.18.0.1:52340"
}
```

### Prometheus Metrics
- `http_requests_total` — Request count by method, path, status
- `http_request_duration_seconds` — Latency histogram
- `http_requests_in_flight` — Active requests gauge
- Go runtime metrics (goroutines, memory)

### Distributed Tracing (Jaeger)
Traces propagate through: HTTP → Service → Repository → Database/Redis

### Grafana Dashboard
Pre-configured dashboard with:
- Request rate & error rate
- Latency percentiles (p50, p95, p99)
- Rate limiter 429 responses
- Go runtime (goroutines, memory allocation)

## Rate Limiting
- Algorithm: Sliding window counter (Redis-backed)
- Default: 100 requests per 60 seconds per IP
- Response headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- Configurable via environment variables

## Tech Stack

| Component       | Choice                          |
| --------------- | ------------------------------- |
| Language        | Go 1.22+                        |
| Router          | Chi                              |
| Logging         | slog (stdlib)                    |
| Metrics         | OpenTelemetry → Prometheus       |
| Tracing         | OpenTelemetry → Jaeger           |
| Database        | PostgreSQL 16                    |
| Cache           | Redis 7                          |
| Auth            | JWT (access + refresh tokens)    |
| Dashboard       | Grafana (auto-provisioned)       |
| Load Testing    | K6                               |
| Containerization| Docker + Docker Compose          |

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Start everything
make docker-up

# Stop everything
make docker-down

# Run load test
make load-test
```

## Load Test Results

> Run with K6 against the Docker Compose stack on Apple M-series (8 cores, 16GB RAM).

```
TODO: Run `make load-test` and paste results here after deployment.
```

## Project Structure

```
.
├── cmd/server/main.go          # Entry point + wiring
├── internal/
│   ├── config/                 # Environment-based configuration
│   ├── handler/                # HTTP handlers
│   ├── middleware/              # Request ID, logging, tracing, rate limit, auth, recovery
│   ├── model/                  # Domain models
│   ├── observability/          # Logger, metrics, tracer setup
│   ├── repository/             # PostgreSQL data access
│   └── service/                # Business logic + token management
├── migrations/                 # SQL migrations
├── grafana/                    # Pre-configured dashboards + datasources
├── k6/                         # Load test scripts
├── docker-compose.yml          # Full observability stack
├── Dockerfile                  # Multi-stage build
├── Makefile                    # Developer commands
└── prometheus.yml              # Prometheus scrape config
```

## License

MIT
