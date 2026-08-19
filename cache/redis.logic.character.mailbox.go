package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	pb "server/proto/pb"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

const (
	characterMailboxFieldUsedUUID = "usedUUID"
	characterMailboxFieldPrefix   = "mail:"
)

var (
	errCharacterMailboxFull          = errors.New("character mailbox is full")
	errCharacterMailboxUUIDExhausted = errors.New("character mailbox uuid is exhausted")
	errCharacterMailboxDataInvalid   = errors.New("character mailbox data is invalid")
	errCharacterMailNotFound         = errors.New("character mail not found")
)

func characterMailboxField(mailUUID uint64) string {
	return characterMailboxFieldPrefix + strconv.FormatUint(mailUUID, 10)
}

func characterMailExpireDuration() time.Duration {
	return time.Duration(pb.Constants_Constants_Mail_Expire_Days) * 24 * time.Hour
}

func (p *Redis) GetCharacterMailbox(ctx context.Context, aid uint64, characterUUID uint64) (*pb.MailboxRecord, error) {
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	values, err := p.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, errors.WithMessagef(err, "get character mailbox from redis failed, aid: %d, characterUUID: %d %v", aid, characterUUID, xruntime.Location())
	}
	mailboxRecord := &pb.MailboxRecord{MailRecordMap: make(map[uint64]*pb.MailRecord)}
	if len(values) == 0 {
		return mailboxRecord, nil
	}
	usedUUIDText, ok := values[characterMailboxFieldUsedUUID]
	if !ok {
		return nil, errors.Wrap(errCharacterMailboxDataInvalid, "usedUUID field is missing")
	}
	usedUUID, err := strconv.ParseUint(usedUUIDText, 10, 64)
	if err != nil || usedUUID == 0 || strconv.FormatUint(usedUUID, 10) != usedUUIDText {
		return nil, errors.Wrapf(errCharacterMailboxDataInvalid, "usedUUID field is invalid: %q", usedUUIDText)
	}
	mailboxRecord.UsedUuid = usedUUID
	nowTimestampMs := time.Now().UnixMilli()
	expiredFields := make([]string, 0)
	for field, value := range values {
		if field == characterMailboxFieldUsedUUID {
			continue
		}
		if !strings.HasPrefix(field, characterMailboxFieldPrefix) {
			return nil, errors.Wrapf(errCharacterMailboxDataInvalid, "unknown field: %q", field)
		}
		mailUUIDText := strings.TrimPrefix(field, characterMailboxFieldPrefix)
		mailUUID, parseErr := strconv.ParseUint(mailUUIDText, 10, 64)
		if parseErr != nil || mailUUID == 0 || strconv.FormatUint(mailUUID, 10) != mailUUIDText {
			return nil, errors.Wrapf(errCharacterMailboxDataInvalid, "mail field is invalid: %q", field)
		}
		mailRecord := &pb.MailRecord{}
		if unmarshalErr := proto.Unmarshal([]byte(value), mailRecord); unmarshalErr != nil {
			return nil, errors.Wrapf(errCharacterMailboxDataInvalid, "unmarshal mail failed, uuid: %d", mailUUID)
		}
		if nowTimestampMs >= mailRecord.GetExpireTimestampMs() {
			expiredFields = append(expiredFields, field)
			continue
		}
		mailboxRecord.MailRecordMap[mailUUID] = mailRecord
	}
	if len(expiredFields) > 0 {
		if err := p.client.HDel(ctx, key, expiredFields...).Err(); err != nil {
			return nil, errors.WithMessagef(err, "delete expired character mail from redis failed, aid: %d, characterUUID: %d %v", aid, characterUUID, xruntime.Location())
		}
	}
	return mailboxRecord, nil
}

func (p *Redis) characterMailboxMailCount(ctx context.Context, aid uint64, characterUUID uint64) (int64, error) {
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	fieldCount, err := p.client.HLen(ctx, key).Result()
	if err != nil {
		return 0, errors.WithMessagef(err, "get character mailbox field count failed, aid: %d, characterUUID: %d %v", aid, characterUUID, xruntime.Location())
	}
	if fieldCount == 0 {
		return 0, nil
	}
	return fieldCount - 1, nil
}

func (p *Redis) nextCharacterMailUUID(ctx context.Context, aid uint64, characterUUID uint64) (uint64, error) {
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	nextUUID, err := p.client.HIncrBy(ctx, key, characterMailboxFieldUsedUUID, 1).Result()
	if err != nil {
		return 0, errors.WithMessagef(err, "increment character mailbox usedUUID failed, aid: %d, characterUUID: %d %v", aid, characterUUID, xruntime.Location())
	}
	if nextUUID <= 0 {
		return 0, errCharacterMailboxUUIDExhausted
	}
	return uint64(nextUUID), nil
}

func (p *Redis) AddSystemMail(ctx context.Context, aid uint64, characterUUID uint64, title string, content string) (*pb.MailRecord, error) {
	mailCount, err := p.characterMailboxMailCount(ctx, aid, characterUUID)
	if err != nil {
		return nil, err
	}
	if mailCount >= int64(pb.Constants_Constants_Mail_System_Inbox_Max_Count) {
		mailboxRecord, err := p.GetCharacterMailbox(ctx, aid, characterUUID)
		if err != nil {
			return nil, err
		}
		if len(mailboxRecord.GetMailRecordMap()) >= int(pb.Constants_Constants_Mail_System_Inbox_Max_Count) {
			return nil, errCharacterMailboxFull
		}
	}

	mailUUID, err := p.nextCharacterMailUUID(ctx, aid, characterUUID)
	if err != nil {
		return nil, err
	}
	createTimestampMs := time.Now().UnixMilli()
	mailRecord := &pb.MailRecord{
		Uuid:              mailUUID,
		Title:             title,
		Content:           content,
		CreateTimestampMs: createTimestampMs,
		ExpireTimestampMs: createTimestampMs + characterMailExpireDuration().Milliseconds(),
	}
	data, err := proto.Marshal(mailRecord)
	if err != nil {
		return nil, errors.WithMessagef(err, "marshal character mail failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	if err := p.client.HSet(ctx, key, characterMailboxField(mailUUID), data).Err(); err != nil {
		return nil, errors.WithMessagef(err, "set character mail to redis failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	return mailRecord, nil
}

func (p *Redis) getCharacterMail(ctx context.Context, aid uint64, characterUUID uint64, mailUUID uint64) (*pb.MailRecord, error) {
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	field := characterMailboxField(mailUUID)
	value, err := p.client.HGet(ctx, key, field).Result()
	if errors.Is(err, redis.Nil) {
		return nil, errCharacterMailNotFound
	}
	if err != nil {
		return nil, errors.WithMessagef(err, "get character mail from redis failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	mailRecord := &pb.MailRecord{}
	if err := proto.Unmarshal([]byte(value), mailRecord); err != nil {
		return nil, errors.Wrapf(errCharacterMailboxDataInvalid, "unmarshal mail failed, uuid: %d", mailUUID)
	}
	if time.Now().UnixMilli() >= mailRecord.GetExpireTimestampMs() {
		if err := p.client.HDel(ctx, key, field).Err(); err != nil {
			return nil, errors.WithMessagef(err, "delete expired character mail from redis failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
		}
		return nil, errCharacterMailNotFound
	}
	return mailRecord, nil
}

func (p *Redis) MarkCharacterMailRead(ctx context.Context, aid uint64, characterUUID uint64, mailUUID uint64) error {
	mailRecord, err := p.getCharacterMail(ctx, aid, characterUUID, mailUUID)
	if err != nil || mailRecord.GetIsRead() {
		return err
	}
	mailRecord.IsRead = true
	data, err := proto.Marshal(mailRecord)
	if err != nil {
		return errors.WithMessagef(err, "marshal read character mail failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	if err := p.client.HSet(ctx, key, characterMailboxField(mailUUID), data).Err(); err != nil {
		return errors.WithMessagef(err, "set read character mail to redis failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	return nil
}

func (p *Redis) DeleteCharacterMail(ctx context.Context, aid uint64, characterUUID uint64, mailUUID uint64) error {
	if _, err := p.getCharacterMail(ctx, aid, characterUUID, mailUUID); err != nil {
		return err
	}
	key := RedisKeyCharacterMailbox(aid, characterUUID)
	deleted, err := p.client.HDel(ctx, key, characterMailboxField(mailUUID)).Result()
	if err != nil {
		return errors.WithMessagef(err, "delete character mail from redis failed, aid: %d, characterUUID: %d, mailUUID: %d %v", aid, characterUUID, mailUUID, xruntime.Location())
	}
	if deleted == 0 {
		return errCharacterMailNotFound
	}
	return nil
}
