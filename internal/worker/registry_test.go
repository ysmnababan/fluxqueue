package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"fluxqueue/internal/model"
)

func TestHandlerRegistry_ConcurrentAccess(t *testing.T) {
	hr := NewRegistry()

	wg := &sync.WaitGroup{}
	for i := range 100 {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			hr.Register(fmt.Sprintf("task-%d", idx),
				func(ctx context.Context, t *model.Task) error { return nil })
		}()
	}

	for i := range 100 {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			hr.Get(fmt.Sprintf("task-%d", idx))
		}()
	}
	wg.Wait()
}
