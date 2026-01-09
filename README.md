# **FLUXQUEUE**

A high-performance distributed task queue and scheduler built in Go, using Redis (List + Sorted Set) as the backend. Designed to demonstrate deep understanding of **Go concurrency**, **distributed systems**, **job scheduling**, **fault tolerance**, and **worker orchestration**.

> ⚡ Purpose: a real-world portfolio project that showcases backend engineering skill in concurrency, message processing, scheduling, and distributed architecture.

---

## 🚀 Features

### **1️⃣ Task Queue (Async Background Jobs)**

- Producers enqueue tasks via HTTP API
- Worker pool executes tasks concurrently using goroutines
- Configurable worker concurrency limits (channel-based backpressure)
- Task types: email sending, Excel report generation, webhook delivery, etc.
- Task handlers are pluggable via a registry pattern

### **2️⃣ Scheduler (Run Tasks in the Future)**

Uses Redis ZSET to support:

- Scheduled jobs (`run at timestamp`)
- Delayed jobs (`run after delay`)
- Automatic retry with exponential backoff

Scheduler daemon continuously:

1. Polls Redis ZSET for tasks with `score <= now`
2. Atomically moves them into the ready queue (Redis List)
3. Workers pick them up immediately

### **3️⃣ Reliability Features**

- Automatic retries with exponential backoff (`baseInterval * 2^(attempts-1)`)
- Dead Letter Queue (DLQ) for tasks that exceed max retries
- Idempotency key support to prevent duplicate execution
- Graceful shutdown (drain active jobs before exit)
- Panic recovery in worker goroutines

### **4️⃣ Observability**

- **Prometheus metrics**:
  - `tasks_enqueued_total`, `tasks_processed_total`, `tasks_failed_total`, `tasks_retried_total`
  - `task_processing_duration_seconds`, `task_enqueue_latency_seconds`
- **Structured logging** with `rs/zerolog`
- **Profiling** via `pprof` endpoints (`/debug/pprof/*`)
- **Grafana dashboards** included in `./dashboard/` (latency, resource usage, main dashboard)

### **5️⃣ Developer-Friendly**

- `docker-compose` setup for local development with hot reload (Air)
- Clean architecture: `api → service → worker → store`
- Mock generation with Mockery for unit tests
- Load testing scripts with k6 (smoke, stress, spike, soak tests)
- Linting with golangci-lint

---

## 📐 High-Level Architecture

FluxQueue operates as two separate processes:

### **API Server (`-mode=api`)**

HTTP server exposing task enqueueing endpoints. Pushes tasks to Redis queues.

### **Worker Processor (`-mode=worker`)**

- **Consumer Goroutines**: BRPOP from Redis `queue:ready`
- **Worker Pool**: Processes tasks with configurable concurrency
- **Scheduler**: Moves due tasks from `queue:scheduled` (ZSET) to `queue:ready` (List)
- **Handler Registry**: Maps task types to handler functions

```sh
                   +-----------------------+
                   |   HTTP API Server     |
                   | /api/v1/enqueue       |
                   | /api/v1/schedule      |
                   +-----------+-----------+
                               |
                         Enqueue Task
                               |
                               v
                    +----------+----------+
                    |      Redis          |
                    |   List: queue:ready |
                    |   ZSET: queue:scheduled |
                    |   List: queue:dead  |
                    +----------+----------+
                               |
     +-------------------------+---------------------------+
     |                                                     |
     v                                                     v
+----+-----+                                      +--------+-------+
| Scheduler|                                      |  Worker Pool   |
| (Ticker) |                                      | (Goroutines)   |
+----+-----+                                      +--------+-------+
     |                                                     |
  Move due tasks                               Execute Handler (Registry)
  to ready queue                                          |
                                                  +-------+-------+
                                                  |   Success     |
                                                  | Retry → ZSET  |
                                                  | Failed → DLQ  |
                                                  +---------------+
```

---

## 📦 Project Structure

```sh
.
├── cmd/                            # (currently empty, can add separate entrypoints)
├── configs/
│   ├── config.yaml                 # Active config file
│   └── example.config.yaml         # Example configuration
├── dashboard/                      # Grafana dashboard JSON files
│   ├── main_dashboard.json         # Overview dashboard
│   ├── latency.json                # Latency metrics dashboard
│   └── resource.json               # Resource usage dashboard
├── deploy/
│   └── (deployment configs)
├── docs/
│   └── (documentation)
├── internal/
│   ├── api/
│   │   ├── grpc/                   # (gRPC handlers, if added)
│   │   └── http/
│   │       ├── handler/            # HTTP request handlers
│   │       ├── middleware/         # Echo middleware (logging, recovery, etc.)
│   │       ├── response/           # Standard response helpers
│   │       └── server.go           # HTTP server setup
│   ├── config/                     # Configuration loading (Viper)
│   ├── logging/                    # Structured logging (zerolog)
│   ├── mail/                       # Email service (SMTP)
│   ├── metrics/                    # Prometheus metrics
│   ├── model/                      # Core data structures (Task, Handler types)
│   ├── processor/                  # Application bootstrap & lifecycle
│   ├── scheduler/                  # Scheduler daemon
│   ├── service/                    # Business logic (email, report generation)
│   ├── storage/                    # MinIO/S3 client
│   ├── store/                      # Redis client & queue operations
│   └── worker/                     # Worker pool & handler registry
├── prom_targets/                   # Prometheus service discovery targets
├── scripts/
│   ├── smoke.js                    # k6: Smoke test (low load, 30s)
│   ├── baseline_load.js            # k6: Baseline load test
│   ├── stress_enqueue.js           # k6: Stress test
│   ├── spike.js                    # k6: Spike test
│   ├── soak.js                     # k6: Soak test (sustained load)
│   └── generate_worker_sd.sh       # Generate Prometheus service discovery for workers
├── utils/
│   └── validator/                  # Custom validator for request validation
├── .air.toml                       # Air config for hot reload
├── .golangci.yml                   # Linter configuration
├── .mockery.yaml                   # Mock generation configuration
├── docker-compose.dev.yml          # Development environment (with MinIO, MailHog, hot reload)
├── docker-compose.yml              # Production-like environment
├── Dockerfile                      # Production Dockerfile
├── Dockerfile.dev                  # Development Dockerfile (with Air)
├── go.mod
├── main.go                         # Application entrypoint
├── Makefile                        # Common commands
└── prometheus.yml                  # Prometheus configuration
```

---

## 🧪 Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- (Optional) k6 for load testing

---

## 🛠️ Development Commands

### **Local Development (with Docker)**

Start the full development stack with hot reload, MinIO, MailHog, Prometheus, and Grafana:

```bash
# Start dev environment with hot reload (Air)
make dev-up

# Start with multiple workers (default: 3)
make dev-up-scaled WORKERS=5

# View logs
make logs

# Stop dev environment
make dev-down

# Start stopped containers
make dev-start

# Stop running containers (without removing)
make dev-stop
```

**Services available:**

- API Server: <http://localhost:8080>
- MailHog UI: <http://localhost:8025>
- MinIO Console: <http://localhost:9091> (minioadmin/minioadmin)
- Prometheus: <http://localhost:9090>
- Grafana: <http://localhost:3000> (admin/admin)

**Import Grafana Dashboards:**

1. Go to Grafana (<http://localhost:3000>)
2. Add Prometheus as data source (<http://prometheus:9090>)
3. Import dashboards from `./dashboard/` directory:
   - `main_dashboard.json` - Overview of all metrics
   - `latency.json` - Task latency analysis
   - `resource.json` - Resource usage monitoring

### **Production (Docker Compose)**

Production-like environment without development tools:

```bash
# Start production stack
make prod-up

# Stop production stack
make prod-down

# Stop without removing containers
make prod-stop
```

---

## 🧪 Testing

### **Unit Tests**

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests in specific package
go test ./internal/worker

# Run specific test
go test ./internal/worker -run TestWorkerPool

# Verbose output
go test -v ./...
```

### **Race Condition Detection**

```bash
# Run tests with race detector
go test -race ./...

# Run tests with race detector and verbose output
go test -race -v ./...

# Test specific package for races
go test -race ./internal/worker
```

### **Load Testing (k6)**

Ensure the API server is running first.

```bash
# Smoke test (sanity check, low load, 30s)
k6 run scripts/smoke.js

# Baseline load test
k6 run scripts/baseline_load.js

# Stress test (high load)
k6 run scripts/stress_enqueue.js

# Spike test (sudden traffic spike)
k6 run scripts/spike.js

# Soak test (sustained load over time)
k6 run scripts/soak.js
```

---

## 🔍 Code Quality

### **Linting**

```bash
# Run linter (uses .golangci.yml config)
golangci-lint run

# Run with auto-fix
golangci-lint run --fix

# Lint specific directory
golangci-lint run ./internal/worker/...
```

### **Mock Generation (for Development)**

Generate mocks for testing using Mockery (configured in `.mockery.yaml`):

```bash
# Generate all mocks
mockery

# Mocks are generated with naming pattern: <InterfaceName>_mock_test.go
# Example: IRedisClient_mock_test.go
```

---

## 🔧 Configuration

Configuration is loaded from `./configs/config.yaml` using Viper. See `./configs/example.config.yaml` for all available options.

**Key config sections:**

- `redis`: Connection settings, worker count (consumer goroutines)
- `server`: HTTP port, environment, service name
- `worker`: Worker pool size, retry settings, scheduler tick interval
- `storage`: MinIO/S3 settings for file storage tasks
- `email`: SMTP settings for email sending tasks

---

## 📊 Example Usage

### Enqueue a Task

```bash
curl -X POST http://localhost:8080/api/v1/enqueue \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email.send",
    "payload": {
      "to": "user@example.com",
      "subject": "Welcome!",
      "body": "<h1>Hello!</h1>"
    },
    "max_retries": 3
  }'
```

### Schedule a Task (delayed execution)

```bash
curl -X POST http://localhost:8080/api/v1/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "type": "report.generate",
    "payload": {
      "report_id": "Q4-2024"
    },
    "run_at": "2026-01-10T10:00:00Z",
    "max_retries": 5
  }'
```

---

## 🔥 Use Cases

- Asynchronous email sending
- Generating Excel reports
- Background data processing
- Webhook delivery with retries
- Scheduled billing tasks
- ETL/CDC pipelines
- Cleanup jobs (cache invalidation, expired data removal)

---

## 🎯 Key Concepts

### **Concurrency Patterns**

- **Fan-out**: Multiple consumers (BRPOP) + multiple workers
- **Buffered channel**: `taskChan` provides backpressure (size = 2 × worker count)
- **Graceful shutdown**: Cancel context → close channel → WaitGroup synchronization

### **Reliability**

- **Exponential backoff**: `delay = baseInterval * 2^(attempts-1)`
- **Idempotency**: Redis SetNX ensures tasks execute exactly once
- **Dead Letter Queue**: Failed tasks moved to `queue:dead` for inspection

### **Distributed Design**

- Stateless workers (can scale horizontally)
- Atomic Redis operations (Lua scripts)
- No distributed locks needed

---
