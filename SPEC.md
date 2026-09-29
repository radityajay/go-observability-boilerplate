# SPEC: Go Auth Microservice with Full Observability

## Overview
Sebuah authentication microservice production-ready yang dibangun dengan Go, dilengkapi rate limiting middleware dan full observability stack. Tujuan utama: portfolio piece untuk rekruter US market.

## Tech Stack
| Komponen | Pilihan | Alasan |
|----------|---------|--------|
| Language | Go 1.22+ | Target market |
| Router | Chi | Lightweight, idiomatic, middleware-friendly |
| Logging | slog (stdlib) | Modern, structured, zero-dependency |
| Metrics | OpenTelemetry SDK → Prometheus | Industry standard |
| Tracing | OpenTelemetry SDK → Jaeger | Distributed tracing |
| Database | PostgreSQL | User data storage |
| Cache/State | Redis | Rate limiter state + token blacklist |
| Auth | JWT (access + refresh token) | Stateless auth |
| Load Test | K6 | Modern, scriptable, presentable results |
| Container | Docker + Docker Compose | One-command setup |

## Endpoints

### Auth
| Method | Path | Auth | Deskripsi |
|--------|------|------|-----------|
| POST | `/auth/register` | Public | Register user baru |
| POST | `/auth/login` | Public | Login, return access + refresh token |
| POST | `/auth/refresh` | Public (with refresh token) | Rotate refresh token |
| POST | `/auth/logout` | Protected | Blacklist current token |
| GET | `/auth/me` | Protected | Get current user profile |

### Ops
| Method | Path | Auth | Deskripsi |
|--------|------|------|-----------|
| GET | `/health` | Public | Liveness probe |
| GET | `/ready` | Public | Readiness probe (cek DB + Redis) |
| GET | `/metrics` | Public | Prometheus metrics endpoint |

## Middleware Stack (urutan)
1. **Request ID** — inject unique request ID ke context + response header
2. **Structured Logging** — log setiap request dengan method, path, status, duration, request_id
3. **Tracing** — OpenTelemetry span per request
4. **Rate Limiter** — sliding window per IP, backed by Redis
5. **Auth (conditional)** — JWT validation untuk protected endpoints
6. **Recovery** — panic recovery → 500 + log

## Rate Limiter
- Algoritma: Sliding window counter
- Storage: Redis
- Default: 100 requests / 60 detik per IP
- Configurable via environment variable
- Return header: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## Auth Flow
- Register: hash password (bcrypt), simpan ke PostgreSQL
- Login: verify password → generate access token (15min) + refresh token (7d)
- Access token: JWT signed with HS256, payload: `{sub, email, exp, iat}`
- Refresh token: opaque token, disimpan di Redis dengan TTL
- Logout: blacklist access token di Redis sampai exp
- Token validation: cek signature → cek blacklist di Redis

## Observability

### Structured Logging (slog)
- Format: JSON
- Fields wajib: `timestamp`, `level`, `msg`, `request_id`, `method`, `path`, `status`, `duration_ms`
- Log level configurable via env var

### Metrics (Prometheus)
- `http_requests_total` — counter per method, path, status
- `http_request_duration_seconds` — histogram per method, path
- `http_requests_in_flight` — gauge
- Go runtime metrics (memory, goroutines) — otomatis dari OpenTelemetry

### Tracing (Jaeger)
- Span per HTTP request
- Span per database query
- Span per Redis operation
- Propagasi context ke semua layer

### Grafana
- Pre-configured dashboard (provisioned via Docker Compose):
  - Request rate & error rate
  - Latency percentiles (p50, p95, p99)
  - Rate limiter hits
  - Go runtime (memory, goroutines)

## Project Structure
```
.
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Env-based configuration
│   ├── handler/
│   │   ├── auth.go              # Auth handlers
│   │   └── health.go            # Health + readiness handlers
│   ├── middleware/
│   │   ├── logging.go           # Structured logging
│   │   ├── tracing.go           # OpenTelemetry tracing
│   │   ├── ratelimit.go         # Redis-backed rate limiter
│   │   ├── auth.go              # JWT auth middleware
│   │   ├── requestid.go         # Request ID injection
│   │   └── recovery.go          # Panic recovery
│   ├── model/
│   │   └── user.go              # User model
│   ├── repository/
│   │   └── user.go              # PostgreSQL user repository
│   ├── service/
│   │   ├── auth.go              # Auth business logic
│   │   └── token.go             # JWT + refresh token logic
│   └── observability/
│       ├── logger.go            # slog setup
│       ├── metrics.go           # Prometheus metrics setup
│       └── tracer.go            # OpenTelemetry tracer setup
├── migrations/
│   └── 001_create_users.sql     # PostgreSQL migration
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── datasource.yml   # Prometheus + Jaeger datasource
│   │   └── dashboards/
│   │       └── dashboard.yml    # Dashboard provisioning config
│   └── dashboards/
│       └── auth-service.json    # Pre-built Grafana dashboard
├── k6/
│   └── load-test.js             # K6 load test script
├── docker-compose.yml           # Full stack: app + postgres + redis + prometheus + jaeger + grafana
├── Dockerfile                   # Multi-stage build
├── Makefile                     # make run, make test, make build, make load-test
├── .env.example                 # Environment variable template
├── go.mod
├── go.sum
└── README.md                    # Dengan hasil load test
```

## Docker Compose Services
| Service | Port | Deskripsi |
|---------|------|-----------|
| app | 8080 | Auth microservice |
| postgres | 5432 | User database |
| redis | 6379 | Rate limiter + token blacklist |
| prometheus | 9090 | Metrics scraping |
| jaeger | 16686 (UI), 4318 (OTLP) | Distributed tracing |
| grafana | 3000 | Dashboard (auto-provisioned) |

## Makefile Targets
- `make run` — jalankan service lokal (tanpa Docker)
- `make build` — build binary
- `make test` — jalankan unit test
- `make docker-up` — `docker compose up -d --build`
- `make docker-down` — `docker compose down`
- `make load-test` — jalankan K6 load test
- `make migrate` — jalankan database migration
- `make lint` — jalankan golangci-lint

## Environment Variables
| Var | Default | Deskripsi |
|-----|---------|-----------|
| `PORT` | `8080` | Server port |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `DB_URL` | `postgres://user:pass@localhost:5432/auth?sslmode=disable` | PostgreSQL connection |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection |
| `JWT_SECRET` | (required) | JWT signing secret |
| `JWT_ACCESS_TTL` | `15m` | Access token TTL |
| `JWT_REFRESH_TTL` | `168h` | Refresh token TTL (7d) |
| `RATE_LIMIT_MAX` | `100` | Max requests per window |
| `RATE_LIMIT_WINDOW` | `60s` | Rate limit window |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP endpoint (Jaeger) |

## Acceptance Criteria
1. `docker compose up` → semua service running, Grafana accessible di :3000
2. Register → Login → Access /auth/me → Refresh → Logout flow works end-to-end
3. Rate limiter blocks setelah exceed limit, return 429 + proper headers
4. Setiap request menghasilkan: log entry (JSON), metric update, trace span
5. Grafana dashboard menampilkan data real-time
6. K6 load test bisa dijalankan dan hasilnya terdokumentasi di README
7. `make test` pass dengan coverage ≥ 70%
