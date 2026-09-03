package main

import (
	pb "server/proto/pb"

	xactor "github.com/75912001/xlib/actor"
	xtimer "github.com/75912001/xlib/timer"
)

// characterMgr 是 Account actor 内的角色运行态聚合.
// manager 不拥有 AccountRecord 数据, 角色单元只引用账号档案中的 CharacterRecord.
// 所有方法只能由所属 Account actor 调用, 因此角色单元之间不需要额外加锁.
type characterMgr struct {
	account    *Account
	characters map[uint64]*character
}

// character 保存单个角色的在线状态、交互开关、自动遇敌状态和 CombatRoom actor 消息入口.
// record 属于 Account.accountRecord, 其余字段只在当前账号会话内有效, 不写入 cache.
type character struct {
	account     *Account
	record      *pb.CharacterRecord
	online      bool
	sceneID     uint32
	teamEnabled bool
	duelEnabled bool

	autoEncounterEnabled bool
	autoEncounterTimer   *xtimer.Second
	combatRoom           *xactor.Actor[string]
}

// newCharacterMgr 基于 RPC 边界已校验的账号档案构建全部有效角色单元.
func newCharacterMgr(account *Account, record *pb.AccountRecord) *characterMgr {
	manager := &characterMgr{
		account:    account,
		characters: make(map[uint64]*character),
	}
	if record == nil {
		return manager
	}
	for _, characterRecord := range record.GetCharacterRecordList() {
		if characterRecord.GetBase().GetUuid() == 0 {
			continue
		}
		characterUUID := characterRecord.GetBase().GetUuid()
		manager.characters[characterUUID] = &character{
			account: account,
			record:  characterRecord,
		}
	}
	return manager
}

// find 返回指定 UUID 对应的角色单元, 无效或不存在时返回 nil.
func (m *characterMgr) find(characterUUID uint64) *character {
	if m == nil || characterUUID == 0 {
		return nil
	}
	return m.characters[characterUUID]
}

// clearRuntime 先清理全部在线角色的队伍关系和场景表现, 再释放自动遇敌 timer、CombatRoom actor 指针及在线状态.
func (m *characterMgr) clearRuntime() {
	if m == nil {
		return
	}
	for _, character := range m.characters {
		if character != nil && character.online && character.record != nil && character.record.GetBase() != nil {
			base := character.record.GetBase()
			key := sceneCharacterKey{aid: m.account.aid, characterUUID: base.GetUuid()}
			m.account.dischargeCharacterTeam(key)
			m.account.removeCharacterPresence(character.sceneID, key)
		}
		character.clearRuntime()
		character.online = false
	}
}
