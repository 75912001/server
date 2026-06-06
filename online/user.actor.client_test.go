package main

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"server/common"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
	xgrpcprotoregistry "github.com/75912001/xlib/grpc/proto/registry"
	xgrpcresolve "github.com/75912001/xlib/grpc/resolve"
	xgrpcselector "github.com/75912001/xlib/grpc/selector"
	xlog "github.com/75912001/xlib/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

var (
	testLogInit           sync.Once
	testLogInitErr        error
	testCacheSelectorInit sync.Once
	testCacheServerSeq    atomic.Uint32
)

func TestUserCreateReqUserRecordNilReturnsInternal(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	user := &User{uid: 10001, account: "account-10001"}

	user.onUserCreateReq(gateway, newUserCreatePacket(t, user.uid))

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.Internal.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	assertNoCacheSetUserRecord(t, cache)
}

func TestUserCreateReqCreatedUserReturnsAlreadyExists(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	user := &User{
		uid:     10002,
		account: "account-10002",
		userRecord: &pb.UserRecord{
			Uid:                 10002,
			Account:             "account-10002",
			AccountCreateTimeMs: 111,
			UserCreateTimeMs:    222,
		},
	}

	user.onUserCreateReq(gateway, newUserCreatePacket(t, user.uid))

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.AlreadyExists.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	assertNoCacheSetUserRecord(t, cache)
}

func TestUserCreateReqExistingRecordCreatesUser(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	const accountCreateTimeMs int64 = 333
	user := &User{
		uid:     10003,
		account: "account-10003",
		userRecord: &pb.UserRecord{
			Uid:                 10003,
			Account:             "account-10003",
			AccountCreateTimeMs: accountCreateTimeMs,
		},
	}

	user.onUserCreateReq(gateway, newUserCreatePacket(t, user.uid))

	cacheReq := readCacheSetUserRecordReq(t, cache)
	record := cacheReq.GetUserRecord()
	if got, want := cacheReq.GetUid(), user.uid; got != want {
		t.Fatalf("cache set uid = %d, want %d", got, want)
	}
	if got, want := record.GetUid(), user.uid; got != want {
		t.Fatalf("record uid = %d, want %d", got, want)
	}
	if got, want := record.GetAccount(), user.account; got != want {
		t.Fatalf("record account = %q, want %q", got, want)
	}
	if got, want := record.GetAccountCreateTimeMs(), accountCreateTimeMs; got != want {
		t.Fatalf("record account create time = %d, want %d", got, want)
	}
	if record.GetUserCreateTimeMs() == 0 {
		t.Fatal("record user create time is zero")
	}

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.Success.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	var res pb.UserCreateRes
	if err := proto.Unmarshal(pkt.GetBody(), &res); err != nil {
		t.Fatalf("unmarshal user create res: %v", err)
	}
	if got, want := res.GetUserRecord().GetUserCreateTimeMs(), record.GetUserCreateTimeMs(); got != want {
		t.Fatalf("response user create time = %d, want %d", got, want)
	}
}

type fakeCacheService struct {
	pb.UnimplementedCacheServiceServer

	setUserRecordCalls chan *pb.CacheSetUserRecordReq
}

func (p *fakeCacheService) CacheSetUserRecord(_ context.Context, req *pb.CacheSetUserRecordReq) (*pb.CacheSetUserRecordRes, error) {
	p.setUserRecordCalls <- req
	return &pb.CacheSetUserRecordRes{}, nil
}

func setupFakeCacheServer(t *testing.T) *fakeCacheService {
	t.Helper()

	ensureTestLog(t)
	testCacheSelectorInit.Do(func() {
		xgrpcprotoregistry.Init()
		xgrpcselector.Init()
	})

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	cache := &fakeCacheService{
		setUserRecordCalls: make(chan *pb.CacheSetUserRecordReq, 1),
	}
	pb.RegisterCacheServiceServer(server, cache)
	go func() {
		_ = server.Serve(listener)
	}()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial fake cache: %v", err)
	}

	groupID := uint32(65000)
	serverID := uint32(65000) + testCacheServerSeq.Add(1)
	clientConn := newFakeCacheClientConn("fake-cache", conn)
	xgrpcresolve.AddServer(groupID, common.CacheServerName, serverID, clientConn, "cache", "CacheService")
	t.Cleanup(func() {
		_, _ = xgrpcresolve.RemoveServer(groupID, common.CacheServerName, serverID, "cache", "CacheService")
		server.Stop()
		_ = listener.Close()
	})
	return cache
}

type fakeCacheClientConn struct {
	id        string
	conn      *grpc.ClientConn
	available atomic.Bool
}

func newFakeCacheClientConn(id string, conn *grpc.ClientConn) *fakeCacheClientConn {
	clientConn := &fakeCacheClientConn{id: id, conn: conn}
	clientConn.available.Store(true)
	return clientConn
}

func (p *fakeCacheClientConn) GetClientConn() *grpc.ClientConn { return p.conn }
func (p *fakeCacheClientConn) Disabled()                       { p.available.Store(false) }
func (p *fakeCacheClientConn) Available() bool                 { return p.available.Load() }
func (p *fakeCacheClientConn) Stop() error                     { return p.conn.Close() }
func (p *fakeCacheClientConn) GetID() string                   { return p.id }

type fakeOnlineTunnelStream struct {
	sent chan *pb.OnlineStreamTunnelRes
}

func newFakeOnlineTunnelStream() *fakeOnlineTunnelStream {
	return &fakeOnlineTunnelStream{
		sent: make(chan *pb.OnlineStreamTunnelRes, 1),
	}
}

func (p *fakeOnlineTunnelStream) Recv() (*pb.OnlineStreamTunnelReq, error) {
	return nil, io.EOF
}

func (p *fakeOnlineTunnelStream) Send(res *pb.OnlineStreamTunnelRes) error {
	p.sent <- res
	return nil
}

func (p *fakeOnlineTunnelStream) SetHeader(metadata.MD) error  { return nil }
func (p *fakeOnlineTunnelStream) SendHeader(metadata.MD) error { return nil }
func (p *fakeOnlineTunnelStream) SetTrailer(metadata.MD)       {}
func (p *fakeOnlineTunnelStream) Context() context.Context     { return context.Background() }
func (p *fakeOnlineTunnelStream) SendMsg(any) error            { return nil }
func (p *fakeOnlineTunnelStream) RecvMsg(any) error            { return io.EOF }

func newGatewayWithStreamForTest(t *testing.T) (*Gateway, *fakeOnlineTunnelStream) {
	t.Helper()

	ensureTestLog(t)
	gateway := newGateway("gateway-test")
	stream := newFakeOnlineTunnelStream()
	gateway.BindStream(stream)
	t.Cleanup(func() {
		_ = gateway.Stop()
	})
	return gateway, stream
}

func ensureTestLog(t *testing.T) {
	t.Helper()

	testLogInit.Do(func() {
		if xlog.GLog != nil {
			return
		}
		xlog.GLog, testLogInitErr = xlog.NewMgr(xlog.NewOptions().
			WithIsWriteFile(false).
			WithIsReportCaller(false),
		)
	})
	if testLogInitErr != nil {
		t.Fatalf("init test log: %v", testLogInitErr)
	}
}

func newUserCreatePacket(t *testing.T, uid uint64) *pb.OnlineClientPacket {
	t.Helper()

	body, err := proto.Marshal(&pb.UserCreateReq{})
	if err != nil {
		t.Fatalf("marshal user create req: %v", err)
	}
	return &pb.OnlineClientPacket{
		MessageId: uint32(pb.MsgIDUser_UserCreateReq_CMD),
		SessionId: 123,
		Key:       uid,
		Body:      body,
	}
}

func readClientPacket(t *testing.T, stream *fakeOnlineTunnelStream) *pb.OnlineClientPacket {
	t.Helper()

	select {
	case res := <-stream.sent:
		frames := res.GetFrames()
		if len(frames) != 1 {
			t.Fatalf("frame count = %d, want 1", len(frames))
		}
		pkt := frames[0].GetClientPacket()
		if pkt == nil {
			t.Fatal("client packet is nil")
		}
		if got, want := pkt.GetMessageId(), uint32(pb.MsgIDUser_UserCreateRes_CMD); got != want {
			t.Fatalf("message id = %d, want %d", got, want)
		}
		return pkt
	case <-time.After(time.Second):
		t.Fatal("timeout waiting client packet")
		return nil
	}
}

func readCacheSetUserRecordReq(t *testing.T, cache *fakeCacheService) *pb.CacheSetUserRecordReq {
	t.Helper()

	select {
	case req := <-cache.setUserRecordCalls:
		return req
	case <-time.After(time.Second):
		t.Fatal("timeout waiting CacheSetUserRecord")
		return nil
	}
}

func assertNoCacheSetUserRecord(t *testing.T, cache *fakeCacheService) {
	t.Helper()

	select {
	case req := <-cache.setUserRecordCalls:
		t.Fatalf("unexpected CacheSetUserRecord uid=%d", req.GetUid())
	default:
	}
}
