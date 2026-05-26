package main

import (
	"context"
	"os"
)

func main() {
	srv := NewCacheServer(os.Args)
	if srv == nil {
		return
	}

	ctx := context.Background()

	if err := srv.PreStart(ctx); err != nil {
		panic(err)
	}
	if err := srv.Start(ctx); err != nil {
		panic(err)
	}
	if err := srv.PostStart(); err != nil {
		panic(err)
	}
}
