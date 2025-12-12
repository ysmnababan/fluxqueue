package service

import (
	"context"
	"errors"
	"fluxqueue/internal/model"
	"time"
)

type Service struct {
}

func NewTaskService() *Service {
	return &Service{}
}

func (s *Service) SendEmail(ctx context.Context, t *model.Task) error {
	// simulate sending an email
	now := time.Now().UnixMilli()
	_ = t
	time.Sleep(200 * time.Millisecond)
	if now%2 == 0 {
		return errors.New("some error happen")
	}
	return nil
}

func (s Service) GenerateExcelReport(ctx context.Context, t *model.Task) error {
	_ = t
	time.Sleep(300 * time.Millisecond)
	return nil
}
