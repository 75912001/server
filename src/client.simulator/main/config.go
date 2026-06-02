package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var GConfigYaml *ConfigYaml

type ConfigYaml struct {
	Etcd              EtcdConfig    `yaml:"etcd"`
	ProtoPath         string        `yaml:"protoPath"`
	IgnoreMsgID       []uint32      `yaml:"ignoreMsgID"`
	HeartbeatInterval time.Duration `yaml:"heartbeatInterval"`
	CacheTokenExpire  uint64        `yaml:"cacheTokenExpireSecond"`
}

type EtcdConfig struct {
	Endpoints   []string      `yaml:"endpoints"`
	TTLDuration time.Duration `yaml:"ttlDuration"`
	ProjectName string        `yaml:"projectName"`
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
	if GConfigYaml.CacheTokenExpire == 0 {
		GConfigYaml.CacheTokenExpire = 10
	}
	if GConfigYaml.Etcd.TTLDuration <= 0 {
		GConfigYaml.Etcd.TTLDuration = 30 * time.Second
	}
	if GConfigYaml.Etcd.ProjectName == "" {
		GConfigYaml.Etcd.ProjectName = "project"
	}
	if GConfigYaml.ProtoPath != "" && !filepath.IsAbs(GConfigYaml.ProtoPath) {
		GConfigYaml.ProtoPath = filepath.Join(filepath.Dir(path), GConfigYaml.ProtoPath)
	}
	return nil
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
