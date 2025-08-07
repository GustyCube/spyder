package dedup

import (
	"context"
	"time"
	"github.com/go-redis/redis/v9"
)

type Redis struct {
	cli *redis.Client
	ttl time.Duration
}

func NewRedis(addr string, ttl time.Duration) (*Redis, error) {
	cli := redis.NewClient(&redis.Options{Addr: addr})
	if err := cli.Ping(context.Background()).Err(); err != nil { return nil, err }
	return &Redis{cli: cli, ttl: ttl}, nil
}

func (r *Redis) Seen(key string) bool {
	ctx := context.Background()
	ok, err := r.cli.SetNX(ctx, "seen:"+key, 1, r.ttl).Result()
	if err != nil { return false } // be permissive on failure
	return !ok
}
