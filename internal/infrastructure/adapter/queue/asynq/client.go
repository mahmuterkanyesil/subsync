package asynq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type AsynqTaskQueue struct {
	client *asynq.Client
}

func NewAsynqTaskQueue(redisURL string) *AsynqTaskQueue {
	opt := parseRedisURL(redisURL)
	client := asynq.NewClient(opt)
	return &AsynqTaskQueue{client: client}
}

func (q *AsynqTaskQueue) Enqueue(ctx context.Context, taskName string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("payload marshal failed: %w", err)
	}

	task := asynq.NewTask(taskName, data)
	_, err = q.client.EnqueueContext(ctx, task,
		asynq.MaxRetry(10),
		asynq.Timeout(30*time.Minute),
	)
	return err
}
