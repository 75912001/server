package main

import (
	"context"

	"server/common"
	pb "server/proto/pb"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func validateCharacterMailboxOwner(ctx context.Context, aid uint64, characterUUID uint64) error {
	accountRecord, err := GRedis.GetAccountRecord(ctx, aid)
	if errors.Is(err, redis.Nil) {
		return grpcstatus.Error(grpccodes.NotFound, "account record not found")
	}
	if err != nil {
		return grpcstatus.Error(grpccodes.Internal, err.Error())
	}
	if accountRecord.GetAid() != aid {
		return grpcstatus.Error(grpccodes.Internal, "account record aid mismatch")
	}
	for _, characterRecord := range accountRecord.GetCharacterRecordList() {
		if characterRecord.GetBase().GetUuid() == characterUUID {
			return nil
		}
	}
	return grpcstatus.Error(grpccodes.NotFound, "character not found")
}

func characterMailboxGRPCError(err error) error {
	switch {
	case errors.Is(err, errCharacterMailboxFull):
		return grpcstatus.Error(grpccodes.ResourceExhausted, err.Error())
	case errors.Is(err, errCharacterMailboxUUIDExhausted):
		return grpcstatus.Error(grpccodes.FailedPrecondition, err.Error())
	case errors.Is(err, errCharacterMailNotFound):
		return grpcstatus.Error(grpccodes.NotFound, err.Error())
	default:
		return grpcstatus.Error(grpccodes.Internal, err.Error())
	}
}

func (s *cacheGRPCServer) CacheGetCharacterMailbox(ctx context.Context, req *pb.CacheGetCharacterMailboxReq) (*pb.CacheGetCharacterMailboxRes, error) {
	aid := req.GetAid()
	characterUUID := req.GetCharacterUuid()
	if aid == 0 || characterUUID == 0 {
		return &pb.CacheGetCharacterMailboxRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := validateCharacterMailboxOwner(ctx, aid, characterUUID); err != nil {
		return &pb.CacheGetCharacterMailboxRes{}, err
	}
	mailboxRecord, err := GRedis.GetCharacterMailbox(ctx, aid, characterUUID)
	if err != nil {
		return &pb.CacheGetCharacterMailboxRes{}, characterMailboxGRPCError(err)
	}
	return &pb.CacheGetCharacterMailboxRes{MailboxRecord: mailboxRecord}, nil
}

func (s *cacheGRPCServer) CacheAddSystemMail(ctx context.Context, req *pb.CacheAddSystemMailReq) (*pb.CacheAddSystemMailRes, error) {
	aid := req.GetAid()
	characterUUID := req.GetCharacterUuid()
	if aid == 0 || characterUUID == 0 {
		return &pb.CacheAddSystemMailRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	title, content, err := common.NormalizeSystemMailText(req.GetTitle(), req.GetContent())
	if err != nil {
		return &pb.CacheAddSystemMailRes{}, grpcstatus.Error(grpccodes.InvalidArgument, err.Error())
	}
	if err := validateCharacterMailboxOwner(ctx, aid, characterUUID); err != nil {
		return &pb.CacheAddSystemMailRes{}, err
	}
	mailRecord, err := GRedis.AddSystemMail(ctx, aid, characterUUID, title, content)
	if err != nil {
		return &pb.CacheAddSystemMailRes{}, characterMailboxGRPCError(err)
	}
	return &pb.CacheAddSystemMailRes{MailRecord: mailRecord}, nil
}

func (s *cacheGRPCServer) CacheMarkCharacterMailRead(ctx context.Context, req *pb.CacheMarkCharacterMailReadReq) (*pb.CacheMarkCharacterMailReadRes, error) {
	aid := req.GetAid()
	characterUUID := req.GetCharacterUuid()
	mailUUID := req.GetMailUuid()
	if aid == 0 || characterUUID == 0 || mailUUID == 0 {
		return &pb.CacheMarkCharacterMailReadRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := validateCharacterMailboxOwner(ctx, aid, characterUUID); err != nil {
		return &pb.CacheMarkCharacterMailReadRes{}, err
	}
	if err := GRedis.MarkCharacterMailRead(ctx, aid, characterUUID, mailUUID); err != nil {
		return &pb.CacheMarkCharacterMailReadRes{}, characterMailboxGRPCError(err)
	}
	return &pb.CacheMarkCharacterMailReadRes{}, nil
}

func (s *cacheGRPCServer) CacheDeleteCharacterMail(ctx context.Context, req *pb.CacheDeleteCharacterMailReq) (*pb.CacheDeleteCharacterMailRes, error) {
	aid := req.GetAid()
	characterUUID := req.GetCharacterUuid()
	mailUUID := req.GetMailUuid()
	if aid == 0 || characterUUID == 0 || mailUUID == 0 {
		return &pb.CacheDeleteCharacterMailRes{}, grpcstatus.Error(grpccodes.InvalidArgument, "invalid argument")
	}
	if err := validateCharacterMailboxOwner(ctx, aid, characterUUID); err != nil {
		return &pb.CacheDeleteCharacterMailRes{}, err
	}
	if err := GRedis.DeleteCharacterMail(ctx, aid, characterUUID, mailUUID); err != nil {
		return &pb.CacheDeleteCharacterMailRes{}, characterMailboxGRPCError(err)
	}
	return &pb.CacheDeleteCharacterMailRes{}, nil
}
