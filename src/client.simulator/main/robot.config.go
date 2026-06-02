package main

import "time"

type RobotConfig struct {
	Count                int                  `yaml:"count"`
	UIDStart             uint64               `yaml:"uidStart"`
	UIDStep              uint64               `yaml:"uidStep"`
	StartupBatchSize     int                  `yaml:"startupBatchSize"`
	StartupBatchInterval time.Duration        `yaml:"startupBatchInterval"`
	HeartbeatInterval    time.Duration        `yaml:"heartbeatInterval"`
	ActionInterval       time.Duration        `yaml:"actionInterval"`
	ActionJitter         time.Duration        `yaml:"actionJitter"`
	SendChanCapacity     int                  `yaml:"sendChanCapacity"`
	Messages             []RobotMessageConfig `yaml:"messages"`
	Logging              RobotLoggingConfig   `yaml:"logging"`
}

type RobotMessageConfig struct {
	Name   string `yaml:"name"`
	Weight int    `yaml:"weight"`
}

type RobotLoggingConfig struct {
	SummaryInterval time.Duration `yaml:"summaryInterval"`
	DetailFailures  bool          `yaml:"detailFailures"`
}

func normalizeRobotConfig(cfg *ConfigYaml) {
	robot := &cfg.Robot
	if robot.Count <= 0 {
		robot.Count = 1
	}
	if robot.UIDStart == 0 {
		robot.UIDStart = 10001
	}
	if robot.UIDStep == 0 {
		robot.UIDStep = 1
	}
	if robot.StartupBatchSize <= 0 {
		robot.StartupBatchSize = 100
	}
	if robot.StartupBatchInterval <= 0 {
		robot.StartupBatchInterval = 100 * time.Millisecond
	}
	if robot.HeartbeatInterval <= 0 {
		robot.HeartbeatInterval = cfg.HeartbeatInterval
	}
	if robot.HeartbeatInterval <= 0 {
		robot.HeartbeatInterval = 10 * time.Second
	}
	if robot.SendChanCapacity <= 0 {
		robot.SendChanCapacity = 1000
	}
	if robot.Logging.SummaryInterval <= 0 {
		robot.Logging.SummaryInterval = 5 * time.Second
	}
}
