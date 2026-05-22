package main

import (
	xpacket "github.com/75912001/xlib/packet"
)

// DefaultHeaderStrategy 实现 IHeaderStrategy 接口，用于处理 xlib 24字节标准包头
type DefaultHeaderStrategy struct{}

func (s *DefaultHeaderStrategy) GetHeaderMode() xpacket.HeaderMode {
	return xpacket.HeaderModeLengthFirst
}

func (s *DefaultHeaderStrategy) GetLengthSize() uint32 {
	return 4
}

func (s *DefaultHeaderStrategy) UnpackLength(buf []byte) uint32 {
	return xpacket.GEndian.Uint32(buf[0:4])
}

func (s *DefaultHeaderStrategy) UnpackMessageID(buf []byte) uint32 {
	return xpacket.GEndian.Uint32(buf[4:8])
}
