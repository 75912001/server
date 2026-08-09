package main

import (
	"context"

	pb "server/proto/pb"

	"github.com/pkg/errors"
	grpcstatus "google.golang.org/grpc/status"
)

func unaryCacheSetAccountRecord(aid uint64, accountRecord *pb.AccountRecord) error {
	_, err := pb.GXCacheServiceService.CacheSetAccountRecord(context.Background(), &pb.CacheSetAccountRecordReq{
		Aid:           aid,
		AccountRecord: accountRecord,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d, code:%v, message:%s", aid, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheSetAccountRecord aid:%d", aid)
	}
	return nil
}

func unaryCacheGetAccountRecord(aid uint64) (*pb.AccountRecord, error) {
	res, err := pb.GXCacheServiceService.CacheGetAccountRecord(context.Background(), &pb.CacheGetAccountRecordReq{
		Aid: aid,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheGetAccountRecord aid:%d, code:%v, message:%s", aid, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetAccountRecord aid:%d", aid)
	}
	return res.GetAccountRecord(), nil
}

func unaryCacheGetCharacterMailbox(aid uint64, characterUUID uint64) (*pb.MailboxRecord, error) {
	res, err := pb.GXCacheServiceService.CacheGetCharacterMailbox(context.Background(), &pb.CacheGetCharacterMailboxReq{
		Aid:           aid,
		CharacterUuid: characterUUID,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheGetCharacterMailbox aid:%d character:%d, code:%v, message:%s", aid, characterUUID, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheGetCharacterMailbox aid:%d character:%d", aid, characterUUID)
	}
	return res.GetMailboxRecord(), nil
}

func unaryCacheAddSystemMail(aid uint64, characterUUID uint64, title string, content string) (*pb.MailRecord, error) {
	res, err := pb.GXCacheServiceService.CacheAddSystemMail(context.Background(), &pb.CacheAddSystemMailReq{
		Aid:           aid,
		CharacterUuid: characterUUID,
		Title:         title,
		Content:       content,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return nil, errors.WithMessagef(err, "CacheAddSystemMail aid:%d character:%d, code:%v, message:%s", aid, characterUUID, s.Code(), s.Message())
		}
		return nil, errors.WithMessagef(err, "CacheAddSystemMail aid:%d character:%d", aid, characterUUID)
	}
	return res.GetMailRecord(), nil
}

func unaryCacheMarkCharacterMailRead(aid uint64, characterUUID uint64, mailUUID uint64) error {
	_, err := pb.GXCacheServiceService.CacheMarkCharacterMailRead(context.Background(), &pb.CacheMarkCharacterMailReadReq{
		Aid:           aid,
		CharacterUuid: characterUUID,
		MailUuid:      mailUUID,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheMarkCharacterMailRead aid:%d character:%d mail:%d, code:%v, message:%s", aid, characterUUID, mailUUID, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheMarkCharacterMailRead aid:%d character:%d mail:%d", aid, characterUUID, mailUUID)
	}
	return nil
}

func unaryCacheDeleteCharacterMail(aid uint64, characterUUID uint64, mailUUID uint64) error {
	_, err := pb.GXCacheServiceService.CacheDeleteCharacterMail(context.Background(), &pb.CacheDeleteCharacterMailReq{
		Aid:           aid,
		CharacterUuid: characterUUID,
		MailUuid:      mailUUID,
	})
	if err != nil {
		if s, ok := grpcstatus.FromError(err); ok {
			return errors.WithMessagef(err, "CacheDeleteCharacterMail aid:%d character:%d mail:%d, code:%v, message:%s", aid, characterUUID, mailUUID, s.Code(), s.Message())
		}
		return errors.WithMessagef(err, "CacheDeleteCharacterMail aid:%d character:%d mail:%d", aid, characterUUID, mailUUID)
	}
	return nil
}
