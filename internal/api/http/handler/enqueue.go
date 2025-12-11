package handler

import (
	"context"
	"encoding/json"
	"fluxqueue/internal/api/http/response"
	"fluxqueue/internal/model"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type IRedisClient interface {
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) (string, error)
	LPush(ctx context.Context, key string, value string) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
}

type handler struct {
	redis          IRedisClient
	queueReady     string
	queueScheduled string
}

func NewHandler(redis IRedisClient) *handler {
	h := &handler{
		redis:          redis,
		queueReady:     "queue:ready",
		queueScheduled: "queue:scheduled",
	}
	return h
}

func (h *handler) Enqueue(c echo.Context) error {
	req := new(EnqueueRequest)
	if err := c.Bind(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error bind: %w", err))
	}
	if err := c.Validate(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error validate: %w", err))
	}
	now := time.Now().UTC()
	task := &model.Task{
		ID:             uuid.NewString(),
		Type:           req.Type,
		Payload:        req.Payload,
		Attempts:       0,
		MaxRetries:     req.MaxRetries,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error marshalling: %w", err))
	}
	if err := h.redis.LPush(c.Request().Context(), h.queueReady, string(data)); err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error redis push: %w", err))
	}

	return response.WithStatusOKResponse(
		map[string]any{
			"task_id": task.ID,
		}, c)
}

func (h *handler) Schedule(c echo.Context) error {
	req := new(ScheduleRequest)
	if err := c.Bind(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error bind: %w", err))
	}
	if err := c.Validate(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error validate: %w", err))
	}

	now := time.Now().UTC()
	task := &model.Task{
		ID:             uuid.NewString(),
		Type:           req.Type,
		Payload:        req.Payload,
		Attempts:       0,
		MaxRetries:     req.MaxRetries,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
		RunAt:          &req.RunAt,
	}

	data, err := json.Marshal(task)
	if err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error marshalling: %w", err))
	}
	if err := h.redis.LPush(c.Request().Context(), h.queueScheduled, string(data)); err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error redis push: %w", err))
	}

	return response.WithStatusOKResponse(
		map[string]any{
			"task_id": task.ID,
		}, c)
}
