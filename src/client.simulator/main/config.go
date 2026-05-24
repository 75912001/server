package main

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Addr              string        `yaml:"addr"`              // Gateway TCP 地址。
	APIPath           string        `yaml:"apiPath"`           // api.yaml 路径；支持绝对路径，也支持相对 config.yaml 所在目录。
	ProtoPath         string        `yaml:"protoPath"`         // proto 目录路径；用于记录协议来源，支持绝对路径和相对 config.yaml 所在目录。
	IgnoreMsgID       []uint32      `yaml:"ignoreMsgID"`       // 收包打印时忽略的消息号列表。
	ClientCount       int           `yaml:"clientCount"`       // 并发模拟的客户端数量。
	UIDStart          uint64        `yaml:"uidStart"`          // 起始 uid，第 N 个客户端 uid = UIDStart + N。
	TokenPrefix       string        `yaml:"tokenPrefix"`       // 默认 token 前缀，最终 token = TokenPrefix + uid。
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval"` // 默认心跳间隔。
}

type APIData struct {
	ID  string         `yaml:"id"`  // 消息号，支持 0x 前缀。
	Msg map[string]any `yaml:"msg"` // Protobuf 消息字段，按 protojson 字段名填写。
}

func loadConfig(path string) (Config, error) {
	cfg := Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if !filepath.IsAbs(cfg.APIPath) {
		cfg.APIPath = filepath.Join(filepath.Dir(path), cfg.APIPath)
	}
	if cfg.ProtoPath != "" && !filepath.IsAbs(cfg.ProtoPath) {
		cfg.ProtoPath = filepath.Join(filepath.Dir(path), cfg.ProtoPath)
	}
	return cfg, nil
}

// loadAPI 读取 api.yaml，返回命令名到消息定义的映射。
func loadAPI(path string) (map[string]APIData, error) {
	data := map[string]APIData{}
	file, err := os.Open(path)
	if err != nil {
		return data, err
	}
	defer file.Close()
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return data, err
	}
	return data, nil
}
