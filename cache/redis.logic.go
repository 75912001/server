package main

import (
	"context"
	"time"
)

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func redisScriptResultIsOK(result any) bool {
	switch v := result.(type) {
	case int64:
		return v == 1
	case int:
		return v == 1
	case string:
		return v == "1"
	default:
		return false
	}
}
