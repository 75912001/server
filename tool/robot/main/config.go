package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var GConfigYaml *ConfigYaml

type ConfigYaml struct {
	Etcd                          EtcdConfig         `yaml:"etcd"`
	Login                         LoginConfig        `yaml:"login"`
	ProtoPath                     string             `yaml:"protoPath"`
	IgnoreMsgID                   []uint32           `yaml:"ignoreMsgID"`
	HeartbeatInterval             time.Duration      `yaml:"heartbeatInterval"`
	CacheAccountVerifyTokenExpire uint64             `yaml:"cacheAccountVerifyTokenExpireSecond"`
	Robot                         RobotConfig        `yaml:"robot"`
	ControlPanel                  ControlPanelConfig `yaml:"controlPanel"`
}

type EtcdConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	TTLDuration time.Duration `yaml:"ttlDuration"`
	ProjectName string        `yaml:"projectName"`
}

type LoginConfig struct {
	Addr                   string        `yaml:"addr"`
	AccountVerifyTokenPath string        `yaml:"accountVerifyTokenPath"`
	SessionPath            string        `yaml:"sessionPath"`
	Timeout                time.Duration `yaml:"timeout"`
}

type ApiData struct {
	ID  string         `yaml:"id"`
	Msg map[string]any `yaml:"msg"`
}

func parseConfigYaml(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("read config yaml failed: %v\n", err)
		return err
	}
	GConfigYaml = &ConfigYaml{}
	if err := yaml.Unmarshal(data, GConfigYaml); err != nil {
		fmt.Printf("parse config yaml failed: %v\n", err)
		return err
	}
	if GConfigYaml.HeartbeatInterval <= 0 {
		GConfigYaml.HeartbeatInterval = 10 * time.Second
	}
	if GConfigYaml.CacheAccountVerifyTokenExpire == 0 {
		GConfigYaml.CacheAccountVerifyTokenExpire = 10
	}
	if GConfigYaml.Etcd.TTLDuration <= 0 {
		GConfigYaml.Etcd.TTLDuration = 30 * time.Second
	}
	if GConfigYaml.Etcd.ProjectName == "" {
		GConfigYaml.Etcd.ProjectName = "project"
	}
	normalizeLoginConfig(GConfigYaml)
	normalizeRobotConfig(GConfigYaml)
	normalizeControlPanelConfig(GConfigYaml)
	if GConfigYaml.ProtoPath != "" && !filepath.IsAbs(GConfigYaml.ProtoPath) {
		GConfigYaml.ProtoPath = filepath.Join(filepath.Dir(path), GConfigYaml.ProtoPath)
	}
	return nil
}

func normalizeLoginConfig(cfg *ConfigYaml) {
	if cfg.Login.Addr == "" {
		cfg.Login.Addr = "http://127.0.0.1:30401"
	}
	if !strings.HasPrefix(cfg.Login.Addr, "http://") && !strings.HasPrefix(cfg.Login.Addr, "https://") {
		cfg.Login.Addr = "http://" + cfg.Login.Addr
	}
	cfg.Login.Addr = strings.TrimRight(cfg.Login.Addr, "/")
	if cfg.Login.AccountVerifyTokenPath == "" {
		cfg.Login.AccountVerifyTokenPath = "/api/login/accountVerifyToken"
	}
	if !strings.HasPrefix(cfg.Login.AccountVerifyTokenPath, "/") {
		cfg.Login.AccountVerifyTokenPath = "/" + cfg.Login.AccountVerifyTokenPath
	}
	if cfg.Login.SessionPath == "" {
		cfg.Login.SessionPath = "/api/login/session"
	}
	if !strings.HasPrefix(cfg.Login.SessionPath, "/") {
		cfg.Login.SessionPath = "/" + cfg.Login.SessionPath
	}
	if cfg.Login.Timeout <= 0 {
		cfg.Login.Timeout = 5 * time.Second
	}
}

func loadAPI(path string) (map[string]ApiData, error) {
	data := map[string]ApiData{}
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

func buildIgnoreMsgID(ids []uint32) map[uint32]struct{} {
	m := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}
