// Package service implements task handlers for email sending, report generation, and other background operations.
package service

import (
	"context"
	"errors"
	"fluxqueue/internal/model"
	"fmt"
	"math/rand"
	"slices"
	"time"
)

type Service struct {
}

func NewTaskService() *Service {
	return &Service{}
}

func (s *Service) SendEmail(ctx context.Context, t *model.Task) error {
	// simulate sending an email
	_ = t
	n := rand.Intn(3500) + 200 // in ms
	fmt.Printf("wait for %d sec\n", n)
	time.Sleep(time.Duration(n) * time.Millisecond)
	return simulateError(10)
}

func (s Service) GenerateExcelReport(ctx context.Context, t *model.Task) error {
	_ = t
	n := rand.Intn(2000) + 100 // in ms
	time.Sleep(time.Duration(n) * time.Millisecond)
	return simulateError(2)
}

// helper function to generate error based on percentage
func simulateError(errorPercent int) error {
	prob := []int{}
	for range 100 {
		val := rand.Intn(100)
		if slices.Contains(prob, val) {
			continue
		}
		prob = append(prob, val)
	}

	if slices.Contains(prob, errorPercent) {
		return errors.New("some error")
	}
	return nil
}
