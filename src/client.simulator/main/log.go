package main

import (
	"fmt"

	xlog "github.com/75912001/xlib/log"
	xruntime "github.com/75912001/xlib/runtime"
)

var log xlog.ILog

func logCallBackFunc(args ...any) error {
	level := args[0].(uint32)
	outString := args[1].(string)
	if xruntime.IsDebug() {
		fmt.Println(level, outString)
	}
	return nil
}
