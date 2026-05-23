package main

import (
	xpacket "github.com/75912001/xlib/packet"
)

// DefaultHeaderStrategy 实现 IHeaderStrategy 接口，用于处理 xlib 24字节标准包头
type DefaultHeaderStrategy struct{}

func (p *DefaultHeaderStrategy) GetHeaderMode() xpacket.HeaderMode {
	return xpacket.HeaderModeLengthFirst
}

func (p *DefaultHeaderStrategy) GetLengthSize() uint32 {
	return 4
}

func (p *DefaultHeaderStrategy) UnpackLength(buf []byte) uint32 {
	return xpacket.GEndian.Uint32(buf[0:4])
}

func (p *DefaultHeaderStrategy) UnpackMessageID(buf []byte) uint32 {
	return xpacket.GEndian.Uint32(buf[4:8])
}
