package main

import (
	"context"
	"os"
)

func main() {
	app := NewGateway(os.Args)
	if app == nil {
		return
	}

	ctx := context.Background()

	if err := app.PreStart(ctx); err != nil {
		panic(err)
	}
	if err := app.Start(ctx); err != nil {
		panic(err)
	}
	_ = app.PostStart()
}
