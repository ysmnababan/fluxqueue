package model

import (
	"context"
)

type HandlerFunc func(context.Context, *Task) error
