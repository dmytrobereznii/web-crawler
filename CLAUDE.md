# web-crawler — CLAUDE.md

A distributed web crawler built in Go. Submitted URLs are crawled concurrently, extracted links are followed up to a configurable depth, and results are persisted to PostgreSQL. The project grows incrementally — from a simple in-memory API to a multi-service distributed system with observability.

---

## Functionality

### HTTP API
- `POST /crawls` — submit a root URL to crawl; returns a crawl ID and initial status
- `GET /crawls/:id` — get crawl status and results (visited URLs, extracted links, page metadata)

### Crawler
- Concurrent fetcher worker pool — configurable worker count
- In-memory URL frontier via buffered channel
- Visited-URL deduplication via `sync.Map`
- Per-domain rate limiting (token bucket)
- `robots.txt` compliance
- Configurable crawl depth and max pages per crawl
- Graceful shutdown via context cancellation

### Storage
- In-memory store (initial implementation)
- PostgreSQL via `pgx` (persistent implementation)
  - Crawl jobs, visited URLs, extracted page data (title, links, status code, response time)
  - Schema migrations via `golang-migrate`
  - Repository pattern — DB logic isolated from handlers

### Distributed Architecture (later stages)
- Split into `scheduler-service` (HTTP API, URL frontier, deduplication) and `fetcher-service` (concurrent fetching, HTML parsing, result reporting)
- Services communicate via gRPC for synchronous dispatch and Kafka for async URL events
- Idempotent Kafka consumer with deduplication via URL hash
- Failed URL retry with exponential backoff and dead-letter handling

### Infrastructure
- Multi-stage Dockerfile + docker-compose (API + PostgreSQL + Kafka)
- Kubernetes manifests: Deployment, Service, ConfigMap, Secrets
- GitHub Actions CI: lint, test, build, push image on merge
- Readiness and liveness probes, resource requests and limits

### Observability
- Prometheus metrics: crawl submission rate, pages/sec, fetch error rate by domain, fetch latency
- Grafana RED dashboard (Rate, Errors, Duration) for each service
- OpenTelemetry distributed tracing with `trace_id` propagated through Kafka messages

---

## Project Structure

```
web-crawler/
├── cmd/
│   ├── server/             # API entrypoint — main.go only, no business logic
│   └── fetcher/            # fetcher-service entrypoint (later stage)
├── internal/
│   ├── api/                # HTTP handlers and middleware
│   ├── crawler/            # worker pool, frontier, deduplication
│   ├── fetcher/            # HTTP fetching, HTML parsing, link extraction
│   ├── ratelimit/          # per-domain token bucket
│   └── store/              # storage interfaces + implementations
├── pkg/                    # reusable, export-safe packages
├── proto/                  # protobuf definitions (later stage)
├── k8s/                    # Kubernetes manifests (later stage)
├── Makefile
└── go.mod
```

---

## How to Help

**Primary role:** Go code reviewer and technical tutor. The emphasis is on writing idiomatic Go — not just code that works.

**When reviewing code:**
- Call out PHP habits bleeding into Go: class-thinking instead of interfaces, try/catch instincts instead of explicit error returns, ORM-style patterns instead of `pgx`
- Flag non-idiomatic Go even if it compiles — "it works" is not enough
- Explain *why* the idiomatic approach is preferred, not just what it is

**When explaining Go:**
- Assume strong backend experience — skip basics, go straight to the tradeoff or the idiom
- Frame relative to PHP where the comparison is genuinely useful
- Prefer short explanations; expand only if asked

**When asked to write code:**
- Prefer explaining the pattern and letting the developer implement it
- If writing code to unblock, always annotate the Go idiom it demonstrates
- Never produce a working solution without surfacing what makes it idiomatic

---

## Go Standards

**Error handling**
- Wrap with `fmt.Errorf("context: %w", err)` at every boundary — never discard
- No `panic` outside `main` or test helpers
- Sentinel errors (`var ErrNotFound = errors.New(...)`) for errors callers need to inspect

**Interfaces**
- "Accept interfaces, return structs" — enforce at handler and store boundaries
- Define interfaces at the consumer, not the producer
- Keep interfaces small — single-method interfaces are idiomatic and testable

**Concurrency**
- Every goroutine must have a clear, documented exit condition — no leaks
- Channels for communication between goroutines; mutexes for protecting shared state
- `go test -race` must pass — this is non-negotiable, not optional

**HTTP**
- No framework — stdlib `net/http` only
- Handlers are thin: parse input, call a service, encode response
- All error responses return structured JSON, never raw strings or stack traces

**Testing**
- Table-driven tests for all handlers and pure functions
- Dependencies injected via interfaces — if something is hard to test, the design is wrong
- Target 70%+ coverage; 100% on critical paths (deduplication, rate limiting, error branches)

**Logging**
- Structured JSON logs via `zerolog` or `zap` — no `fmt.Println` in production paths
- Include `crawl_id` and `url` in log context wherever relevant

**Configuration**
- All config via environment variables — no hardcoded values
- Use `godotenv` or `viper`; document every variable in `.env.example`
