package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	common "server/common"

	xconfig "github.com/75912001/xlib/config"
	xconfigconstants "github.com/75912001/xlib/config/constants"
	xcontrol "github.com/75912001/xlib/control"
	xlog "github.com/75912001/xlib/log"
	xnettcp "github.com/75912001/xlib/net/tcp"
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
		WithNamePrefix("client.simulator").
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
	GetClient().ignoreMsgID = buildIgnoreMsgID(GConfigYaml.IgnoreMsgID)
	GetClient().iEventMgr.Start()
	defer func() {
		if err = stopServiceDiscovery(); err != nil {
			panic(err)
		}
	}()
	if err = startServiceDiscovery(context.Background()); err != nil {
		panic(err)
	}
	gatewayAddr, err := waitGatewayAddr(5 * time.Second)
	if err != nil {
		panic(err)
	}
	GetClient().gatewayAddr = gatewayAddr
	xtimer.GTimer = xtimer.NewTimer()
	if err = xtimer.GTimer.Start(context.Background()); err != nil {
		panic("timer start err")
	}
	defer xtimer.GTimer.Stop()

	GetClient().TCP = xnettcp.NewClient(GetClient())
	opts := xnettcp.NewConnectOptions().
		WithAddress(gatewayAddr).
		WithSendChanCapacity(1000).
		WithHeaderStrategy(&common.DefaultHeaderStrategy{}).
		WithIOut(GetClient().iEventMgr)
	if err = GetClient().TCP.Connect(context.Background(), opts); err != nil {
		ColorPrintf(Red, "connect fail: %v\n", err)
		panic(err)
	}
	GetClient().Remote = GetClient().TCP.IRemote
	GetClient().iEventMgr.Send(&xcontrol.Event{
		ISwitch:   xcontrol.NewSwitchButton(true),
		ICallBack: xcontrol.NewCallBack(func(args ...any) error { return GetClient().OnConnect(GetClient().Remote) }),
	})

	reader := bufio.NewReader(os.Stdin)
	for {
		command, err := reader.ReadString('\n')
		if err != nil {
			ColorPrintf(Red, "Scan fail, err:%v\n", err)
			continue
		}
		command = strings.TrimSpace(command)
		switch command {
		case "":
			continue
		case "quit", "exit":
			GetClient().Close()
			return
		case "list":
			printAPIList()
			continue
		}
		GetClient().iEventMgr.Send(&EventCommand{Command: command})
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
