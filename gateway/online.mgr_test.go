package main

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xmap "github.com/75912001/xlib/map"
	"google.golang.org/grpc"
)

var (
	testOnlineMgrLogInit    sync.Once
	testOnlineMgrLogInitErr error
)

func TestOnlineMgrReserveByAvailableLoadDistributesEqualLoad(t *testing.T) {
	mgr := newTestOnlineMgr()
	key1 := "/project/server/1/online/1/"
	key2 := "/project/server/1/online/2/"
	mgr.m.Add(key1, newTestOnline(t, key1, 2))
	mgr.m.Add(key2, newTestOnline(t, key2, 2))

	counts := reserveOnlineKeys(t, mgr, 4)
	if counts[key1] != 2 || counts[key2] != 2 {
		t.Fatalf("reserve counts = %v, want %s=2 and %s=2", counts, key1, key2)
	}
	if online, err := mgr.ReserveByAvailableLoad(); err == nil {
		t.Fatalf("reserve after exhausted returned %s", online.Key)
	}
}

func TestOnlineMgrReserveByAvailableLoadConsumesDifferentLoad(t *testing.T) {
	mgr := newTestOnlineMgr()
	key1 := "/project/server/1/online/1/"
	key2 := "/project/server/1/online/2/"
	mgr.m.Add(key1, newTestOnline(t, key1, 3))
	mgr.m.Add(key2, newTestOnline(t, key2, 1))

	counts := reserveOnlineKeys(t, mgr, 4)
	if counts[key1] != 3 || counts[key2] != 1 {
		t.Fatalf("reserve counts = %v, want %s=3 and %s=1", counts, key1, key2)
	}
}

func TestOnlineMgrUpdateAvailableLoadOverridesReservedLoad(t *testing.T) {
	ensureOnlineMgrTestLog(t)

	mgr := newTestOnlineMgr()
	key := "/project/server/1/online/1/"
	online := newTestOnline(t, key, 2)
	mgr.m.Add(key, online)

	if _, err := mgr.ReserveByAvailableLoad(); err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if got, want := atomic.LoadUint32(&online.AvailableLoad), uint32(1); got != want {
		t.Fatalf("available load after reserve = %d, want %d", got, want)
	}

	mgr.UpdateAvailableLoad(key, &xetcd.ValueJson{AvailableLoad: 5})
	if got, want := atomic.LoadUint32(&online.AvailableLoad), uint32(5); got != want {
		t.Fatalf("available load after update = %d, want %d", got, want)
	}
}

func ensureOnlineMgrTestLog(t *testing.T) {
	t.Helper()

	testOnlineMgrLogInit.Do(func() {
		if xlog.GLog != nil {
			return
		}
		xlog.GLog, testOnlineMgrLogInitErr = xlog.NewMgr(xlog.NewOptions().
			WithIsWriteFile(false).
			WithIsReportCaller(false),
		)
	})
	if testOnlineMgrLogInitErr != nil {
		t.Fatalf("init test log: %v", testOnlineMgrLogInitErr)
	}
}

func TestOnlineMgrReserveByAvailableLoadSkipsZeroAndUnavailable(t *testing.T) {
	mgr := newTestOnlineMgr()
	disabledKey := "/project/server/1/online/1/"
	zeroKey := "/project/server/1/online/2/"
	availableKey := "/project/server/1/online/3/"
	disabled := newTestOnline(t, disabledKey, 5)
	disabled.Disabled()
	mgr.m.Add(disabledKey, disabled)
	mgr.m.Add(zeroKey, newTestOnline(t, zeroKey, 0))
	mgr.m.Add(availableKey, newTestOnline(t, availableKey, 1))

	online, err := mgr.ReserveByAvailableLoad()
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	if online.Key != availableKey {
		t.Fatalf("reserved key = %s, want %s", online.Key, availableKey)
	}
	if online, err = mgr.ReserveByAvailableLoad(); err == nil {
		t.Fatalf("reserve after valid load exhausted returned %s", online.Key)
	}
}

func newTestOnlineMgr() *OnlineMgr {
	return &OnlineMgr{
		m: xmap.NewMapMutexMgr[string, *Online](),
	}
}

func reserveOnlineKeys(t *testing.T, mgr *OnlineMgr, times int) map[string]int {
	t.Helper()

	counts := make(map[string]int)
	for i := 0; i < times; i++ {
		online, err := mgr.ReserveByAvailableLoad()
		if err != nil {
			t.Fatalf("reserve %d failed: %v", i, err)
		}
		counts[online.Key]++
	}
	return counts
}

func newTestOnline(t *testing.T, key string, availableLoad uint32) *Online {
	t.Helper()

	grpcAddr := startTestOnlineGRPC(t)
	xOnlineService, err := pb.NewXOnlineService(grpcAddr)
	if err != nil {
		t.Fatalf("new online service: %v", err)
	}
	t.Cleanup(func() {
		_ = xOnlineService.Stop()
	})

	return &Online{
		XOnlineService: xOnlineService,
		Key:            key,
		GroupID:        1,
		ServerName:     "online",
		ServerID:       1,
		PackageName:    "online",
		ServiceName:    "OnlineService",
		AvailableLoad:  availableLoad,
	}
}

func startTestOnlineGRPC(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen online grpc: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterOnlineServiceServer(server, &testOnlineService{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}

type testOnlineService struct {
	pb.UnimplementedOnlineServiceServer
}

func (p *testOnlineService) OnlineBindUser(context.Context, *pb.OnlineBindUserReq) (*pb.OnlineBindUserRes, error) {
	return &pb.OnlineBindUserRes{}, nil
}

func (p *testOnlineService) OnlineUnbindUser(context.Context, *pb.OnlineUnbindUserReq) (*pb.OnlineUnbindUserRes, error) {
	return &pb.OnlineUnbindUserRes{}, nil
}

func (p *testOnlineService) OnlineStreamTunnel(stream grpc.BidiStreamingServer[pb.OnlineStreamTunnelReq, pb.OnlineStreamTunnelRes]) error {
	for {
		if _, err := stream.Recv(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
