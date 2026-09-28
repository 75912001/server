package main

import (
	"os"
	"path/filepath"

	xconfig "github.com/75912001/xlib/config"
)

const (
	// defaultAccountRecordFlushIntervalSecond 是账号档案延迟落盘的默认窗口秒数.
	defaultAccountRecordFlushIntervalSecond = 10
	// defaultAccountRecordFlushWriteCount 是账号档案延迟落盘的默认写次数阈值.
	defaultAccountRecordFlushWriteCount = 10
)

var (
	GCfgCustomGameConfigDir string
	// GCfgCustomAccountRecordFlushIntervalSecond 是账号档案延迟落盘的窗口秒数, 从首次变脏起算且不重置.
	GCfgCustomAccountRecordFlushIntervalSecond int
	// GCfgCustomAccountRecordFlushWriteCount 是窗口内累计写次数阈值, 达到即立即落盘; 0 表示关闭次数触发.
	GCfgCustomAccountRecordFlushWriteCount int
)

// initCustomConfig 从 xlib 配置管理器读取 online 自定义配置.
func initCustomConfig() {
	GCfgCustomGameConfigDir = resolveGameConfigDir(xconfig.GConfigMgr.GetCustomString("gameConfigDir", "config"))
	GCfgCustomAccountRecordFlushIntervalSecond = normalizeAccountRecordFlushInterval(
		xconfig.GConfigMgr.GetCustomInt("accountRecordFlushIntervalSecond", defaultAccountRecordFlushIntervalSecond),
	)
	GCfgCustomAccountRecordFlushWriteCount = normalizeAccountRecordFlushWriteCount(
		xconfig.GConfigMgr.GetCustomInt("accountRecordFlushWriteCount", defaultAccountRecordFlushWriteCount),
	)
}

// normalizeAccountRecordFlushInterval 约束落盘窗口为正数, 非法值回退默认值.
// 窗口不允许关闭: 关闭会让在线玩家永不落盘.
func normalizeAccountRecordFlushInterval(second int) int {
	if second <= 0 {
		return defaultAccountRecordFlushIntervalSecond
	}
	return second
}

// normalizeAccountRecordFlushWriteCount 约束写次数阈值; 负值回退默认值, 0 表示关闭次数触发.
func normalizeAccountRecordFlushWriteCount(count int) int {
	if count < 0 {
		return defaultAccountRecordFlushWriteCount
	}
	return count
}

func resolveGameConfigDir(dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	candidates := []string{dir}
	if xconfig.GConfigMgr.ExecutablePath != "" {
		configDir := filepath.Dir(xconfig.GConfigMgr.ExecutablePath)
		candidates = append(candidates, filepath.Join(configDir, dir))
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return filepath.Clean(candidate)
		}
	}
	return filepath.Clean(dir)
}
