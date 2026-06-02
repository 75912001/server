package main

import "fmt"

// 定义颜色代码
const (
	Reset  = "\033[0m"  // 重置
	Red    = "\033[31m" // 红色
	Green  = "\033[32m" // 绿色
	Yellow = "\033[33m" // 黄色
	Blue   = "\033[34m" // 蓝色
	Purple = "\033[35m" // 紫色
	Cyan   = "\033[36m" // 青色
	White  = "\033[37m" // 白色
)

// ColorPrintf 打印带颜色的格式化文本
func ColorPrintf(color string, format string, a ...any) {
	fmt.Printf(color+format+Reset, a...)
}
