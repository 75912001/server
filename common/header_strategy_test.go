package common

import (
	"testing"

	xpacket "github.com/75912001/xlib/packet"
)

func TestDefaultHeaderStrategy(t *testing.T) {
	strategy := &DefaultHeaderStrategy{}
	if got := strategy.GetHeaderMode(); got != xpacket.HeaderModeLengthFirst {
		t.Fatalf("GetHeaderMode = %v, want %v", got, xpacket.HeaderModeLengthFirst)
	}
	if got := strategy.GetLengthSize(); got != 4 {
		t.Fatalf("GetLengthSize = %d, want 4", got)
	}

	buf := make([]byte, 8)
	xpacket.GEndian.PutUint32(buf[0:4], xpacket.HeaderLengthFieldSize+16)
	xpacket.GEndian.PutUint32(buf[4:8], 10001)
	if got := strategy.UnpackLength(buf); got != 16 {
		t.Fatalf("UnpackLength = %d, want 16", got)
	}
	if got := strategy.UnpackMessageID(buf); got != 10001 {
		t.Fatalf("UnpackMessageID = %d, want 10001", got)
	}

	xpacket.GEndian.PutUint32(buf[0:4], xpacket.HeaderLengthFieldSize-1)
	if got := strategy.UnpackLength(buf); got != 0 {
		t.Fatalf("short UnpackLength = %d, want 0", got)
	}
}
