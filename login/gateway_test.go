package main

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	pb "server/proto/pb"

	xetcd "github.com/75912001/xlib/etcd"
	xlog "github.com/75912001/xlib/log"
	xnetcommon "github.com/75912001/xlib/net/common"
	"google.golang.org/grpc"
)

var (
	testGatewayLogInit    sync.Once
	testGatewayLogInitErr error
)

func TestGatewayMgrReserveByAvailableLoadDistributesEqualLoad(t *testing.T) {
	mgr := newGatewayMgr()
	key1 := "/project/server/1/gateway/1/"
	key2 := "/project/server/1/gateway/2/"
	mgr.m.Add(key1, newTestGateway(t, key1, 2))
	mgr.m.Add(key2, newTestGateway(t, key2, 2))

	counts := reserveGatewayKeys(t, mgr, 4)
	if counts[key1] != 2 || counts[key2] != 2 {
		t.Fatalf("reserve counts = %v, want %s=2 and %s=2", counts, key1, key2)
	}
	if gateway, ok := mgr.ReserveByAvailableLoad(); ok {
		t.Fatalf("reserve after exhausted returned %s", gateway.Key)
	}
}

func TestGatewayMgrReserveByAvailableLoadConsumesDifferentLoad(t *testing.T) {
	mgr := newGatewayMgr()
	key1 := "/project/server/1/gateway/1/"
	key2 := "/project/server/1/gateway/2/"
	mgr.m.Add(key1, newTestGateway(t, key1, 3))
	mgr.m.Add(key2, newTestGateway(t, key2, 1))

	counts := reserveGatewayKeys(t, mgr, 4)
	if counts[key1] != 3 || counts[key2] != 1 {
		t.Fatalf("reserve counts = %v, want %s=3 and %s=1", counts, key1, key2)
	}
}

func TestGatewayMgrUpdateOverridesReservedAvailableLoad(t *testing.T) {
	ensureGatewayTestLog(t)

	mgr := newGatewayMgr()
	key := "/project/server/1/gateway/1/"
	gateway := newTestGateway(t, key, 2)
	mgr.m.Add(key, gateway)

	if _, ok := mgr.ReserveByAvailableLoad(); !ok {
		t.Fatal("reserve failed")
	}
	if got, want := atomic.LoadUint32(&gateway.AvailableLoad), uint32(1); got != want {
		t.Fatalf("available load after reserve = %d, want %d", got, want)
	}

	if err := mgr.Update(key, newTestGatewayValue(gateway.GrpcAddr, 5)); err != nil {
		t.Fatalf("update gateway: %v", err)
	}
	if got, want := atomic.LoadUint32(&gateway.AvailableLoad), uint32(5); got != want {
		t.Fatalf("available load after update = %d, want %d", got, want)
	}
}

func ensureGatewayTestLog(t *testing.T) {
	t.Helper()

	testGatewayLogInit.Do(func() {
		if xlog.GLog != nil {
			return
		}
		xlog.GLog, testGatewayLogInitErr = xlog.NewMgr(xlog.NewOptions().
			WithIsWriteFile(false).
			WithIsReportCaller(false),
		)
	})
	if testGatewayLogInitErr != nil {
		t.Fatalf("init test log: %v", testGatewayLogInitErr)
	}
}

func TestGatewayMgrReserveByAvailableLoadSkipsZeroAndUnavailable(t *testing.T) {
	mgr := newGatewayMgr()
	disabledKey := "/project/server/1/gateway/1/"
	zeroKey := "/project/server/1/gateway/2/"
	availableKey := "/project/server/1/gateway/3/"
	disabled := newTestGateway(t, disabledKey, 5)
	disabled.Disabled()
	mgr.m.Add(disabledKey, disabled)
	mgr.m.Add(zeroKey, newTestGateway(t, zeroKey, 0))
	mgr.m.Add(availableKey, newTestGateway(t, availableKey, 1))

	gateway, ok := mgr.ReserveByAvailableLoad()
	if !ok {
		t.Fatal("reserve failed")
	}
	if gateway.Key != availableKey {
		t.Fatalf("reserved key = %s, want %s", gateway.Key, availableKey)
	}
	if gateway, ok = mgr.ReserveByAvailableLoad(); ok {
		t.Fatalf("reserve after valid load exhausted returned %s", gateway.Key)
	}
}

func reserveGatewayKeys(t *testing.T, mgr *GatewayMgr, times int) map[string]int {
	t.Helper()

	counts := make(map[string]int)
	for i := 0; i < times; i++ {
		gateway, ok := mgr.ReserveByAvailableLoad()
		if !ok {
			t.Fatalf("reserve %d failed", i)
		}
		counts[gateway.Key]++
	}
	return counts
}

func newTestGateway(t *testing.T, key string, availableLoad uint32) *Gateway {
	t.Helper()

	grpcAddr := startTestGatewayGRPC(t)
	xGatewayService, err := pb.NewXGatewayService(grpcAddr)
	if err != nil {
		t.Fatalf("new gateway service: %v", err)
	}
	t.Cleanup(func() {
		_ = xGatewayService.Stop()
	})

	return &Gateway{
		XGatewayService: xGatewayService,
		Key:             key,
		Addr:            "127.0.0.1:10101",
		GrpcAddr:        grpcAddr,
		GroupID:         1,
		ServerName:      "gateway",
		ServerID:        1,
		AvailableLoad:   availableLoad,
	}
}

func newTestGatewayValue(grpcAddr string, availableLoad uint32) *xetcd.ValueJson {
	tcpAddr := "127.0.0.1:10101"
	tcpType := xnetcommon.ServerNetTypeNameTCP
	packageName := "gateway"
	serviceName := "GatewayService"
	return &xetcd.ValueJson{
		AvailableLoad: availableLoad,
		ServerNet: []*xetcd.ServerNet{
			{
				Addr: &tcpAddr,
				Type: &tcpType,
			},
		},
		GrpcService: &xetcd.GrpcService{
			PackageName: &packageName,
			ServiceName: &serviceName,
			Addr:        &grpcAddr,
		},
	}
}

func startTestGatewayGRPC(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen gateway grpc: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterGatewayServiceServer(server, &testGatewayService{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr().String()
}

type testGatewayService struct {
	pb.UnimplementedGatewayServiceServer
}

func (p *testGatewayService) GatewayKickUser(context.Context, *pb.GatewayKickUserReq) (*pb.GatewayKickUserRes, error) {
	return &pb.GatewayKickUserRes{}, nil
}
