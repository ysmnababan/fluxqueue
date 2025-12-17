// Package worker implements a worker pool for processing tasks concurrently with retries and backoff.
package worker

import (
	"fluxqueue/internal/model"
	"sync"
)

type HandlerRegistry struct {
	// use RWMutex instead of Mutex, because it won't block read if not writing.
	// Also for case when read is more frequent than writing
	mu sync.RWMutex

	handlers map[string]model.HandlerFunc // map is not concurrent safe, so add mutex
}

func NewRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		mu:       sync.RWMutex{},
		handlers: make(map[string]model.HandlerFunc),
	}
}

func (r *HandlerRegistry) Register(taskType string, fun model.HandlerFunc) {
	r.mu.Lock() // exclusive writes, only one goroutine can modify at a time
	r.handlers[taskType] = fun
	r.mu.Unlock()
}

func (r *HandlerRegistry) Get(taskType string) (model.HandlerFunc, bool) {
	r.mu.RLock() // concurrent reads, multiple goroutines can read simultaneously
	fun, ok := r.handlers[taskType]
	r.mu.RUnlock()
	return fun, ok
}
