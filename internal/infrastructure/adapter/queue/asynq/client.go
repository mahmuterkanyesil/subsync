package asynq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
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

	h := fnv.New64a()
	h.Write([]byte(taskName))
	h.Write(data)
	taskID := fmt.Sprintf("%016x", h.Sum64())

	task := asynq.NewTask(taskName, data)
	_, err = q.client.EnqueueContext(ctx, task,
		asynq.TaskID(taskID),
		asynq.MaxRetry(10),
		asynq.Timeout(30*time.Minute),
	)
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}
