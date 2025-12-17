// Package model defines core data structures for tasks, handlers, and queue items.
package model

import (
	"context"
)

type HandlerFunc func(context.Context, *Task) error
