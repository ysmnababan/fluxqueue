package model

import "time"

type TaskPayload map[string]any // or []byte if protobuf

type Task struct {
	ID             string            `json:"id"`                        // uuid
	Type           string            `json:"type"`                      // handler name
	Payload        TaskPayload       `json:"payload"`                   // per-type data
	Attempts       int               `json:"attempts"`                  // current attempt count
	MaxRetries     int               `json:"max_retries"`               // allowed retries
	IdempotencyKey string            `json:"idempotency_key,omitempty"` // dedupe key
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"` // processing timeout seconds
	RunAt          *time.Time        `json:"run_at,omitempty"`          // scheduled time, nil = now
	CreatedAt      time.Time         `json:"created_at"`
	Meta           map[string]string `json:"meta,omitempty"`
	Priority       int               `json:"priority,omitempty"`
	Version        string            `json:"version,omitempty"`

	FailedAt  *time.Time `json:"failed_at"`
	LastError string     `json:"last_error"`
}
