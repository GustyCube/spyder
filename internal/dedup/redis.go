package dedup

import (
	"context"
	"time"
	"log"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	cli *redis.Client
	ttl time.Duration
	errorCount int
}

func NewRedis(addr string, ttl time.Duration) (*Redis, error) {
	cli := redis.NewClient(&redis.Options{Addr: addr})
	if err := cli.Ping(context.Background()).Err(); err != nil { return nil, err }
	return &Redis{cli: cli, ttl: ttl}, nil
}

func (r *Redis) Seen(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	ok, err := r.cli.SetNX(ctx, "seen:"+key, 1, r.ttl).Result()
	if err != nil {
		r.errorCount++
		if r.errorCount%100 == 1 { // Log every 100th error to avoid spam
			log.Printf("Redis dedup error (count: %d): %v", r.errorCount, err)
		}
		return false // be permissive on failure
	}
	return !ok
}
