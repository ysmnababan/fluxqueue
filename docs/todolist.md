# 1) Portfolio-worthy features (MUST implement)

These are the pieces recruiters/engineers expect to see in a production-grade background processing system:

* **HTTP API** to enqueue immediate tasks and schedule delayed tasks (JSON).
* **Worker pool** implementation in Go demonstrating goroutines, semaphores/bounded concurrency, WaitGroup, context cancellation.
* **Redis-backed queue** (use Redis Streams or LIST+ZSET for scheduled jobs) with visibility/lease or Streams consumer group semantics.
* **Scheduler** that moves scheduled jobs into the ready queue at the right time.
* **Retries + Exponential backoff + jitter** and a **Dead-Letter Queue (DLQ)** for failed tasks.
* **Handler registry** / pluggable handlers so adding a new task type is trivial.
* **Idempotency keys** and a simple idempotency mechanism for safe retries.
* **Graceful shutdown**: stop accepting tasks, drain inflight tasks or requeue safely.
* **Prometheus metrics & basic Grafana dashboard screenshots** (or simple metrics output).
* **Integration tests** (docker-compose for Redis) and unit tests for core logic.
* **Dockerfile + docker-compose** for local dev (Redis + app).
* **README** with quickstart, architecture, how to run tests, and benchmark results.

# 2) To-do list (ordered, actionable)

Follow these tasks in order. Each item can be a separate commit/PR.

### Core MVP

1. Initialize repo, `go mod`, basic README skeleton and `.gitignore`.
2. Create `cmd/queue-server/main.go` to load config and start HTTP server + worker subsystem.
3. Define `Task` model (envelope): `id`, `type`, `payload`, `attempts`, `max_retries`, `idempotency_key`, `timeout`, `meta`.
4. Implement small **HTTP API**:

   * `POST /tasks/enqueue` — enqueue immediate task
   * `POST /tasks/schedule` — schedule task with `run_at` timestamp
   * `GET /tasks/:id` — task status (optional)
5. Implement **Redis queue backend** (start with Lists + ZSET scheduled set):

   * `Enqueue(task)` pushes to `queue:ready`
   * `Schedule(task, runAt)` zadd to `queue:scheduled`
   * `Pop()` BRPOP from ready list
6. Implement **worker pool**:

   * Bounded goroutine pool pulling tasks from Redis and calling handler.
   * Handler registry `RegisterHandler(type string, HandlerFunc)`.
   * Basic handler examples: `send_email` (simulated), `generate_report` (sleep + log).
7. Implement **graceful shutdown**: accept OS signals, cancel context, stop accepting new tasks, close channels, wait for inflight tasks or requeue.

### Reliability & correctness

8. Add **retry logic** with exponential backoff & jitter; re-schedule on failure until `max_retries` then push to DLQ (`queue:dlq`).
9. Implement **idempotency**: store processed `idempotency_key` in Redis with TTL; handlers check before side-effects.
10. Implement a **visibility / in-flight marker** (simple): when pop, add to `processing:<taskID>` key with TTL; if worker dies, scheduler/cleanup reclaims tasks whose processing key expired.
11. Add **scheduler loop**: poll ZSET for tasks with score <= now, move them to ready list atomically (use Lua if needed).

### Observability & ops

12. Integrate **Prometheus**:

    * Counters: `tasks_processed_total`, `tasks_failed_total`, `tasks_retried_total`.
    * Gauges/histograms: `queue_length`, `task_processing_duration_seconds`, `enqueue_latency`.
13. Structured logging (zap/logrus) and basic request logs for API.
14. Add `pprof` and simple health endpoints (`/health`, `/metrics`, `/ready`).

### Testing & CI

15. Write **unit tests** for backoff, scheduler logic, handler registry.
16. Write **integration tests** using docker-compose (start Redis) that enqueue a task and assert it was processed and not left in queue.
17. Add GitHub Actions workflow: `go test ./...` + run integration tests (services via docker-compose).

### Performance & polish (Nice-to-have)

18. Add **batch pop / batch push** optimization for higher throughput.
19. Add **rate limiting / per-type concurrency limit** (e.g., only 2 concurrent `pdf_gen`).
20. Create small **CLI client** (or `curl` examples) and a demo script that enqueues many jobs.
21. Add benchmark/load script (vegeta/k6) and record sample results (graphs/screenshots).
22. Add simple web UI to list queue lengths and DLQ entries (optional).

# 3) Architecture diagram (ASCII) + explanation

```
                   +--------------------+
                   |   Client / CLI     |
                   |  (enqueue/schedule)|
                   +---------+----------+
                             |
                             v
                      +------+-------+
                      | HTTP API     |
                      | (enqueue API)|
                      +------+-------+
                             |
             enqueue -> push|          scheduler polls zset
                             v
                  +----------+----------+
                  |      Redis          |
                  |  - queue:ready (list)   <-- scheduler moves from queue:scheduled
                  |  - queue:scheduled (zset)|
                  |  - queue:dlq (list)      |
                  |  - processing:<taskid>   |
                  +----------+----------+
                             ^
                             |
                 pop / BRPOP |
                             v
                  +----------+----------+
                  | Worker Pool Service |
                  |  (goroutines + sem) |
                  +----+---+---+---+-----+
                       |   |   |   |
     Handler registry  |   |   |   |
   +----------------+  |   |   |   |
   | send_email     |  |   |   |   |
   | pdf_gen        |  |   |   |   |
   | webhook_retry  |  |   |   |   |
   +----------------+  v   v   v   v
                 +----------------------+
                 | External Services    |
                 | SMTP, S3, DB, HTTP   |
                 +----------------------+

Monitoring:
 - /metrics (Prometheus) exposed by API or workers
 - Logs + pprof
```

Explanation:

* Clients call API to enqueue or schedule a task. The API writes to Redis.
* A scheduler process scans `queue:scheduled` (ZSET) for tasks whose run time has come and atomically moves them to `queue:ready` (LIST).
* Workers (many goroutines, maybe many instances) BRPOP from `queue:ready`, mark processing keys, call the appropriate handler, handle retries and DLQ.
* Prometheus scrapes metrics; logs + DLQ provide operational visibility.

# 4) Project folder structure (recommended)

A clear folder layout shows you know how to structure production Go code. Use a feature-driven or layered approach; below mixes clarity & practicality.

```
/ (repo root)
├── .github/
│   └── workflows/ci.yml            # GitHub Actions: unit + integration tests
├── cmd/
│   └── queue-server/
│       └── main.go                 # bootstraps app, loads config, starts API + workers + scheduler
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
└── Makefile
```

### Short description of key files

* `model/task.go` — canonical Task envelope, JSON tags, validation.
* `store/queue.go` — single place for Redis operations: `Enqueue`, `Schedule`, `Pop`, `MoveScheduledToReady`, `PushDLQ`.
* `worker/pool.go` — starts N worker goroutines, channel/semaphore pattern, WaitGroup, shutdown.
* `worker/executor.go` — logic that calls registry, handles retries/backoff, updates metrics.
* `scheduler/scheduler.go` — loop that moves scheduled tasks to ready list and reclaims expired processing keys.
* `api/http/handlers.go` — validates payloads and calls `store.Enqueue` / `store.Schedule`.
* `metrics/metrics.go` — registers counters/histograms and helper functions for instrumentation.

---

## Extra notes (quick tips)

* Start with **simple, correct** behavior: Redis LIST + ZSET for scheduled jobs is easier to reason about. You can later switch to Streams for consumer groups if you want advanced features.
* Keep handlers small and idempotent — use simulated external calls during development.
* Document concurrency decisions in README — point to the code lines where WaitGroup, context, atomic, or semaphores are used.
* Include a short demo script (`scripts/demo_enqueue.sh`) that enqueues a few tasks and tails logs to show end-to-end behavior — reviewers love a one-command demo.

