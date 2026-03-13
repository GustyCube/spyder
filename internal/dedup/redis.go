package dedup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Redis struct {
	cli        *redis.Client
	ttl        time.Duration
	log        *zap.SugaredLogger
	errorCount int
}

func NewRedis(addr string, ttl time.Duration, log *zap.SugaredLogger) (*Redis, error) {
	cli := redis.NewClient(&redis.Options{Addr: addr})
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Redis{cli: cli, ttl: ttl, log: log}, nil
}

func (r *Redis) Seen(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ok, err := r.cli.SetNX(ctx, "seen:"+key, 1, r.ttl).Result()
	if err != nil {
		r.errorCount++
		if r.errorCount%100 == 1 {
			r.log.Warnw("redis dedup error", "count", r.errorCount, "err", err)
		}
		return false
	}
	return !ok
}
