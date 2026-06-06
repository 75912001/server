package main

import (
	"context"
	"testing"

	pb "server/proto/pb"

	grpccodes "google.golang.org/grpc/codes"
)

func TestOnlineBindUserValidation(t *testing.T) {
	resetTestOnlineUserMgr(t)

	srv := newTestOnlineGRPCServer()
	ctx := context.Background()

	_, err := srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.InvalidArgument)

	srv = &onlineGRPCServer{
		cacheGetUserRecord: func(uid uint64) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				Uid:                 uid,
				Account:             "robot.other",
				AccountCreateTimeMs: 111,
			}, nil
		},
	}
	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		Account:     "robot.1001",
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.Unauthenticated)
	if user := GUserMgr.GetByUID(1001); user != nil {
		t.Fatalf("bind mismatch left user actor: %#v", user)
	}

	srv = &onlineGRPCServer{
		cacheGetUserRecord: func(uid uint64) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				Uid:     uid,
				Account: "robot.1001",
			}, nil
		},
	}
	_, err = srv.OnlineBindUser(ctx, &pb.OnlineBindUserReq{
		Uid:         1001,
		Account:     "robot.1001",
		GatewayKey:  "gateway-1",
		UserSession: "session-1",
	})
	requireStatusCode(t, err, grpccodes.Internal)
	if user := GUserMgr.GetByUID(1001); user != nil {
		t.Fatalf("invalid record left user actor: %#v", user)
	}
}

func TestOnlineBindUserSuccess(t *testing.T) {
	resetTestOnlineUserMgr(t)

	_, err := newTestOnlineGRPCServer().OnlineBindUser(context.Background(), validOnlineBindReq(1001, "robot.1001", "gateway-1", "session-1"))
	if err != nil {
		t.Fatalf("OnlineBindUser failed: %v", err)
	}

	user := GUserMgr.GetByUID(1001)
	if user == nil {
		t.Fatal("bound user not found")
	}
	if user.gatewayID != "gateway-1" || user.userSession != "session-1" || user.account != "robot.1001" {
		t.Fatalf("bound user state = gateway:%q session:%q account:%q", user.gatewayID, user.userSession, user.account)
	}
}

func TestUserBindRejectsInvalidAccountCreateTime(t *testing.T) {
	resetTestOnlineUserMgr(t)

	_, err := GUserMgr.Bind(1001, validOnlineBindReq(1001, "robot.1001", "gateway-1", "session-1"), &pb.UserRecord{
		Uid:     1001,
		Account: "robot.1001",
	})
	requireStatusCode(t, err, grpccodes.Internal)
	if user := GUserMgr.GetByUID(1001); user != nil {
		t.Fatalf("invalid record left user actor: %#v", user)
	}
}

func TestOnlineUnbindUserMissingActorSuccess(t *testing.T) {
	resetTestOnlineUserMgr(t)

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
	resetTestOnlineUserMgr(t)

	srv := newTestOnlineGRPCServer()
	_, err := srv.OnlineBindUser(context.Background(), validOnlineBindReq(1001, "robot.1001", "gateway-1", "new-session"))
	if err != nil {
		t.Fatalf("OnlineBindUser failed: %v", err)
	}
	user := GUserMgr.GetByUID(1001)
	if user == nil {
		t.Fatal("bound user not found")
	}

	_, err = srv.OnlineUnbindUser(context.Background(), &pb.OnlineUnbindUserReq{
		Uid:         1001,
		GatewayKey:  "gateway-1",
		UserSession: "old-session",
	})
	if err != nil {
		t.Fatalf("OnlineUnbindUser mismatch failed: %v", err)
	}
	if got := GUserMgr.GetByUID(1001); got != user {
		t.Fatalf("mismatch unbind changed actor: got=%p want=%p", got, user)
	}
	if user.userSession != "new-session" {
		t.Fatalf("mismatch unbind changed userSession: %q", user.userSession)
	}
}

func TestOnlineUnbindUserMatchStopsActor(t *testing.T) {
	resetTestOnlineUserMgr(t)

	srv := newTestOnlineGRPCServer()
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
	if user := GUserMgr.GetByUID(1001); user != nil {
		t.Fatalf("unbound user still exists: %#v", user)
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
