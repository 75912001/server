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
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

var (
	testLogInit           sync.Once
	testLogInitErr        error
	testCacheSelectorInit sync.Once
	testCacheServerSeq    atomic.Uint32
)

func TestAccountCreateReqAccountRecordNilReturnsInternal(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	account := &Account{uid: 10001, account: "account-10001"}

	account.onAccountCreateReq(gateway, newAccountCreatePacket(t, account.uid))

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.Internal.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	assertNoCacheSetAccountRecord(t, cache)
}

func TestAccountCreateReqCreatedAccountReturnsAlreadyExists(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	account := &Account{
		uid:     10002,
		account: "account-10002",
		accountRecord: &pb.AccountRecord{
			Uid:                            10002,
			Account:                        "account-10002",
			AccountCreateTimestampMs:       111,
			AccountRecordCreateTimestampMs: 222,
		},
	}

	account.onAccountCreateReq(gateway, newAccountCreatePacket(t, account.uid))

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.AlreadyExists.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	assertNoCacheSetAccountRecord(t, cache)
}

func TestAccountCreateReqExistingRecordCreatesAccount(t *testing.T) {
	cache := setupFakeCacheServer(t)
	gateway, stream := newGatewayWithStreamForTest(t)
	const accountCreateTimeMs int64 = 333
	account := &Account{
		uid:     10003,
		account: "account-10003",
		accountRecord: &pb.AccountRecord{
			Uid:                      10003,
			Account:                  "account-10003",
			AccountCreateTimestampMs: accountCreateTimeMs,
		},
	}

	account.onAccountCreateReq(gateway, newAccountCreatePacket(t, account.uid))

	cacheReq := readCacheSetAccountRecordReq(t, cache)
	record := cacheReq.GetAccountRecord()
	if got, want := cacheReq.GetUid(), account.uid; got != want {
		t.Fatalf("cache set uid = %d, want %d", got, want)
	}
	if got, want := record.GetUid(), account.uid; got != want {
		t.Fatalf("record uid = %d, want %d", got, want)
	}
	if got, want := record.GetAccount(), account.account; got != want {
		t.Fatalf("record account = %q, want %q", got, want)
	}
	if got, want := record.GetAccountCreateTimestampMs(), accountCreateTimeMs; got != want {
		t.Fatalf("record account create time = %d, want %d", got, want)
	}
	if record.GetAccountRecordCreateTimestampMs() == 0 {
		t.Fatal("record account create time is zero")
	}
	if got := len(record.GetCharacterRecordMap()); got == 0 {
		t.Fatalf("record character count = %d, want > 0", got)
	}
	for uuid, character := range record.GetCharacterRecordMap() {
		if character == nil {
			t.Fatalf("record character uuid %d is nil", uuid)
		}
		if got := character.GetUuid(); got != uuid {
			t.Fatalf("record character map key = %d, character uuid = %d", uuid, got)
		}
		if got, want := character.GetAssetId(), uint64(defaultCharacterID); got != want {
			t.Fatalf("record character asset id = %d, want %d", got, want)
		}
		if got := len(character.GetPetRecordMap()); got == 0 {
			t.Fatalf("record character pet count = %d, want > 0", got)
		}
	}
	if record.GetUsedUuid() == 0 {
		t.Fatal("record used uuid is zero")
	}

	pkt := readClientPacket(t, stream)
	if got, want := pkt.GetResultId(), xerror.Success.Code(); got != want {
		t.Fatalf("result id = %d, want %d", got, want)
	}
	var res pb.AccountCreateRes
	if err := proto.Unmarshal(pkt.GetBody(), &res); err != nil {
		t.Fatalf("unmarshal account create res: %v", err)
	}
	if got, want := res.GetAccountRecord().GetAccountRecordCreateTimestampMs(), record.GetAccountRecordCreateTimestampMs(); got != want {
		t.Fatalf("response account create time = %d, want %d", got, want)
	}
}

type fakeCacheService struct {
	pb.UnimplementedCacheServiceServer

	setAccountRecordCalls chan *pb.CacheSetAccountRecordReq
	getAccountRecord      func(uid uint64) (*pb.AccountRecord, error)
}

func (p *fakeCacheService) CacheSetAccountRecord(_ context.Context, req *pb.CacheSetAccountRecordReq) (*pb.CacheSetAccountRecordRes, error) {
	p.setAccountRecordCalls <- req
	return &pb.CacheSetAccountRecordRes{}, nil
}

func (p *fakeCacheService) CacheGetAccountRecord(_ context.Context, req *pb.CacheGetAccountRecordReq) (*pb.CacheGetAccountRecordRes, error) {
	if p.getAccountRecord == nil {
		return nil, grpcstatus.Error(grpccodes.NotFound, "account record not found")
	}
	accountRecord, err := p.getAccountRecord(req.GetUid())
	if err != nil {
		return nil, err
	}
	return &pb.CacheGetAccountRecordRes{
		AccountRecord: accountRecord,
	}, nil
}

func setupFakeCacheServer(t *testing.T, configure ...func(*fakeCacheService)) *fakeCacheService {
	t.Helper()

	ensureTestLog(t)
	testCacheSelectorInit.Do(func() {
		xgrpcprotoregistry.Init()
		xgrpcselector.Init()
	})

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	cache := &fakeCacheService{
		setAccountRecordCalls: make(chan *pb.CacheSetAccountRecordReq, 1),
	}
	for _, fn := range configure {
		fn(cache)
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

func newAccountCreatePacket(t *testing.T, uid uint64) *pb.OnlineClientPacket {
	t.Helper()

	body, err := proto.Marshal(&pb.AccountCreateReq{})
	if err != nil {
		t.Fatalf("marshal account create req: %v", err)
	}
	return &pb.OnlineClientPacket{
		MessageId: uint32(pb.MsgIDUser_AccountCreateReq_CMD),
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
		if got, want := pkt.GetMessageId(), uint32(pb.MsgIDUser_AccountCreateRes_CMD); got != want {
			t.Fatalf("message id = %d, want %d", got, want)
		}
		return pkt
	case <-time.After(time.Second):
		t.Fatal("timeout waiting client packet")
		return nil
	}
}

func readCacheSetAccountRecordReq(t *testing.T, cache *fakeCacheService) *pb.CacheSetAccountRecordReq {
	t.Helper()

	select {
	case req := <-cache.setAccountRecordCalls:
		return req
	case <-time.After(time.Second):
		t.Fatal("timeout waiting CacheSetAccountRecord")
		return nil
	}
}

func assertNoCacheSetAccountRecord(t *testing.T, cache *fakeCacheService) {
	t.Helper()

	select {
	case req := <-cache.setAccountRecordCalls:
		t.Fatalf("unexpected CacheSetAccountRecord uid=%d", req.GetUid())
	default:
	}
}
