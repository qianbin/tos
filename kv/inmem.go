package kv

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
)

type inmemkv struct {
	c *cache.Cache
}

func (i *inmemkv) Get(ctx context.Context, key string) ([]byte, error) {
	if v, has := i.c.Get(key); has {
		return v.([]byte), nil
	}
	return nil, nil
}

func (i *inmemkv) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	i.c.Set(key, value, ttl)
	return nil
}
