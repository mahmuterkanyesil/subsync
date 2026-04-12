package port

import "context"

type TaskQueue interface {
	Enqueue(ctx context.Context, taskName string, payload any) error
}
