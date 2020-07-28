package kv

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type rediskv struct {
	client *redis.Client
}

func (r *rediskv) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	return []byte(v), nil
}

func (r *rediskv) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}
