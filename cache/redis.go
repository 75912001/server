package main

import (
	"context"

	xconfig "github.com/75912001/xlib/config"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

var GRedis *Redis

type Redis struct {
	client *redis.ClusterClient
}

func newRedis(configs []*xconfig.Redis) (*Redis, error) {
	for _, redisCfg := range configs {
		if len(redisCfg.Addrs) == 0 {
			return nil, errors.Errorf("redis addrs is empty %v", xruntime.Location())
		}
		client := redis.NewClusterClient(
			&redis.ClusterOptions{
				Addrs:        append([]string(nil), redisCfg.Addrs...),
				Password:     *redisCfg.Password,
				DialTimeout:  *redisCfg.DialTimeoutDuration,
				ReadTimeout:  *redisCfg.ReadTimeoutDuration,
				WriteTimeout: *redisCfg.WriteTimeoutDuration,
			},
		)
		return &Redis{client: client}, nil
	}
	return nil, errors.Errorf("redis config not found %v", xruntime.Location())
}

func (p *Redis) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}

func (p *Redis) Close() error {
	return p.client.Close()
}

func (p *Redis) Get(ctx context.Context, key string) (string, error) {
	return p.client.Get(ctx, key).Result()
}
