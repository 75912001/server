package main

import (
	"fmt"
	"math"

	pb "server/proto/pb"

	xutil "github.com/75912001/xlib/util"
)

const (
	characterLuckLSSecondsPerDay = int64(5400)
	characterLuckLSDaysPerYear   = int64(100)
	characterLuckLSEraSeconds    = int64(912771809)
)

type characterLuckLSBucket struct {
	year int64
	day  int64
}

// characterOnlineRecordBackup 保存角色上线写cache前被修改的字段, 供写入失败时完整恢复内存档案.
type characterOnlineRecordBackup struct {
	lastLoginTimestampMs int64
	luckState            *pb.CharacterLuckState
}

// characterLuckBucketAt 按8.5 RealTimeToLSTime把Unix毫秒时间转换为LS年和LS日.
func characterLuckBucketAt(timestampMs int64) (characterLuckLSBucket, error) {
	if timestampMs <= 0 {
		return characterLuckLSBucket{}, fmt.Errorf("timestamp must be positive")
	}
	lsSeconds := timestampMs/1000 - characterLuckLSEraSeconds
	if lsSeconds < 0 {
		return characterLuckLSBucket{}, fmt.Errorf("timestamp is before LS era")
	}
	lsDays := lsSeconds / characterLuckLSSecondsPerDay
	return characterLuckLSBucket{
		year: lsDays / characterLuckLSDaysPerYear,
		day:  lsDays % characterLuckLSDaysPerYear,
	}, nil
}

// validateCharacterLuckState 校验基础运气和刷新时间必须共同处于首次登录前或已刷新状态.
func validateCharacterLuckState(state *pb.CharacterLuckState) error {
	if state == nil {
		return fmt.Errorf("luck state is missing")
	}
	baseLuck := state.GetBaseLuck()
	lastRefreshTimestampMs := state.GetLastRefreshTimestampMs()
	if baseLuck == 0 {
		if lastRefreshTimestampMs != 0 {
			return fmt.Errorf("pending base luck has refresh timestamp")
		}
		return nil
	}
	if baseLuck > 5 {
		return fmt.Errorf("base luck %d is out of range", baseLuck)
	}
	if lastRefreshTimestampMs <= 0 {
		return fmt.Errorf("base luck %d has no refresh timestamp", baseLuck)
	}
	if _, err := characterLuckBucketAt(lastRefreshTimestampMs); err != nil {
		return fmt.Errorf("last refresh timestamp is invalid: %w", err)
	}
	return nil
}

// characterLuckNeedsRefresh 只在首次登录或当前LS年/日与上次刷新不同时返回true.
func characterLuckNeedsRefresh(nowMs int64, state *pb.CharacterLuckState) (bool, error) {
	if err := validateCharacterLuckState(state); err != nil {
		return false, err
	}
	nowBucket, err := characterLuckBucketAt(nowMs)
	if err != nil {
		return false, fmt.Errorf("resolve current LS bucket: %w", err)
	}
	if state.GetBaseLuck() == 0 {
		return true, nil
	}
	lastBucket, err := characterLuckBucketAt(state.GetLastRefreshTimestampMs())
	if err != nil {
		return false, fmt.Errorf("resolve last refresh LS bucket: %w", err)
	}
	return nowBucket != lastBucket, nil
}

// characterBaseLuckFromRoll 保留8.5 RAND(0,99)和40/30/20/7/3概率分段.
func characterBaseLuckFromRoll(roll uint32) (uint32, error) {
	if roll > 99 {
		return 0, fmt.Errorf("luck roll %d is out of range", roll)
	}
	switch {
	case roll >= 60:
		return 1, nil
	case roll >= 30:
		return 2, nil
	case roll >= 10:
		return 3, nil
	case roll >= 3:
		return 4, nil
	default:
		return 5, nil
	}
}

func randomCharacterLuckRoll() uint32 {
	return xutil.RandomU32(0, 99)
}

// prepareCharacterOnlineRecord 在所有本地校验完成后更新上线所需字段, 不执行cache或网络副作用.
// roll只在首次登录或跨LS日时调用, 保证同一LS日重复登录不额外消耗随机数.
func prepareCharacterOnlineRecord(record *pb.CharacterRecord, nowMs int64, roll func() uint32) (characterOnlineRecordBackup, error) {
	if record == nil {
		return characterOnlineRecordBackup{}, fmt.Errorf("character record is nil")
	}
	if roll == nil {
		return characterOnlineRecordBackup{}, fmt.Errorf("luck roll function is nil")
	}
	if record == nil || record.GetBase() == nil {
		return characterOnlineRecordBackup{}, fmt.Errorf("character base record is nil")
	}
	base := record.GetBase()
	needsRefresh, err := characterLuckNeedsRefresh(nowMs, base.GetLuckState())
	if err != nil {
		return characterOnlineRecordBackup{}, err
	}

	backup := characterOnlineRecordBackup{
		lastLoginTimestampMs: base.GetLastLoginTimestampMs(),
		luckState:            base.LuckState,
	}
	if needsRefresh {
		baseLuck, err := characterBaseLuckFromRoll(roll())
		if err != nil {
			return characterOnlineRecordBackup{}, err
		}
		base.LuckState = &pb.CharacterLuckState{
			BaseLuck:               baseLuck,
			LastRefreshTimestampMs: nowMs,
		}
	}
	base.LastLoginTimestampMs = nowMs
	return backup, nil
}

func (backup characterOnlineRecordBackup) restore(record *pb.CharacterRecord) {
	if record == nil {
		return
	}
	record.Base.LastLoginTimestampMs = backup.lastLoginTimestampMs
	record.Base.LuckState = backup.luckState
}

// newCombatLuckSnapshot 根据本次从已装备物品配置解析的修正值, 计算基础运气、装备合计和限制在1-5的有效运气.
func newCombatLuckSnapshot(baseLuck uint32, equipmentConfigModifierList []int32) (*pb.CombatLuckSnapshot, error) {
	if baseLuck < 1 || baseLuck > 5 {
		return nil, fmt.Errorf("base luck %d is out of range", baseLuck)
	}
	modifierSum := int64(0)
	for _, modifier := range equipmentConfigModifierList {
		modifierSum += int64(modifier)
		if modifierSum < math.MinInt32 || modifierSum > math.MaxInt32 {
			return nil, fmt.Errorf("equipment luck modifier sum %d is out of range", modifierSum)
		}
	}
	effectiveLuck := int64(baseLuck) + modifierSum
	if effectiveLuck < 1 {
		effectiveLuck = 1
	} else if effectiveLuck > 5 {
		effectiveLuck = 5
	}
	return &pb.CombatLuckSnapshot{
		BaseLuck:          int32(baseLuck),
		EquipmentModifier: int32(modifierSum),
		EffectiveLuck:     int32(effectiveLuck),
	}, nil
}
