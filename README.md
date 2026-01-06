# **FLUXQUEUE**

A high-performance distributed task queue and scheduler built in Go,
using Redis (List + Sorted Set) as the backend.
Designed to demonstrate deep understanding of **Go concurrency**, **distributed
systems**, **job scheduling**, **fault tolerance**, and **worker orchestration**.

> ⚡ Purpose: a real-world portfolio project that showcases backend engineering
> skill in concurrency, message processing, scheduling, and distributed architecture.

---

## 🚀 Features

### **1️⃣ Task Queue (Async Background Jobs)**

- Producers enqueue tasks via HTTP API
- Worker pool executes tasks concurrently using goroutines
- Configurable worker concurrency limits (semaphore-based)
- Task types: email sending, image processing, webhook delivery, data sync, etc.
- Task handlers are pluggable via a registry

---

### **2️⃣ Scheduler (Run Tasks in the Future)**

Uses Redis ZSET to support:

- Scheduled jobs (`run at timestamp`)
- Delayed jobs (`run after 10s`)
- Recurring jobs (`every 1m`)

Scheduler daemon continuously:

1. Reads tasks with `score <= now`
2. Moves them into the queue (Redis List)
3. Signals workers to process them

---

### **3️⃣ Reliability Features**

- Automatic retries with exponential backoff
- Dead Letter Queue (DLQ) for tasks that fail repeatedly
- Idempotency key support to avoid duplicate side effects
- Worker lease/visibility timeout to recover tasks from crashed workers
- Graceful shutdown (drain active jobs before exit)

---

### **4️⃣ Observability**

- Prometheus metrics:
  - `tasks_processed_total`
  - `tasks_failed_total`
  - `task_enqueue_latency_seconds`
  - `task_processing_duration_seconds`

- Structured logging (zerolog or slog)
- Distributed tracing-ready (OTEL optional)

---

### **5️⃣ Developer-Friendly**

- `docker-compose` for local Redis + app
- Clean architecture with layers:

  ```sh
  api → service → queue → worker → store
  ```

- Clear Go interfaces for extensibility
- Integration tests using Testcontainers
- Load test scripts (k6 or Vegeta)

---

## 📐 High-Level Architecture

```sh
                   +-----------------------+
                   |     HTTP API Server   |
                   |  /enqueue, /status    |
                   +-----------+-----------+
                               |
                           Enqueue Task
                               |
                               v
                    +----------+----------+
                    |      Redis          |
                    |   - List: queue     |
                    |   - ZSET: scheduler |
                    +----------+----------+
                               |
     +-------------------------+---------------------------+
     |                                                       |
     v                                                       v
+----+-----+                                         +-------+------+
| Scheduler |                                         | Worker Pool |
|  (ZSET → List)                                      |  Goroutines |
+----+-----+                                         +-------+------+
     |                                                       |
     |                                               Execute Handler
     |                                                       |
     |                                               +-------+------+
     |                                               |   Handlers   |
     |                                               +--------------+
     |
Move due tasks
to ready queue
```

---

## 📦 Project Structure

```sh
/ (repo root)
├── .github/
│   └── workflows/ci.yml            # GitHub Actions: unit + integration tests
├── api/
│   └── http/
│       ├── server.go               # HTTP server setup, routes
│       └── handlers.go             # enqueue/schedule endpoints
├── internal/
│   ├── config/
│   │   └── config.go               # env/config parsing
│   ├── model/
│   │   └── task.go                 # Task struct & validation
│   ├── store/
│   │   ├── redis_client.go         # Redis connection & helpers
│   │   ├── queue.go                # Enqueue/Pop/Schedule/MoveToReady/DLQ
│   │   └── lua_scripts.go          # (optional) atomic lua scripts
│   ├── worker/
│   │   ├── pool.go                 # worker pool implementation
│   │   ├── executor.go             # execute task + retry + backoff
│   │   └── registry.go             # handler registry, RegisterHandler(...)
│   ├── scheduler/
│   │   └── scheduler.go            # scheduled-to-ready mover + reclaim logic
│   ├── metrics/
│   │   └── metrics.go              # prometheus metrics registration
│   └── logging/
│       └── logger.go               # zap/logrus wrapper
├── pkg/
│   └── backoff/
│       └── backoff.go              # backoff algorithm used by workers
├── scripts/
│   ├── demo_enqueue.sh             # demo scripts to enqueue tasks
│   └── load_test.sh                # example vegeta/k6 wrapper
├── deploy/
│   └── docker-compose.yml          # Redis + app (dev)
├── configs/
│   └── dev.yaml                    # example config
├── tests/
│   └── integration/
│       └── redis_integration_test.go
├── docs/
│   └── architecture.md
├── go.mod
├── README.md
├── main.go
└── Makefile
```

---

## 🧪 Running Locally

### 1. Start Redis + services

```sh
docker-compose up -d
```

### 2. Start API server

```sh
go run ./cmd/api
```

### 3. Start worker service

```sh
go run ./cmd/worker
```

### 4. Enqueue a task

```bash
curl -X POST localhost:8080/enqueue \
  -H "Content-Type: application/json" \
  -d '{"type":"email.send", "payload":{"to":"user@example.com"}}'
```

---

## 🔥 Example Use Cases

- Asynchronous email sending
- Generating PDFs or images
- Webhook delivery with retries
- ETL/CDC background tasks
- Data ingestion pipelines
- Scheduled billing tasks
- Cleanup jobs (TTL cleanup, cache invalidation)

---

## 🧱 Roadmap

- Redis Streams backend (optional)
- Dashboard UI for monitoring
- Multi-worker clustering with auto-scaling
- Canary deployment of new handler versions
- Distributed tracing (OpenTelemetry)

---

## 🎯 Why This Project Matters

This system demonstrates:

- **Goroutine orchestration**
- **Channel patterns** (fan-out, rate-limiting, worker pools)
- **Distributed job scheduling**
- **Visibility + retry mechanisms**
- **Backpressure and graceful shutdown**
- **Concurrency safety & performance testing**

---
