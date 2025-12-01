# **FLUXQUEUE**

A high-performance distributed task queue and scheduler built in Go, using Redis (List + Sorted Set) as the backend.
Designed to demonstrate deep understanding of **Go concurrency**, **distributed systems**, **job scheduling**, **fault tolerance**, and **worker orchestration**.

> ⚡ Purpose: a real-world portfolio project that showcases backend engineering skill in concurrency, message processing, scheduling, and distributed architecture.

---

## 🚀 Features

### **1️⃣ Task Queue (Async Background Jobs)**

* Producers enqueue tasks via HTTP API
* Worker pool executes tasks concurrently using goroutines
* Configurable worker concurrency limits (semaphore-based)
* Task types: email sending, image processing, webhook delivery, data sync, etc.
* Task handlers are pluggable via a registry

---

### **2️⃣ Scheduler (Run Tasks in the Future)**

Uses Redis ZSET to support:

* Scheduled jobs (`run at timestamp`)
* Delayed jobs (`run after 10s`)
* Recurring jobs (`every 1m`)

Scheduler daemon continuously:

1. Reads tasks with `score <= now`
2. Moves them into the queue (Redis List)
3. Signals workers to process them

---

### **3️⃣ Reliability Features**

* Automatic retries with exponential backoff
* Dead Letter Queue (DLQ) for tasks that fail repeatedly
* Idempotency key support to avoid duplicate side effects
* Worker lease/visibility timeout to recover tasks from crashed workers
* Graceful shutdown (drain active jobs before exit)

---

### **4️⃣ Observability**

* Prometheus metrics:

  * `tasks_processed_total`
  * `tasks_failed_total`
  * `task_enqueue_latency_seconds`
  * `task_processing_duration_seconds`
* Structured logging (zerolog or slog)
* Distributed tracing-ready (OTEL optional)

---

### **5️⃣ Developer-Friendly**

* `docker-compose` for local Redis + app
* Clean architecture with layers:

  ```
  api → service → queue → worker → store
  ```
* Clear Go interfaces for extensibility
* Integration tests using Testcontainers
* Load test scripts (k6 or Vegeta)

---