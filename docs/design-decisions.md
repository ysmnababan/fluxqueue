# 📚 **Architectural Rationale (Why This Design?)**

This section explains *why* each design decision was chosen for this background job processing system.

---

## 1. Why Redis LIST?

Redis List is:

* O(1) push/pop
* Reliable enough (AOF)
* Battle-tested in major queues (Sidekiq, BullMQ)
* Supports blocking pop (`BRPOP`) → efficient worker wake-up

Using it as the **ready queue** provides:

* predictable FIFO semantics
* simple consumer workflows
* low operational complexity

---

## 2. Why Redis ZSET?

A scheduler requires:

* ordering by timestamp
* ability to fetch “all tasks due now” efficiently

ZSET gives us:

* Sorted ordering by score
* Efficient range queries:

  ```
  ZRANGEBYSCORE key -inf now
  ```
* O(log N) complexity, perfect for scheduled jobs

ZSET enables:

* delayed jobs
* scheduled jobs
* recurring jobs (optional)

---

## 3. Why Go?

Go is the perfect fit for distributed job workers because:

* Goroutines are lightweight → scaling thousands of tasks
* Channels enable safe concurrency
* Strong ecosystem for Redis, pprof, testing
* Easy deployment (single static binary)


---

## 4. Why a Worker Pool?

Worker pool solves:

* Memory safety (limit goroutines)
* Resource management (DB/HTTP connections)
* Backpressure
* Fair task processing
* Better throughput measurement

---

## 5. Why a Separate Scheduler Process?

Separation of concerns:

* API → receives tasks
* Scheduler → makes tasks runnable
* Worker → processes them

This provides:

* cleaner architecture
* easier scaling (scale workers independently)
* ability to deploy scheduler as a Cron-like service
* better observability

---

## 6. Why Redis instead of Kafka?

Kafka is amazing but heavy:

* requires brokers + partitions + ZK/RAFT
* high ops complexity

Redis is:

* lightweight
* fast
* trivial to run locally
* perfect for a portfolio project

If needed, the design can later be upgraded to Kafka (Streams) without major refactor.

---

## 7. Why Task Registry Pattern?

Using a registry allows:

```
Register("email.send", EmailSendHandler)
Register("image.resize", ImageResizeHandler)
```

Benefits:

* Adding new task types is easy
* Code stays modular
* Good interview talking point (plugin architecture)

---

## 8. Why DLQ + exponential backoff?

This is industry standard reliability strategy:

* retry transient failures
* fail fast on permanent errors
* avoid retry storms
* prevent worker overload

Shows understanding of distributed reliability patterns.

---

## 9. Why Prometheus Metrics?

Metrics answer:

* How many tasks processed?
* How many failed?
* How long do tasks spend in queue?
* How long do workers take?
* What’s the throughput?

---
