package main

import (
	xcontrol "github.com/75912001/xlib/control"
	xnetcommon "github.com/75912001/xlib/net/common"
)

func Bus(args ...any) error {
	value := args[0]
	var err error
	switch event := value.(type) {
	case *xnetcommon.Connect:
		err = event.IHandler.OnConnect(event.IRemote)
	case *xnetcommon.Packet:
		err = event.IHandler.OnPacket(event.IRemote, event.IPacket)
	case *xnetcommon.Disconnect:
		err = event.IHandler.OnDisconnect(event.IRemote)
	case *xcontrol.Event:
		if event.ISwitch.IsOff() {
			return nil
		}
		err = event.ICallBack.Execute()
	case *RobotCommand:
		err = event.Robot.SendCommand(event)
	default:
	}
	return err
}
