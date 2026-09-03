package pet

import (
	"fmt"
	"time"

	"server/common/gameconfig"
	"server/proto/pb"
)

// CaptureSnapshot 保存敌人创建时的个体与出生技能, 捕获时不重抽四维, 也不重读可热更新的模板.
// savedBase 是原版 CHAR_ALLOCPOINT: 四项独立随机偏移后, 追加十点初始属性之前的值.
type CaptureSnapshot struct {
	assetID   uint32
	level     uint32
	exp       uint64
	grade     pb.PetGrade
	savedBase [4]int32
	raw       [4]int32
	skills    []uint32
}

// NewCaptureSnapshot 冻结可捕获敌人的持久化输入. 此时不分配账号 UUID, 不创建宠物档案.
func NewCaptureSnapshot(entry *gameconfig.PetEntry, level uint32, savedBase, raw [4]int32) (*CaptureSnapshot, error) {
	if entry == nil || entry.ID == nil || *entry.ID == 0 || !entry.SupportsOrdinaryCreation() {
		return nil, fmt.Errorf("capture pet template is invalid")
	}
	if level < uint32(pb.LevelRange_LevelRange_Min) || level > uint32(pb.LevelRange_LevelRange_Max) {
		return nil, fmt.Errorf("capture pet level is out of range: %d", level)
	}
	growth := entry.Growth
	if growth == nil || growth.BaseVital == nil || growth.BaseStr == nil || growth.BaseTough == nil || growth.BaseDex == nil {
		return nil, fmt.Errorf("capture pet growth is incomplete: %d", *entry.ID)
	}
	if len(entry.SkillSlots) != int(pb.PetSkillLimit_PetSkillLimit_MaxSlotCount) {
		return nil, fmt.Errorf("capture pet birth skills are incomplete: %d", *entry.ID)
	}
	base := [4]uint32{*growth.BaseVital, *growth.BaseStr, *growth.BaseTough, *growth.BaseDex}
	totalOffset := 0
	for index, value := range savedBase {
		offset := int64(value) - int64(base[index])
		if offset < -2 || offset > 2 {
			return nil, fmt.Errorf("capture pet saved base offset is invalid: pet:%d index:%d offset:%d", *entry.ID, index, offset)
		}
		totalOffset += int(offset)
	}
	if _, _, _, _, err := calculatePetPanelAttributes(raw[0], raw[1], raw[2], raw[3]); err != nil {
		return nil, fmt.Errorf("capture pet raw attributes are invalid: %w", err)
	}
	if gameconfig.GGameConfig == nil || gameconfig.GGameConfig.Exp == nil {
		return nil, fmt.Errorf("capture pet experience config is not loaded")
	}
	exp, err := gameconfig.GGameConfig.Exp.GetLevelMinExp(level)
	if err != nil {
		return nil, err
	}
	return &CaptureSnapshot{
		assetID: *entry.ID, level: level, exp: exp,
		grade:     petGradeFromRandomOffsetTotal(totalOffset),
		savedBase: savedBase, raw: raw,
		skills: append([]uint32(nil), entry.SkillSlots...),
	}, nil
}

// NewRecord 在账号容量检查通过后创建捕获档案. 等级和个体全部继承快照, 七槽技能沿用出生模板.
func (s *CaptureSnapshot) NewRecord(uuid uint64) (*pb.PetRecord, error) {
	if s == nil || s.assetID == 0 || uuid == 0 {
		return nil, fmt.Errorf("capture snapshot or pet uuid is empty")
	}
	record := &pb.PetRecord{
		Uuid: uuid, AssetId: s.assetID, Exp: s.exp, Grade: s.grade,
		CarryStatus: pb.PetCarryStatus_PetCarryStatus_Wait,
		Loyalty:     100, SkillIdList: append([]uint32(nil), s.skills...),
		SavedBaseVitality: s.savedBase[0], SavedBaseStrength: s.savedBase[1],
		SavedBaseToughness: s.savedBase[2], SavedBaseDexterity: s.savedBase[3],
		RawVitality: s.raw[0], RawStrength: s.raw[1], RawToughness: s.raw[2], RawDexterity: s.raw[3],
		CreateTimestampMs: time.Now().UnixMilli(),
	}
	if err := recordGrowthBaseline(record, s.level); err != nil {
		return nil, err
	}
	return record, nil
}
