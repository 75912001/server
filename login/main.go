package main

import (
	"context"
	"os"
)

// main 按 xlib 生命周期启动 login 服务。
func main() {
	srv := NewLoginServer(os.Args)
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
