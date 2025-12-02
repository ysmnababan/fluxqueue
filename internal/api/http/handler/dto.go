package handler

import (
	"fluxqueue/internal/model"
)

type EnqueueRequest struct {
	Type           string            `json:"type" validate:"required"`    // handler name
	Payload        model.TaskPayload `json:"payload" validate:"required"` // per-type data
	MaxRetries     int               `json:"max_retries"`                 // allowed retries
	IdempotencyKey string            `json:"idempotency_key,omitempty"`   // dedupe key
}
