package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	xconfig "github.com/75912001/xlib/config"
	xconfigconstants "github.com/75912001/xlib/config/constants"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
	xpacket "github.com/75912001/xlib/packet"
	xruntime "github.com/75912001/xlib/runtime"
	xruntimeconstants "github.com/75912001/xlib/runtime/constants"
	xtimer "github.com/75912001/xlib/timer"
)

func main() {
	var err error
	xruntime.SetRunMode(xruntimeconstants.RunModeDebug)
	xpacket.SetEndianMode(xpacket.LittleEndian)
	initXConfigForClient()

	log, err = xlog.NewMgr(xlog.NewOptions().
		WithAbsPath(filepath.Join(xruntime.ExecutablePath, "log")).
		WithNamePrefix("robot").
		WithLevel(xlog.LevelOn).
		WithLevelCallBack(xcontrol.NewCallBack(logCallBackFunc), xlog.LevelFatal, xlog.LevelError, xlog.LevelWarn))
	if err != nil {
		panic(err)
	}
	xlog.GLog = log
	defer func() {
		if err = log.Stop(); err != nil {
			panic(err)
		}
	}()

	executablePath, err := xruntime.GetExecutablePath()
	if err != nil {
		panic(err)
	}
	apiYamlPath = filepath.Join(executablePath, "api.yaml")
	if err = parseConfigYaml(filepath.Join(executablePath, "config.yaml")); err != nil {
		panic(err)
	}
	appCtx, cancel := context.WithCancel(context.Background())
	GRobotManager = NewRobotManager()
	GRobotManager.StartEventLoop()
	xtimer.GTimer = xtimer.NewTimer()
	if err = xtimer.GTimer.Start(appCtx); err != nil {
		panic("timer start err")
	}
	panel, err := StartControlPanel(appCtx, GRobotManager)
	if err != nil {
		ColorPrintf(Red, "start control panel failed: %v\n", err)
		log.Errorf("start control panel failed: %v", err)
	}
	defer func() {
		cancel()
		if panel != nil {
			panel.Stop()
		}
		GRobotManager.Stop()
		if err = stopServiceDiscovery(); err != nil {
			panic(err)
		}
		xtimer.GTimer.Stop()
	}()
	if err = startServiceDiscovery(appCtx); err != nil {
		panic(err)
	}
	if err = GRobotManager.Start(appCtx); err != nil {
		panic(err)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		command, err := reader.ReadString('\n')
		if err != nil {
			ColorPrintf(Red, "Scan fail, err:%v\n", err)
			continue
		}
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		if GRobotManager.ExecuteCommand(command) {
			return
		}
	}
}

func initXConfigForClient() {
	processingMode := xconfigconstants.ProcessingModeBus
	packetLengthMax := uint32(65535)
	xconfig.GConfigMgr.Base.ProcessingMode = &processingMode
	xconfig.GConfigMgr.Base.PacketLengthMax = &packetLengthMax
}

func printAPIList() {
	data, err := loadAPI(apiYamlPath)
	if err != nil {
		ColorPrintf(Red, "load api failed: %v\n", err)
		return
	}
	for name, api := range data {
		fmt.Printf("%s%s%s  %s\n", Cyan, name, Reset, api.ID)
	}
}
