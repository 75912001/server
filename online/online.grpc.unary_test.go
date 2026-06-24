package main

import (
	"context"
	"testing"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
)

func TestOnlineBindUserValidation(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	cache := setupFakeCacheGetAccountRecord(t, func(uid uint64) (*pb.AccountRecord, error) {
		return &pb.AccountRecord{
			Uid:                      uid,
			Account:                  "robot.other",
			AccountCreateTimestampMs: 111,
		}, nil
	})
	srv := &onlineGRPCServer{}
	ctx := context.Background()

	_, err := srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		Account:     "robot.1001",
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.Unauthenticated)
	if account := GAccountMgr.GetByUID(1001); account != nil {
		t.Fatalf("bind mismatch left account actor: %#v", account)
	}

	setFakeCacheGetAccountRecord(cache, func(uid uint64) (*pb.AccountRecord, error) {
		return &pb.AccountRecord{
			Uid:     uid,
			Account: "robot.1001",
		}, nil
	})
	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		Account:     "robot.1001",
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.Internal)
	if account := GAccountMgr.GetByUID(1001); account != nil {
		t.Fatalf("invalid record left account actor: %#v", account)
	}
}

func TestOnlineBindUserSuccess(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	_, err := setupTestOnlineGRPCServer(t).OnlineBindUser(context.Background(), validOnlineBindReq(1001, "robot.1001", "gateway-1", "session-1"))
	if err != nil {
		t.Fatalf("OnlineBindUser failed: %v", err)
	}

	account := GAccountMgr.GetByUID(1001)
	if account == nil {
		t.Fatal("bound account not found")
	}
	if account.gatewayKey != "gateway-1" || account.userSession != "session-1" || account.account != "robot.1001" {
		t.Fatalf("bound account state = gateway:%q session:%q account:%q", account.gatewayKey, account.userSession, account.account)
	}
}

func TestUserBindRejectsInvalidAccountCreateTime(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	_, err := GAccountMgr.Bind(1001, validOnlineBindReq(1001, "robot.1001", "gateway-1", "session-1"), &pb.AccountRecord{
		Uid:     1001,
		Account: "robot.1001",
	})
	requireStatusCode(t, err, grpccodes.Internal)
	if account := GAccountMgr.GetByUID(1001); account != nil {
		t.Fatalf("invalid record left account actor: %#v", account)
	}
}

func TestOnlineUnbindUserMissingActorSuccess(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	_, err := (&onlineGRPCServer{}).OnlineUnbindUser(context.Background(), &pb.OnlineUnbindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	if err != nil {
		t.Fatalf("OnlineUnbindUser missing actor failed: %v", err)
	}
}

func TestOnlineUnbindUserSessionMismatchKeepsActor(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	srv := setupTestOnlineGRPCServer(t)
	_, err := srv.OnlineBindUser(context.Background(), validOnlineBindReq(1001, "robot.1001", "gateway-1", "new-session"))
	if err != nil {
		t.Fatalf("OnlineBindUser failed: %v", err)
	}
	account := GAccountMgr.GetByUID(1001)
	if account == nil {
		t.Fatal("bound account not found")
	}

	_, err = srv.OnlineUnbindUser(context.Background(), &pb.OnlineUnbindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "old-session",
	})
	if err != nil {
		t.Fatalf("OnlineUnbindUser mismatch failed: %v", err)
	}
	if got := GAccountMgr.GetByUID(1001); got != account {
		t.Fatalf("mismatch unbind changed actor: got=%p want=%p", got, account)
	}
	if account.userSession != "new-session" {
		t.Fatalf("mismatch unbind changed userSession: %q", account.userSession)
	}
}

func TestOnlineUnbindUserMatchStopsActor(t *testing.T) {
	resetTestOnlineAccountMgr(t)

	srv := setupTestOnlineGRPCServer(t)
	_, err := srv.OnlineBindUser(context.Background(), validOnlineBindReq(1001, "robot.1001", "gateway-1", "session-1"))
	if err != nil {
		t.Fatalf("OnlineBindUser failed: %v", err)
	}

	_, err = srv.OnlineUnbindUser(context.Background(), &pb.OnlineUnbindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	if err != nil {
		t.Fatalf("OnlineUnbindUser failed: %v", err)
	}
	if account := GAccountMgr.GetByUID(1001); account != nil {
		t.Fatalf("unbound account still exists: %#v", account)
	}
}

func validOnlineBindReq(uid uint64, account string, gatewayKey string, userSession string) *pb.OnlineBindUserReq {
	return &pb.OnlineBindUserReq{
		Uid:         uid,
		Account:     account,
		GatewayKey:  gatewayKey,
		ClientIp:    "127.0.0.1",
		UserSession: userSession,
	}
}
