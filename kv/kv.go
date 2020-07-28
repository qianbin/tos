package kv

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
)

type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

func New(ctx context.Context, url string) (KV, error) {
	if url == "" {
		return &inmemkv{cache.New(5*time.Minute, 10*time.Minute)}, nil
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, err
	}

	return &rediskv{client}, nil
}
