package queue

import (
	"context"
	"time"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	cli *redis.Client
	queueKey string
	procKey string
	leaseTTL time.Duration
}

type item struct {
	Host string `json:"host"`
	TS   int64  `json:"ts"`
	Attempt int `json:"attempt"`
}

func NewRedis(addr, key string, lease time.Duration) (*RedisQueue, error) {
	cli := redis.NewClient(&redis.Options{Addr: addr})
	if err := cli.Ping(context.Background()).Err(); err != nil { return nil, err }
	return &RedisQueue{cli: cli, queueKey: key, procKey: key+":processing", leaseTTL: lease}, nil
}

func (q *RedisQueue) Lease(ctx context.Context) (string, func() error, error) {
	res, err := q.cli.BRPopLPush(ctx, q.queueKey, q.procKey, 5*time.Second).Result()
	if err == redis.Nil { return "", func() error { return nil }, nil }
	if err != nil { return "", func() error { return err }, err }
	var it item
	if err := json.Unmarshal([]byte(res), &it); err != nil { return "", func() error { return err }, err }
	ack := func() error {
		return q.cli.LRem(ctx, q.procKey, 1, res).Err()
	}
	return it.Host, ack, nil
}

// Seed pushes a host into the queue
func (q *RedisQueue) Seed(ctx context.Context, host string) error {
	b, _ := json.Marshal(item{Host: host, TS: time.Now().UTC().Unix(), Attempt: 0})
	return q.cli.LPush(ctx, q.queueKey, string(b)).Err()
}
