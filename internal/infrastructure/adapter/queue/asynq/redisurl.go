package asynq

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
)

// parseRedisURL parses redis URL like redis://host:port/db into asynq.RedisClientOpt
func parseRedisURL(redisURL string) asynq.RedisClientOpt {
    if strings.HasPrefix(redisURL, "redis://") || strings.HasPrefix(redisURL, "rediss://") {
        if u, err := url.Parse(redisURL); err == nil {
            opt := asynq.RedisClientOpt{Addr: u.Host}
            if u.User != nil {
                if p, ok := u.User.Password(); ok {
                    opt.Password = p
                }
            }
            if u.Path != "" {
                dbStr := strings.TrimPrefix(u.Path, "/")
                if dbStr != "" {
                    if db, err := strconv.Atoi(dbStr); err == nil {
                        opt.DB = db
                    }
                }
            }
            return opt
        }
    }
    return asynq.RedisClientOpt{Addr: redisURL}
}
