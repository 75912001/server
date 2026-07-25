package main

import (
	"context"
	"server/common"
	"server/proto/pb"
	"strconv"
	"time"

	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// SetAccountVerifyToken 写入 accountVerifyToken。
// 使用 SETNX 保证未消费的 accountVerifyToken 不会被覆盖, 返回 false 表示 key 已存在。
func (p *Redis) SetAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string, expire time.Duration) (bool, error) {
	key := RedisKeyAccountVerifyToken(account)
	return p.client.SetNX(ctx, key, accountVerifyToken, expire).Result()
}

// useAccountVerifyTokenScript 原子校验并消费 accountVerifyToken。
// 成功时删除 accountVerifyToken key; key 不存在或 accountVerifyToken 不匹配时返回 0。
const useAccountVerifyTokenScript = `
local current = redis.call("GET", KEYS[1])
if current == false then
	return 0
end
if current ~= ARGV[1] then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`

// UseAccountVerifyToken 验证并消费 accountVerifyToken。
// 消费成功后删除 accountVerifyToken key, 避免同一 accountVerifyToken 被重复使用。
func (p *Redis) UseAccountVerifyToken(ctx context.Context, account string, accountVerifyToken string) (bool, error) {
	key := RedisKeyAccountVerifyToken(account)
	result, err := p.client.Eval(ctx, useAccountVerifyTokenScript, []string{key}, accountVerifyToken).Result()
	if err != nil {
		return false, errors.WithMessagef(err, "use accountVerifyToken from redis failed, account: %s, accountVerifyToken: %s %v", account, accountVerifyToken, xruntime.Location())
	}
	return redisScriptResultIsOK(result), nil
}

// EnsureAccount 确保 account 有唯一 aid 和 AccountRecord。
// 返回值 created 表示本次调用是否新建了账号。
func (p *Redis) EnsureAccount(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	for {
		accountRecord, found, err := p.GetAccountRecordByAccount(ctx, account)
		if err != nil || found {
			return accountRecord, false, err
		}

		locked, err := p.client.SetNX(ctx, RedisKeyAccountLock(account), "1", GCfgCustomRedisAccountCreateLockDuration).Result()
		if err != nil {
			return nil, false, errors.WithMessagef(err, "lock account create failed, account: %s %v", account, xruntime.Location())
		}
		if !locked {
			if err = sleepContext(ctx, 20*time.Millisecond); err != nil {
				return nil, false, err
			}
			continue
		}

		accountRecord, created, err := p.createAccountAfterLock(ctx, account)
		if _, unlockErr := p.client.Del(ctx, RedisKeyAccountLock(account)).Result(); unlockErr != nil && err == nil {
			err = errors.WithMessagef(unlockErr, "unlock account create failed, account: %s %v", account, xruntime.Location())
		}
		return accountRecord, created, err
	}
}

// createAccountAfterLock 在持有账号创建锁后创建账号数据。
// 创建前会再次查询账号映射，避免等待锁期间其他请求已经完成创建。
func (p *Redis) createAccountAfterLock(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	accountRecord, found, err := p.GetAccountRecordByAccount(ctx, account)
	if err != nil || found {
		return accountRecord, false, err
	}

	groupID := GCfgBaseGroupID
	startAID := common.GroupAIDStart(groupID)
	sequenceKey := RedisKeyAccountAIDSequence(groupID)
	if err = p.client.SetNX(ctx, sequenceKey, startAID-1, 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "init aid sequence failed, account: %s %v", account, xruntime.Location())
	}
	aid, err := p.client.Incr(ctx, sequenceKey).Uint64()
	if err != nil {
		return nil, false, errors.WithMessagef(err, "incr aid sequence failed, account: %s %v", account, xruntime.Location())
	}

	now := time.Now().UnixMilli()
	characterRecords := make([]*pb.CharacterRecord, int(pb.AccountRecordLimit_AccountRecordLimit_MaxCharacterSlotCount))
	for i := range characterRecords {
		// 空槽必须保存为非 nil 的零 UUID 记录, 使槽位下标在所有服务间保持稳定.
		characterRecords[i] = &pb.CharacterRecord{}
	}
	accountRecord = &pb.AccountRecord{
		Aid:                   aid,
		Account:               account,
		CreateTimestampMs:     now,
		CharacterRecordList:   characterRecords,
		PetWarehouseRecordMap: make(map[uint64]*pb.PetRecord),
		ItemWarehouse: &pb.ItemContainerRecord{
			ItemCountMap:       make(map[uint32]uint64),
			EquipmentRecordMap: make(map[uint64]*pb.EquipmentRecord),
		},
	}
	if err = p.SetAccountRecord(ctx, aid, accountRecord); err != nil {
		return nil, false, err
	}
	if err = p.client.Set(ctx, RedisKeyAccountAID(account), strconv.FormatUint(aid, 10), 0).Err(); err != nil {
		return nil, false, errors.WithMessagef(err, "set account aid failed, account: %s aid: %d %v", account, aid, xruntime.Location())
	}
	return accountRecord, true, nil
}

// GetAccountRecordByAccount 通过 account 读取 aid 和 AccountRecord。
// 如果账号映射存在但 AccountRecord 缺失或关键字段不一致，直接返回错误。
func (p *Redis) GetAccountRecordByAccount(ctx context.Context, account string) (*pb.AccountRecord, bool, error) {
	aid, found, err := p.GetAccountAID(ctx, account)
	if err != nil || !found {
		return nil, found, err
	}
	accountRecord, err := p.GetAccountRecord(ctx, aid)
	if errors.Is(err, redis.Nil) {
		return nil, true, errors.Errorf("account record missing, account: %s aid: %d %v", account, aid, xruntime.Location())
	}
	if err != nil {
		return nil, true, err
	}
	if accountRecord == nil {
		return nil, true, errors.Errorf("account record is nil, account: %s aid: %d %v", account, aid, xruntime.Location())
	}
	if accountRecord.GetAid() != aid {
		return nil, true, errors.Errorf("account record aid mismatch, account: %s aid: %d record_aid: %d %v", account, aid, accountRecord.GetAid(), xruntime.Location())
	}
	if accountRecord.GetAccount() != account {
		return nil, true, errors.Errorf("account record account mismatch, account: %s aid: %d record_account: %s %v", account, aid, accountRecord.GetAccount(), xruntime.Location())
	}
	if accountRecord.GetCreateTimestampMs() == 0 {
		return nil, true, errors.Errorf("account record create time is empty, account: %s aid: %d %v", account, aid, xruntime.Location())
	}
	return accountRecord, true, nil
}

// GetAccountAID 读取 account 到 aid 的映射。
// found 为 true 但返回错误时，表示 Redis 中存在不可用的 aid 值。
func (p *Redis) GetAccountAID(ctx context.Context, account string) (uint64, bool, error) {
	value, err := p.client.Get(ctx, RedisKeyAccountAID(account)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, errors.WithMessagef(err, "get account aid failed, account: %s %v", account, xruntime.Location())
	}
	aid, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, true, errors.WithMessagef(err, "parse account aid failed, account: %s value: %s %v", account, value, xruntime.Location())
	}
	if aid == 0 {
		return 0, true, errors.Errorf("parse account aid failed, account: %s value: %s %v", account, value, xruntime.Location())
	}
	return aid, true, nil
}
