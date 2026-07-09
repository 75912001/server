package main

import (
	"os"
	"path/filepath"

	xconfig "github.com/75912001/xlib/config"
)

var GCfgCustomGameConfigDir string

// initCustomConfig 从 xlib 配置管理器读取 online 自定义配置.
func initCustomConfig() {
	GCfgCustomGameConfigDir = resolveGameConfigDir(xconfig.GConfigMgr.GetCustomString("gameConfigDir", "config"))
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
