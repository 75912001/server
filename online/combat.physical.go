package main

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"sort"

	"server/common/gameconfig"
	pb "server/proto/pb"

	xerror "github.com/75912001/xlib/error"
)

const (
	combatRandomIncrement                 = uint64(1442695040888963407)
	combatRandomUint32Denominator         = 1 << 32
	combatMaximumCounter                  = 5
	combatPlayerCounterFists              = 9
	combatEnemyAISkillSlotCount           = 7
	combatCounterDivisor          float32 = 0.08
)

type combatActionKind uint8

const (
	combatActionKindUnknown combatActionKind = iota
	combatActionKindAttack
	combatActionKindDefense
	combatActionKindStandby
	combatActionKindEscape
	combatActionKindContinuationAttack
	combatActionKindGuardBreak
)

// combatEffectKind只用于服务端结算器内部区分原子结果, 不进入线上协议.
// 协议出口会把它转换为CombatEffect.detail中的具体oneof分支.
type combatEffectKind uint8

const (
	combatEffectKindUnknown combatEffectKind = iota
	combatEffectKindDamage
	combatEffectKindGuard
	combatEffectKindDodge
	combatEffectKindKnockdown
	combatEffectKindKnockback
	combatEffectKindEscape
	combatEffectKindActionOnly
)

// combatHitResult保留旧结算器可组合的命中标记. 协议出口会归一化为
// CombatHitOutcome, critical和guarded, 避免客户端重复推导互斥关系.
type combatHitResult uint8

const (
	combatHitResultNormal combatHitResult = iota
	combatHitResultCritical
	combatHitResultMiss
	combatHitResultDodge
	combatHitResultGuard
)

// combatDamageDetail保存协议转换需要的伤害和命中结果.
type combatDamageDetail struct {
	DisplayedDamage uint32
	AppliedHpDamage uint32
	HpBefore        uint32
	HpAfter         uint32
	HitResultList   []combatHitResult
}

func (d *combatDamageDetail) GetDisplayedDamage() uint32 {
	if d == nil {
		return 0
	}
	return d.DisplayedDamage
}

func (d *combatDamageDetail) GetHitResultList() []combatHitResult {
	if d == nil {
		return nil
	}
	return d.HitResultList
}

func (d *combatDamageDetail) GetAppliedHpDamage() uint32 {
	if d == nil {
		return 0
	}
	return d.AppliedHpDamage
}

func (d *combatDamageDetail) GetHpBefore() uint32 {
	if d == nil {
		return 0
	}
	return d.HpBefore
}

func (d *combatDamageDetail) GetHpAfter() uint32 {
	if d == nil {
		return 0
	}
	return d.HpAfter
}

// combatAction是服务端根据skill.yaml解析出的回合行为, 不进入客户端协议和持久化数据.
type combatAction struct {
	unitKey     *pb.CombatUnitKey
	kind        combatActionKind
	skillID     uint32
	targetKey   *pb.CombatUnitKey
	actionValue int64
	comboMember bool

	// declaredTargetKey保存行动前状态处理尚未改写的声明目标.
	declaredTargetCaptured bool
	declaredTargetKey      *pb.CombatUnitKey
	// segmentCount由skill.yaml的continuationAttack.segmentCount锁定.
	segmentCount uint32
	// 连击开始执行后按普通攻击命令参与后续反击资格判断.
	counterCommandPromotedToAttack bool
}

func captureCombatActionDeclaredTarget(action *combatAction) {
	if action == nil || action.declaredTargetCaptured {
		return
	}
	action.declaredTargetCaptured = true
	action.declaredTargetKey = cloneCombatUnitKey(action.targetKey)
}

func combatActionDeclaredTargets(action *combatAction) []*pb.CombatUnitKey {
	if action == nil {
		return nil
	}
	captureCombatActionDeclaredTarget(action)
	if action.declaredTargetKey == nil || combatUnitKeyEmpty(action.declaredTargetKey) {
		return nil
	}
	return []*pb.CombatUnitKey{cloneCombatUnitKey(action.declaredTargetKey)}
}

func appendCombatActionOnlyStep(action *combatAction, events *[]*combatStepResult) {
	if action == nil || events == nil {
		return
	}
	step := &combatStepResult{
		EventKind:         combatStepKindAction,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: cloneCombatUnitKeyList([]*pb.CombatUnitKey{action.targetKey}),
	}
	*events = append(*events, step)
}

func (a *combatAction) isAttack() bool {
	return a != nil && a.kind == combatActionKindAttack
}

func (a *combatAction) isGuard() bool {
	return a != nil && a.kind == combatActionKindDefense
}

func (a *combatAction) isStandby() bool {
	return a != nil && a.kind == combatActionKindStandby
}

func (a *combatAction) isContinuationAttack() bool {
	return a != nil && a.kind == combatActionKindContinuationAttack
}

func (a *combatAction) isGuardBreak() bool {
	return a != nil && a.kind == combatActionKindGuardBreak
}

func (a *combatAction) usesMultiSegmentDamageDivision() bool {
	return a != nil && a.isContinuationAttack()
}

func (a *combatAction) canCounter() bool {
	if a == nil {
		return false
	}
	return a.isAttack() || (a.isContinuationAttack() && a.counterCommandPromotedToAttack)
}

func (a *combatAction) promoteSpecialAttackCommand() {
	if a != nil && a.isContinuationAttack() {
		a.counterCommandPromotedToAttack = true
	}
}

// combatRandom 实现每场独立的PCG32-XSH-RR 64/32 v1随机流.
//
// 8.5源码混用了两种不同的整数抽取方式, 当前实现必须分别保留:
//  1. rangeInt对应RAND(min,max), 把一次原始随机值按区间宽度缩放到闭区间;
//  2. modulo对应rand()%N, 直接对一次原始随机值取模.
//
// 两种方法都恰好调用一次nextUint32. 不使用拒绝采样, 这样命中、伤害、反击和后续动作
// 看到的随机抽取顺序才不会因为区间宽度改变. PCG32只替代8.5的底层rand()随机流,
// rangeInt和modulo仍分别保持原源码的映射语义.
// 上述rangeInt只适用于上下界已经是整数的RAND调用. 8.5宏直接接收浮点表达式时不会
// 先执行参数类型转换; 当前基础行动值因此由actionValueDeduction保留小数上限单独缩放.
type combatRandom struct {
	state     uint64
	increment uint64
	drawCount uint64
}

func newCombatRandom(seed uint64) *combatRandom {
	random := &combatRandom{increment: combatRandomIncrement}
	random.nextUint32()
	random.state += seed
	random.nextUint32()
	random.drawCount = 0
	return random
}

func newCombatRandomSeed() (uint64, error) {
	var buffer [8]byte
	if _, err := cryptorand.Read(buffer[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buffer[:]), nil
}

func (r *combatRandom) nextUint32() uint32 {
	oldState := r.state
	r.state = oldState*6364136223846793005 + r.increment
	xorShifted := uint32(((oldState >> 18) ^ oldState) >> 27)
	rotation := uint32(oldState >> 59)
	r.drawCount++
	return (xorShifted >> rotation) | (xorShifted << ((-rotation) & 31))
}

// combatScaleRandomUint32把一个PCG32原始值映射到[minValue,maxValue]闭区间.
//
// 8.5的RAND(min,max)等价于:
//
//	min + floor((max-min+1) * rand() / (RAND_MAX+1))
//
// PCG32的原始值覆盖[0,2^32-1], 因此分母固定为2^32. 这里使用96位乘积的
// 高位完成除以2^32, 避免浮点舍入改变区间边界. bits.Mul64同时允许AI权重总和等
// 合法区间超过2^32, 不会因为uint64乘法溢出而回到错误结果.
//
// minValue大于maxValue时沿用原rangeInt的容错语义并交换边界. 最后的无符号加法
// 用于安全覆盖跨越0的int64区间; 缩放后的最终结果保证不会越过交换后的上下界.
func combatScaleRandomUint32(randomValue uint32, minValue int64, maxValue int64) int64 {
	if maxValue < minValue {
		minValue, maxValue = maxValue, minValue
	}

	span := uint64(maxValue) - uint64(minValue)
	var offset uint64
	if span == ^uint64(0) {
		// 整个int64值域的区间宽度是2^64, 无法存入uint64. 此时缩放结果可直接化简为
		// randomValue*2^32, 仍然只使用本次PCG32原始值且不增加额外随机抽取.
		offset = uint64(randomValue) << 32
	} else {
		width := span + 1
		high, low := bits.Mul64(uint64(randomValue), width)
		// (high<<64 | low) / 2^32. 因randomValue只有32位, high至多32位,
		// 所以左移后的商可以完整保存在uint64中.
		offset = high<<32 | low>>32
	}
	return int64(uint64(minValue) + offset)
}

func (r *combatRandom) rangeInt(minValue int64, maxValue int64) int64 {
	if r == nil {
		panic("combat random is nil")
	}
	// 即使minValue等于maxValue也必须抽取一次. 8.5的RAND宏仍会调用rand(),
	// 跳过该抽取会让本场战斗之后的全部随机结果错位.
	return combatScaleRandomUint32(r.nextUint32(), minValue, maxValue)
}

// combatScaleActionValueDeduction使用一个PCG32原始值, 复现8.5基础行动值分支中
// RAND(0,work*0.3)的宏展开结果. 它只负责纯计算, 不自行读取随机流.
//
// 8.5的RAND不是接收两个int参数的函数, 而是直接展开调用实参的宏:
//
//	(x-1)+1+(int)((double)(y-(x-1))*rand()/(RAND_MAX+1.0))
//
// 当x=0且y=work*0.3时, y仍是C double; 源码不会先把0.3倍work截成整数.
// 因此扣减量必须在完整执行`(work*0.3+1)*raw/随机源取值总数`后才向零截断.
// 当前战斗底层随机源是PCG32, 原始值域为[0,2^32-1], 所以分母固定为2^32.
// 本分支的所有输入均非负, 最后的向零截断也就是8.5这里需要的向下取整.
//
// 这个差异会真实改变行动顺序. 例如work=21时, y=6.3; 足够大的原始随机值
// 可以让8.5宏得到扣减7. 如果先把y截成6再调用整数rangeInt, 扣减永远不可能超过6.
// 本函数故意保持为行动值专用实现; 道具、职业技能和其他带非零小数下界的RAND分支
// 尚未开放, 等D011对应输入和技能落地后再分别按原始宏表达式实现, 避免提前泛化出错.
func combatScaleActionValueDeduction(randomValue uint32, work int64) int64 {
	upperBound := float64(work) * 0.3
	scaled := (upperBound + 1) * float64(randomValue) / combatRandomUint32Denominator
	return int64(scaled)
}

// actionValueDeduction为一次基础行动值计算恰好消费一个PCG32原始值.
// 即使最终扣减为0也不能跳过本次抽取, 否则同场战斗后续的合击、命中、伤害和
// 反击随机序列都会整体前移. nil随机源属于CombatRoom初始化错误, 与rangeInt一致直接暴露.
func (r *combatRandom) actionValueDeduction(work int64) int64 {
	if r == nil {
		panic("combat random is nil")
	}
	return combatScaleActionValueDeduction(r.nextUint32(), work)
}

// combatScaleDamageRandomFloatBounds使用一个PCG32原始值复刻8.5 BATTLE_DamageCalc
// 中上下界含C double表达式的RAND宏. 它与整数rangeInt不能互换:
//
//	(x-1)+1+(int)((double)(y-(x-1))*rand()/(RAND_MAX+1.0))
//
// 8.5的RAND是宏, 不会像接收int参数的函数那样先转换x和y. 当伤害分段传入
// RAND(0,attack*D_16)或RAND(0,attack*D_8)时, 小数上限必须参与完整缩放;
// 当追加攻防传入RAND(power*0.3,power)时, 小数下限还会保留到两个RAND相减后.
// 当前底层随机源是PCG32, 所以仍用2^32作为分母. C宏中的(int)对有效战斗数值
// 向零截断, 这里先把缩放偏移转成int64再加回小数下界, 由调用点按源码时机完成
// 最终整数转换.
//
// 本函数故意不交换反向边界. 整数rangeInt的边界交换是现代兼容行为, 但8.5宏
// 遇到浮点反向边界时会直接使用负宽度; 擅自交换会再次改变原始公式. 当前A12-1A
// 只传入服务器生成的非负基础攻防值, 均满足合法边界.
func combatScaleDamageRandomFloatBounds(randomValue uint32, minimum float64, maximum float64) float64 {
	width := maximum - (minimum - 1)
	offset := int64(width * float64(randomValue) / combatRandomUint32Denominator)
	return minimum + float64(offset)
}

// damageRandomFloatBounds为BATTLE_DamageCalc的一次浮点边界RAND恰好消费一个
// PCG32原始值. 即使上下界都是0也不能省略, 因为_ADD_DEAMGEDEFC在当前追加
// 攻防值均为0时仍会连续调用两次RAND, 后续反击随机数必须保持原位置.
func (r *combatRandom) damageRandomFloatBounds(minimum float64, maximum float64) float64 {
	if r == nil {
		panic("combat random is nil")
	}
	return combatScaleDamageRandomFloatBounds(r.nextUint32(), minimum, maximum)
}

func (r *combatRandom) modulo(divisor uint32) uint32 {
	if divisor == 0 {
		panic("combat random modulo divisor is zero")
	}
	return r.nextUint32() % divisor
}

type combatUnitKind uint8

const (
	combatUnitKindUnknown combatUnitKind = iota
	combatUnitKindPlayer
	combatUnitKindPet
	combatUnitKindEnemy
)

func combatKind(unit *pb.CombatUnit) combatUnitKind {
	if unit == nil || unit.GetKey() == nil {
		return combatUnitKindUnknown
	}
	if combatUnitIsPlayerCharacter(unit) {
		return combatUnitKindPlayer
	}
	if unit.GetKey().GetAid() != 0 && unit.GetPetId() != 0 {
		return combatUnitKindPet
	}
	if unit.GetKey().GetAid() == 0 && unit.GetPetId() != 0 {
		return combatUnitKindEnemy
	}
	return combatUnitKindUnknown
}

func combatPetElementalPoints(entry *gameconfig.PetEntry) *pb.ElementalPoints {
	if entry == nil {
		return &pb.ElementalPoints{}
	}
	value := func(elemental pb.AssetElemental) uint32 {
		if entry.Elemental[elemental] == nil {
			return 0
		}
		return *entry.Elemental[elemental]
	}
	return &pb.ElementalPoints{
		Earth: value(pb.AssetElemental_AssetElemental_Earth),
		Water: value(pb.AssetElemental_AssetElemental_Water),
		Fire:  value(pb.AssetElemental_AssetElemental_Fire),
		Wind:  value(pb.AssetElemental_AssetElemental_Wind),
	}
}

func combatEffectiveLuck(state *combatUnitRuntimeState) int64 {
	if state == nil || combatKind(state.unit) != combatUnitKindPlayer {
		return 0
	}
	luck := int64(state.unit.GetAttribute().GetLuck().GetEffectiveLuck())
	if luck < 1 {
		return 1
	}
	if luck > 5 {
		return 5
	}
	return luck
}

func combatPosition(unit *pb.CombatUnit) *pb.CombatPosition {
	if unit == nil {
		return nil
	}
	return &pb.CombatPosition{Camp: unit.GetCamp(), Position: unit.GetPosition()}
}

func combatClampUint32(value uint64) uint32 {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value)
}

func combatClampDelta(value uint64) int32 {
	if value > math.MaxInt32 {
		return -math.MaxInt32
	}
	return -int32(value)
}

// combatActionValue按单位当前敏捷计算本回合行动值.
//
// 当前skill.yaml开放动作均使用8.5 BATTLE_DexCalc基础分支. RAND的0.3倍
// 上限必须保留小数缩放语义, 不能先转换成整数后调用rangeInt.
func (r *CombatRoom) combatActionValue(action *combatAction) int64 {
	state := r.stateByKey(action.unitKey)
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return 1
	}
	work := combatEffectiveAgilityPower(state) + 20
	deduction := r.random.actionValueDeduction(work)
	value := work - deduction
	if value < 1 {
		return 1
	}
	return value
}

func sortCombatActionsByValue(actions []*combatAction) {
	sort.SliceStable(actions, func(left int, right int) bool {
		return actions[left].actionValue > actions[right].actionValue
	})
}

// combatDodgeCoreThreshold计算基础物理闪避阈值.
func combatDodgeCoreThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) float32 {
	attackerDex := combatEffectiveAgilityPower(attacker)
	defenderDex := combatEffectiveAgilityPower(defender)
	attackerKind := combatKind(attacker.unit)
	defenderKind := combatKind(defender.unit)
	switch {
	case attackerKind == combatUnitKindEnemy && defenderKind == combatUnitKindPet:
		attackerDex = int64(float64(attackerDex) * 0.8)
	case attackerKind != combatUnitKindEnemy && defenderKind == combatUnitKindPet:
		defenderDex = int64(float64(defenderDex) * 0.8)
	case attackerKind != combatUnitKindPlayer && defenderKind == combatUnitKindPlayer:
		attackerDex = int64(float64(attackerDex) * 0.6)
	case attackerKind == combatUnitKindPlayer && defenderKind != combatUnitKindPlayer:
		defenderDex = int64(float64(defenderDex) * 0.6)
	}

	big := float32(attackerDex)
	small := float32(defenderDex)
	wari := float32(1)
	if defenderDex >= attackerDex {
		big = float32(defenderDex)
		small = float32(attackerDex)
	} else if big <= 0 {
		wari = 0
	} else {
		wari = small / big
	}
	work := (big - small) / float32(0.02)
	if work <= 0 {
		work = 0
	}
	percentage := float32(math.Sqrt(float64(work)))
	percentage *= wari
	percentage += float32(combatEffectiveLuck(defender))
	percentage *= 100
	if percentage > 7500 {
		percentage = 7500
	}
	if percentage <= 0 {
		percentage = 1
	}
	return percentage
}

func (r *CombatRoom) combatDodge(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) bool {
	if r == nil || r.random == nil || attacker == nil || defender == nil ||
		attacker.unit == nil || defender.unit == nil || defender.guard {
		return false
	}
	percentage := combatDodgeCoreThreshold(attacker, defender)
	if combatKind(attacker.unit) == combatUnitKindPlayer {
		minimumHit := int64(float32(attacker.hitModifier) * 0.8)
		maximumHit := int64(float32(attacker.hitModifier) * 1.2)
		percentage -= float32(r.random.rangeInt(minimumHit, maximumHit))
		if percentage < 0 {
			percentage = 0
		}
	}
	if float32(r.random.rangeInt(1, 10000)) <= percentage {
		return true
	}
	return defender.dodgeModifier > 0 && int64(r.random.modulo(100)) < defender.dodgeModifier
}

// combatCriticalThreshold复刻8.5 BATTLE_CriticalCheckPlayer当前一期PVE普通攻击会经过的
// 基础暴击阈值计算. 这里不能使用float64一次算完: 8.5中的Big、Small、wari、Work、per和
// divpara全部是C float, 每次赋值和复合运算都会舍入到单精度; 人物敏捷16攻击敏捷42的
// 敌人时, 正确阈值是939, float64路径会错误得到940, 已经足以改变固定随机数下的战果.
//
// 四种当前PVE关系严格保留8.5分支:
//   - 宠物攻击敌人: 敌方敏捷先乘0.8并按C int复合赋值向零截断;
//   - 敌人攻击宠物: 不开平方, 除数改为10;
//   - 非玩家单位攻击玩家: 不开平方, 除数改为10;
//   - 玩家攻击非玩家单位: 敌方敏捷先乘0.6并按C int复合赋值向零截断.
//
// attacker.criticalModifier对应8.5的装备暴击输入At_Soubi, 当前生产快照尚未接入装备来源,
// 因而实际值仍为0. 字段和0.5加成顺序保留在公式中, 等D017接入真实装备数据后无需改变
// 随机判定链. defender.ultimateKnockbackImmune同时映射源码在公式末尾对真实基础形象
// 101813/101814执行的`per=0`; 守护已经在调用本函数前把计算目标切换到实际守护宠物.
// 弓暴击例外和其他特殊宠物技能加成仍等待对应运行态接入.
func combatCriticalThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	attackerDex := combatEffectiveAgilityPower(attacker)
	defenderDex := combatEffectiveAgilityPower(defender)
	attackerKind := combatKind(attacker.unit)
	defenderKind := combatKind(defender.unit)
	root := true
	divisor := float32(0.09)
	switch {
	case attackerKind == combatUnitKindPet && defenderKind == combatUnitKindEnemy:
		// 8.5的Df_Dex是int, 右侧0.8是未带后缀的double常量. C的*=先按double
		// 完成乘法, 再赋回int并向零截断, 因此这里不能提前转成float32计算.
		defenderDex = int64(float64(defenderDex) * 0.8)
	case attackerKind == combatUnitKindEnemy && defenderKind == combatUnitKindPet:
		root = false
		divisor = 10
	case attackerKind != combatUnitKindPlayer && defenderKind == combatUnitKindPlayer:
		root = false
		divisor = 10
	case attackerKind == combatUnitKindPlayer && defenderKind != combatUnitKindPlayer:
		// 与上面的0.8分支相同, 这里模拟int Df_Dex *= 0.6的double计算和向零截断.
		defenderDex = int64(float64(defenderDex) * 0.6)
	}

	// 以下局部量必须逐步保持float32. 这不是性能优化, 而是8.5 C float舍入行为的一部分.
	big := float32(defenderDex)
	small := float32(attackerDex)
	wari := float32(0)
	if attackerDex >= defenderDex {
		big = float32(attackerDex)
		small = float32(defenderDex)
		wari = 1
	} else if big > 0 {
		wari = small / big
	}
	work := (big - small) / divisor
	if work <= 0 {
		work = 0
	}

	// 8.5表达式中的0.5是double常量, 会先把Work或sqrt结果提升为double, 完成
	// 装备加成后再赋回float per. 即使当前装备修正为0, 也保留这次精度边界.
	percentageBase := work
	if root {
		percentageBase = float32(math.Sqrt(float64(work)))
	}
	percentage := float32(float64(percentageBase) + float64(attacker.criticalModifier)*0.5)
	percentage *= wari
	percentage += float32(combatEffectiveLuck(attacker))
	percentage *= 100
	// 8.5只把负数改为1, per==0必须保持0; 上限则固定截到10000.
	if percentage < 0 {
		percentage = 1
	}
	if percentage > 10000 {
		percentage = 10000
	}
	if defender.ultimateKnockbackImmune {
		// 8.5先完整计算并封顶per, 最后才按雷尔真实基础形象强制归零.
		// 这里保留同一阶段; 外层combatCritical仍必须消费一次RAND(1,10000).
		percentage = 0
	}
	return int64(percentage)
}

func (r *CombatRoom) combatCritical(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) bool {
	return r.random.rangeInt(1, 10000) < combatCriticalThreshold(attacker, defender)
}

const (
	// 8.5的BATTLE_GetAttr按“地、水、火、风、无”保存五项属性. 下方常量只负责
	// 固定这个内部数组顺序, 避免BATTLE_AttrCalc按“火、水、地、风、无”读取参数时
	// 再次散落0、1、2、3、4等无业务语义的下标.
	combatElementEarth = iota
	combatElementWater
	combatElementFire
	combatElementWind
	combatElementNone
	combatElementCount
)

// combatElementArray把当前项目的0至10元素点转换成8.5 BATTLE_GetAttr使用的
// 0至100属性值. 当前角色创建和pet.yaml都要求四项点数总和为10, 所以正常PVE
// 单位的无属性值为0; nil或全0快照仍按8.5的ATTR_MAX补成100点无属性, 供房间
// 内部边界处理和对照测试使用.
//
// 8.5会先把每个负属性钳到0, 再用max(100-sum, 0)计算无属性. 当前PB字段为
// uint32, 不可能表达负数; 四项超过100后的无属性钳0逻辑仍保留, 以免运行期
// 快照边界与源码分叉. 攻击转属性技能会在combatEffectiveElementArray中按8.5
// 顺序覆盖本数组; 套装属性和BATTLE_PROPERTY回调尚未接入, 它们以后必须在
// 进入combatElementMatrixDamage前按各自源码顺序修改这五项值.
func combatElementArray(points *pb.ElementalPoints) [combatElementCount]int64 {
	if points == nil {
		return [combatElementCount]int64{0, 0, 0, 0, 100}
	}
	earth := int64(points.GetEarth()) * 10
	water := int64(points.GetWater()) * 10
	fire := int64(points.GetFire()) * 10
	wind := int64(points.GetWind()) * 10
	none := int64(100) - earth - water - fire - wind
	if none < 0 {
		none = 0
	}
	return [combatElementCount]int64{earth, water, fire, wind, none}
}

// combatEffectiveElementArray返回单位开战快照中的五属性.
func combatEffectiveElementArray(state *combatUnitRuntimeState) [combatElementCount]int64 {
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return [combatElementCount]int64{0, 0, 0, 0, 100}
	}
	return combatElementArray(state.unit.GetAttribute().GetElemental())
}

// combatElementMatrixDamage逐表达式复刻8.5 battle_event.c的BATTLE_AttrAdjust和
// BATTLE_AttrCalc. 参数数组顺序固定为“地、水、火、风、无”, damage是元素修正前
// 已经向零截断的物理伤害. 本函数不读取房间状态、不消费随机数. 它只负责实际
// 伤害结算; 8.5敌方elementalSubdue选目标使用独立的GetSubdueAttribute决策树,
// 不调用本矩阵, 也不比较候选目标的最终伤害.
//
// 类型转换顺序是本函数最重要的约束. 8.5的五项属性和damage都是C int, 但
// AJ_UP=1.5、AJ_SAME=1.0、AJ_DOWN=0.6和D_ATTR=1/(100*100)都是double:
//
//  1. BATTLE_AttrAdjust先对攻方五项分别执行At_pow[i] *= damage.
//  2. BATTLE_AttrCalc的每个“攻方属性*守方属性”先完成整数乘法, 遇到AJ常量后
//     才提升为double; 火、水、地、风、无五条完整加法表达式分别赋回int,
//     所以每条表达式都必须独立向零截断一次.
//  3. 五个已经截断的int相加得到iRet, 再乘double D_ATTR, 最后由int返回值
//     再向零截断一次. 不能把五条double表达式先相加后只截断一次.
//
// 旧实现把每个整数乘积提前转成float32, 并以float32执行AJ和D_ATTR. 例如纯火
// 攻纯风、damage=17902时, 8.5得到26853; float32路径会把中间值舍入后得到
// 26852. 本实现使用float64承载C double运算. 整数乘积使用int64, 因为C有符号
// int溢出属于未定义行为, 不应在Go中伪造某一种溢出结果; 当前0至100元素范围和
// 正常PVE伤害均在8.5有定义的整数乘法范围内.
func combatElementMatrixDamage(attackerElement [combatElementCount]int64, defenderElement [combatElementCount]int64, damage int64) int64 {
	for index := range attackerElement {
		attackerElement[index] *= damage
	}

	// 下列五条表达式故意保持与BATTLE_AttrCalc相同的项顺序. 即使数学上可以
	// 合并公因子, 重排浮点加法也可能改变靠近整数边界时的double结果和截断值.
	myFire := int64(
		float64(attackerElement[combatElementFire]*defenderElement[combatElementNone])*1.5 +
			float64(attackerElement[combatElementFire]*defenderElement[combatElementFire])*1.0 +
			float64(attackerElement[combatElementFire]*defenderElement[combatElementWater])*0.6 +
			float64(attackerElement[combatElementFire]*defenderElement[combatElementEarth])*1.0 +
			float64(attackerElement[combatElementFire]*defenderElement[combatElementWind])*1.5,
	)
	myWater := int64(
		float64(attackerElement[combatElementWater]*defenderElement[combatElementNone])*1.5 +
			float64(attackerElement[combatElementWater]*defenderElement[combatElementFire])*1.5 +
			float64(attackerElement[combatElementWater]*defenderElement[combatElementWater])*1.0 +
			float64(attackerElement[combatElementWater]*defenderElement[combatElementEarth])*0.6 +
			float64(attackerElement[combatElementWater]*defenderElement[combatElementWind])*1.0,
	)
	myEarth := int64(
		float64(attackerElement[combatElementEarth]*defenderElement[combatElementNone])*1.5 +
			float64(attackerElement[combatElementEarth]*defenderElement[combatElementFire])*1.0 +
			float64(attackerElement[combatElementEarth]*defenderElement[combatElementWater])*1.5 +
			float64(attackerElement[combatElementEarth]*defenderElement[combatElementEarth])*1.0 +
			float64(attackerElement[combatElementEarth]*defenderElement[combatElementWind])*0.6,
	)
	myWind := int64(
		float64(attackerElement[combatElementWind]*defenderElement[combatElementNone])*1.5 +
			float64(attackerElement[combatElementWind]*defenderElement[combatElementFire])*0.6 +
			float64(attackerElement[combatElementWind]*defenderElement[combatElementWater])*1.0 +
			float64(attackerElement[combatElementWind]*defenderElement[combatElementEarth])*1.5 +
			float64(attackerElement[combatElementWind]*defenderElement[combatElementWind])*1.0,
	)
	myNone := int64(
		float64(attackerElement[combatElementNone]*defenderElement[combatElementNone])*1.0 +
			float64(attackerElement[combatElementNone]*defenderElement[combatElementFire])*0.6 +
			float64(attackerElement[combatElementNone]*defenderElement[combatElementWater])*0.6 +
			float64(attackerElement[combatElementNone]*defenderElement[combatElementEarth])*0.6 +
			float64(attackerElement[combatElementNone]*defenderElement[combatElementWind])*0.6,
	)

	iRet := myFire + myWater + myEarth + myWind + myNone
	return int64(float64(iRet) * (1.0 / (100 * 100)))
}

// combatElementAdjustedDamage取得双方元素并执行8.5属性矩阵.
func (r *CombatRoom) combatElementAdjustedDamage(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, damage int64) int64 {
	attackerElement := combatEffectiveElementArray(attacker)
	defenderElement := combatEffectiveElementArray(defender)
	return combatElementMatrixDamage(attackerElement, defenderElement, damage)
}

// combatEffectiveAttackPower返回单位开战快照中的攻击力.
func combatEffectiveAttackPower(state *combatUnitRuntimeState) int64 {
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return 0
	}
	return int64(state.unit.GetAttribute().GetAttack())
}

// combatEffectiveDefensePower返回单位开战快照中的防御力.
func combatEffectiveDefensePower(state *combatUnitRuntimeState) int64 {
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return 0
	}
	return int64(state.unit.GetAttribute().GetDefense())
}

// combatEffectiveAgilityPower返回单位开战快照中的敏捷.
func combatEffectiveAgilityPower(state *combatUnitRuntimeState) int64 {
	if state == nil || state.unit == nil || state.unit.GetAttribute() == nil {
		return 0
	}
	return int64(state.unit.GetAttribute().GetAgility())
}

// combatBaseDefensePower复刻_BATTLE_NEWPOWER无骑乘分支的
// `defense = CHAR_WORKDEFENCEPOWER * 0.70`. 8.5右侧先把int防御提升为double,
// 与double常量0.70相乘, 最后赋回C float. 该顺序在很小的正常属性上已经可见:
// 防御9应舍入为float32表示的6.3; 如果先把0.70降为float32再乘, 会得到6.2999997.
// 独立成纯函数是为了让固定向量直接约束生产使用的精度边界, 不承载骑乘或状态逻辑.
func combatBaseDefensePower(defense int64) float32 {
	return float32(float64(defense) * 0.70)
}

// combatBaseDamageWithDefenseMode保留普通伤害随机链.
func (r *CombatRoom) combatBaseDamageWithDefenseMode(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, _ bool) int64 {
	attack := float32(combatEffectiveAttackPower(attacker))
	// 8.5右侧是int WORKDEFENCEPOWER乘double 0.70, 整个乘积最后才赋给float.
	// 先把防御转成float32再乘会把0.70也降成float32, 舍入层级与C源码不同.
	defense := combatBaseDefensePower(combatEffectiveDefensePower(defender))
	if combatKind(defender.unit) == combatUnitKindEnemy {
		// defense和后续运算量都是C float; +2位于乘法之后, 不能改写括号.
		defense += (defense*float32(r.random.modulo(10)) + 2) / 100
	}
	if combatKind(attacker.unit) == combatUnitKindEnemy {
		// 敌方攻击浮动只在攻方确实是enemy时抽取, 与守方浮动使用独立原始随机值.
		attack += (attack*float32(r.random.modulo(10)) + 2) / 100
	}

	damage := int64(0)
	switch {
	case defense <= attack && float64(attack) < float64(defense)*8.0/7.0:
		// 源码的D_16是double 1.0/16. attack会先提升为double, 小数上限参与
		// RAND宏的完整缩放, 直到宏内部(int)才向零截断.
		damage = int64(r.random.damageRandomFloatBounds(0, float64(attack)*(1.0/16)))
	case defense > attack:
		// 这一段两个实参都是整数常量, 继续使用整数rangeInt即可精确对应RAND(0,1).
		damage = r.random.rangeInt(0, 1)
	case attack >= defense*8/7:
		// 第三个条件故意保留源码的整数常量8/7: defense*8和随后/7均按C float
		// 运算. RAND上限和减去attack*D_16则使用double, 结果赋给float K0.
		randomDamage := r.random.damageRandomFloatBounds(0, float64(attack)*(1.0/8))
		k0 := float32(randomDamage - float64(attack)*(1.0/16))
		attackDefenseDifference := attack - defense
		// attack-defense先产生float结果, 再因double DAMAGE_RATE提升为double;
		// 加上提升后的float K0后才执行源码(int)向零截断.
		damage = int64(float64(attackDefenseDifference)*2.0 + float64(k0))
	}
	damage = r.combatElementAdjustedDamage(attacker, defender, damage)

	// _ADD_DEAMGEDEFC的apower和dpower是int, 乘0.3后分别成为double下界.
	// 两个RAND结果仍带各自的小数下界, 源码在相减后赋给int otherpower时才截断;
	// 不能先把两个下界或两个随机结果分别转成整数.
	additionalDamagePower := attacker.otherDamageModifier()
	additionalDefensePower := defender.otherDefenseModifier()
	additionalDamage := r.random.damageRandomFloatBounds(float64(additionalDamagePower)*0.3, float64(additionalDamagePower))
	additionalDefense := r.random.damageRandomFloatBounds(float64(additionalDefensePower)*0.3, float64(additionalDefensePower))
	otherPower := int64(additionalDamage - additionalDefense)
	if otherPower != 0 {
		damage += otherPower
	}
	if damage < 0 {
		return 0
	}
	return damage
}

func (s *combatUnitRuntimeState) otherDamageModifier() int64 {
	return 0
}

func (s *combatUnitRuntimeState) otherDefenseModifier() int64 {
	return 0
}

type combatAttackRoll struct {
	dodged        bool
	critical      bool
	guardBypassed bool
	damage        uint64
}

// combatGuardReductionActive判断目标本次受击是否执行防御减伤.
func combatGuardReductionActive(defender *combatUnitRuntimeState) bool {
	return defender != nil && defender.guard
}

// combatGuardDamageByRoll逐分支复刻8.5 battle_event.c的BATTLE_GuardAdjust.
// damage是普通/暴击基础伤害及当前技能专用倍率都已经结算后的C int等价值;
// guardRoll必须来自一次scaled RAND(1,100). 六档闭区间和倍率固定为:
//
//	[1,25]   -> 0.00
//	[26,50]  -> 0.10
//	[51,70]  -> 0.20
//	[71,85]  -> 0.30
//	[86,95]  -> 0.40
//	[96,100] -> 0.50
//
// 8.5中的damage是int, 0.00至0.50是没有f后缀的double常量. `damage *= 倍率`
// 会先把damage提升为double完成乘法, 再赋回int并向零截断. 因此这里必须把
// damage提升为float64, 不能像旧实现那样先降为float32. 例如damage=3495263、
// guardRoll=71时, 8.5得到1048578; 旧float32路径会舍入成1048579.
//
// 本函数只表达确定性的倍率计算, 不消费随机数, 也不负责伤害小于1后的
// RAND(0,1). 这样测试可以直接锁定全部概率边界, 而生产随机调用仍集中在房间方法中.
func combatGuardDamageByRoll(damage int64, guardRoll int64) int64 {
	multiplier := 0.50
	switch {
	case guardRoll <= 25:
		multiplier = 0.00
	case guardRoll <= 50:
		multiplier = 0.10
	case guardRoll <= 70:
		multiplier = 0.20
	case guardRoll <= 85:
		multiplier = 0.30
	case guardRoll <= 95:
		multiplier = 0.40
	}
	return int64(float64(damage) * multiplier)
}

// combatGuardAdjustedDamage对应一次完整的BATTLE_GuardAdjust调用. 即使传入伤害
// 已经是0, 8.5也会先消费一次RAND(1,100), 再根据结果乘0至0.5; 不能用damage==0
// 提前返回, 否则后续最低伤害、反击和下一单位动作的随机序列都会整体前移.
//
// 调用方必须先通过combatGuardReductionActive确认守方原始命令为Guard且当前
// 混乱值为0. 本函数只执行已经确定要发生的一次六档抽取, 不重复读取运行态,
// 防止调用者误把“混乱中的原始Guard命令”当成有效防御.
func (r *CombatRoom) combatGuardAdjustedDamage(damage int64) int64 {
	guardRoll := r.random.rangeInt(1, 100)
	return combatGuardDamageByRoll(damage, guardRoll)
}

// combatCounterAdjustedDamage逐语句复刻8.5 battle_event.c中BATTLE_Counter
// 调用BATTLE_AttackSeq之后的反击伤害缩放. 传入damage已经完成普通攻击的闪避、
// 暴击、基础伤害、元素、Guard及最低伤害RAND(0,1); 本函数只负责反击专属的
// 0.75倍率和正伤害最低1, 不得再次执行基础伤害或消费随机数.
//
// 8.5中的damage是C int, 0.75是没有f后缀的double常量. `damage *= 0.75`
// 会先把damage提升为double完成乘法, 再赋回int并向零截断. 所以这里必须先把
// damage转为float64; 不能沿用旧实现的float32, 否则较大伤害会在倍率计算前
// 丢失整数精度. 例如damage=16777219时, 8.5得到12582914, 旧float32路径
// 会错误得到12582915.
//
// 源码先把非正伤害归零; 只有原伤害大于0才执行倍率, 并在缩放结果小于1时
// 修正为1. 这保证原伤害0仍为MISS, 而原伤害1不会因0.75截断成0.
func combatCounterAdjustedDamage(damage int64) int64 {
	if damage <= 0 {
		return 0
	}
	damage = int64(float64(damage) * 0.75)
	if damage < 1 {
		return 1
	}
	return damage
}

// combatContinuationAttackAdjustedDamage复刻BATTLE_Attack中gDamageDiv的连续攻击分摊.
//
// 8.5的damage是C int, gDamageDiv是C float. `damage /= gDamageDiv`会先把damage
// 转成float完成除法, 再赋回int并向零截断. 因此这里必须显式使用float32, 不能用
// Go整数除法或float64替代; 大于2^24的整数会真实体现C float舍入差异.
//
// 源码只在原伤害大于0且除数非0时进入该分支. 分摊后小于等于0的正伤害强制为1;
// 原伤害0仍保持0, 继续表现为MISS. 除数等于计划总段数: 当前ContinuationAttack
// 和WildViolentAttack不会超过10段, AttackShoot自己的check()允许1至60段.
// divisor==0只作为内部防护保留原伤害, 不能把不同处理器的上限误写成共同上限.
func combatContinuationAttackAdjustedDamage(damage uint64, divisor uint32) uint64 {
	if damage == 0 || divisor == 0 {
		return damage
	}
	adjusted := int64(float32(damage) / float32(divisor))
	if adjusted <= 0 {
		return 1
	}
	return uint64(adjusted)
}

// combatAttackRoll执行一次基础物理随机和伤害计算.
func (r *CombatRoom) combatAttackRoll(
	attacker *combatUnitRuntimeState,
	defender *combatUnitRuntimeState,
	skipDodge bool,
	counter bool,
	suppressGuardResult bool,
) combatAttackRoll {
	if !skipDodge && r.combatDodge(attacker, defender) {
		return combatAttackRoll{dodged: true}
	}
	critical := r.combatCritical(attacker, defender)
	damage := r.combatBaseDamageWithDefenseMode(attacker, defender, false)
	if critical {
		attackerLevel := maxUint64(1, uint64(attacker.unit.GetAttribute().GetLevel()))
		defenderLevel := maxUint64(1, uint64(defender.unit.GetAttribute().GetLevel()))
		criticalAddition := int64(float32(combatEffectiveDefensePower(defender)) * float32(attackerLevel) / float32(defenderLevel) * 0.5)
		damage += criticalAddition
	}
	guardReductionActive := combatGuardReductionActive(defender)
	if guardReductionActive {
		damage = r.combatGuardAdjustedDamage(damage)
	}
	if damage < 1 {
		damage = r.random.rangeInt(0, 1)
	}
	if counter {
		damage = combatCounterAdjustedDamage(damage)
	}
	return combatAttackRoll{
		critical:      critical,
		guardBypassed: defender.guard && (suppressGuardResult || !guardReductionActive),
		damage:        uint64(damage),
	}
}

// combatCounterBase逐阶段复刻8.5 battle_event.c的BATTLE_CounterCalc.
// attacker不是最初发动普通攻击的单位, 而是当前正在接受“能否反击”判定的候选反击者;
// defender是这次反击准备攻击的原攻击者. 两者方向不能交换, 否则PVE单位类型缩放相反.
//
// 8.5的At_Dex、Df_Dex和Work是C int, Big、Small、wari、divpara和per是C float,
// 但BATTLE_CounterCalc的返回类型仍是int. 因此计算顺序必须严格保留为:
//
//  1. 先按双方单位类型修正整数敏捷或选择除数/是否开平方;
//  2. 把整数敏捷赋给float Big和Small, 再用float计算wari和Work表达式;
//  3. Work赋回int时向零截断;
//  4. sqrt使用C double, 结果显式转回float, 再以float乘wari;
//  5. 最终per从函数返回时再次向零截断为int.
//
// 旧实现返回float64并保留第5步的小数, 会系统性抬高反击阈值. 例如玩家敏捷10、
// 敌人敏捷15时, 敌人敏捷先按0.6变为9, Work为12, sqrt后约3.464, 8.5最终
// 返回3; 旧实现把3.464继续带入玩家反击率.
func combatCounterBase(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) int64 {
	attackerDex := combatEffectiveAgilityPower(attacker)
	defenderDex := combatEffectiveAgilityPower(defender)
	attackerKind := combatKind(attacker.unit)
	defenderKind := combatKind(defender.unit)
	root := true
	divisor := combatCounterDivisor
	switch {
	case attackerKind == combatUnitKindEnemy && defenderKind == combatUnitKindPet:
		root = false
		divisor = float32(10)
	case attackerKind == combatUnitKindPet && defenderKind == combatUnitKindEnemy:
		// Df_Dex是C int, 0.8是没有f后缀的double. compound assignment先以
		// double乘法计算, 再赋回int向零截断; 不能先把敏捷降为float32.
		defenderDex = int64(float64(defenderDex) * 0.8)
	case attackerKind != combatUnitKindPlayer && defenderKind == combatUnitKindPlayer:
		root = false
		divisor = float32(10)
	case attackerKind == combatUnitKindPlayer && defenderKind != combatUnitKindPlayer:
		// 与宠物攻敌分支相同, 这里的0.6也是C double后赋回int.
		defenderDex = int64(float64(defenderDex) * 0.6)
	}

	big := float32(defenderDex)
	small := float32(attackerDex)
	wari := float32(0)
	if attackerDex >= defenderDex {
		big = float32(attackerDex)
		small = float32(defenderDex)
		wari = float32(1)
	} else if big > 0 {
		wari = small / big
	}
	work := int64((big - small) / divisor)
	if work <= 0 {
		work = 0
	}
	percentage := float32(work)
	if root {
		percentage = float32(math.Sqrt(float64(work)))
	}
	percentage *= wari
	return int64(percentage)
}

// combatCounterThreshold对应BATTLE_CounterCheckPlayer和BATTLE_CounterCheckPet.
// 返回的inclusive说明最终随机比较是否包含相等边界:
// 玩家使用RAND < threshold, 玩家宠物和敌人使用RAND <= threshold.
//
// 当前生产战斗快照没有武器类型, 所以玩家双方都按8.5“未装备武器”分支映射为
// ITEM_FIST, CounterTbl的拳对拳系数固定为9. 投掷武器禁止反击和其他武器相性
// 必须等装备类型进入运行态后开发, 不能根据技能ID或equipment_id猜测武器类型.
//
// _SUIT_ADDENDUM在8.5 version.h中启用, 因此玩家阈值保留counterModifier输入.
func combatCounterThreshold(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) (threshold float32, inclusive bool) {
	base := combatCounterBase(attacker, defender)
	if combatKind(attacker.unit) == combatUnitKindPlayer {
		percentage := float32(float64(base*combatPlayerCounterFists)*0.1 + float64(combatEffectiveLuck(attacker)) + float64(attacker.counterModifier))
		threshold = percentage * float32(100)
		if threshold <= 0 {
			threshold = 1
		}
		return threshold, false
	}
	percentage := float32(base)
	if percentage > 100 {
		percentage = 100
	}
	threshold = percentage * float32(100)
	if threshold <= 0 {
		threshold = 1
	}
	return threshold, true
}

// combatCounterCheck只负责BATTLE_CounterCheck最后一次RAND(1,10000)及比较.
// 阈值即使被修正为1也必须消费这一抽. 玩家严格小于意味着threshold=1永远失败;
// 非玩家包含相等意味着随机值1可以成功. 这里把随机结果转为float32, 对应C中
// RAND返回int后与float per比较时的类型提升; 1至10000均可由float32精确表示.
func (r *CombatRoom) combatCounterCheck(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState) bool {
	threshold, inclusive := combatCounterThreshold(attacker, defender)
	randomValue := float32(r.random.rangeInt(1, 10000))
	if inclusive {
		return randomValue <= threshold
	}
	return randomValue < threshold
}

type combatDamageApplication struct {
	hpBefore      uint64
	hpAfter       uint64
	appliedDamage uint64
	killed        bool
	knockback     pb.CombatKnockbackType
}

func (r *CombatRoom) applyCombatDamage(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, damage uint64, critical bool) combatDamageApplication {
	return r.applyCombatDamageWithSingleHitBasis(attacker, defender, damage, damage, critical)
}

// combatUltimateThreshold返回8.5 DamageSub系列共同使用的Ultimate伤害阈值.
//
// 旧源码的maxhp是C int, 表达式中的1.2是double常量, 所以乘法和加20都在
// double中完成, 最后直接与整数damage或CHAR_WORKULTIMATE比较. 当前生产HP仍在
// uint32范围内, 转float64可以精确覆盖旧输入域, 并保留非整数阈值的比较边界.
func combatUltimateThreshold(maxHP uint64) float64 {
	return float64(maxHP)*1.2 + 20
}

// applyCombatUltimateTail复刻BATTLE_DamageSub对CHAR_WORKULTIMATE的处理顺序.
//
// singleHitBasis是源码局部damage. overkillDamage是本次HP扣到0以下的绝对值,
// 即源码addpoint.
//
// 顺序中的三个历史细节不能简化:
//   - 单次伤害先判断. 达标后本次越界量不加入累计值;
//   - 只有未达到单次阈值且本次确有越界量时, 才累加并检查累计阈值;
//   - 雷尔排除发生在上述记账之后. 被排除的Ultimate既不返回击飞类型, 也不
//     清空累计值; 非雷尔只有公共尾部真正返回类型1或2时才清零.
//
// 暴击死亡50%和ABIO无生物强制类型1发生在DamageSub返回之后, 由调用方处理;
// 那两种覆盖本身不会清空CHAR_WORKULTIMATE, 因而不能合并到本函数的清零分支.
func applyCombatUltimateTail(defender *combatUnitRuntimeState, singleHitBasis uint64, overkillDamage uint64) pb.CombatKnockbackType {
	if defender == nil {
		return pb.CombatKnockbackType_CombatKnockbackType_Unknown
	}

	threshold := combatUltimateThreshold(defender.maxHP)
	knockback := pb.CombatKnockbackType_CombatKnockbackType_Unknown
	if float64(singleHitBasis) >= threshold {
		knockback = pb.CombatKnockbackType_CombatKnockbackType_SingleHitOverkill
	} else if overkillDamage > 0 {
		defender.overkillDamage += overkillDamage
		if float64(defender.overkillDamage) >= threshold {
			knockback = pb.CombatKnockbackType_CombatKnockbackType_AccumulatedOverkill
		}
	}

	if knockback == pb.CombatKnockbackType_CombatKnockbackType_Unknown ||
		defender.ultimateKnockbackImmune {
		return pb.CombatKnockbackType_CombatKnockbackType_Unknown
	}
	defender.overkillDamage = 0
	return knockback
}

// applyCombatDamageWithSingleHitBasis应用DamageSub实际扣血、公共Ultimate尾部和
// 调用方死亡后的附加Ultimate覆盖.
//
// 普通攻击的damage与singleHitBasis相同. 针刺外皮是当前唯一不同的可达路径:
// 攻击者实际只承受补偶伤害的一半, 但旧BATTLE_DamageSub在公共尾部判断攻击者
// 单次Ultimate时仍使用补偶后的完整damage; 累计越界量只来自实际一半伤害.
//
// 公共尾部返回后, 8.5攻击调用点还会按以下顺序覆盖结果:
//  1. ABIO无生物死亡强制类型1, 并跳过下一项随机;
//  2. 否则非玩家目标被暴击击倒时必定消费一次RAND(1,100), 严格小于50才
//     把已有类型覆盖为1;
//  3. 最后雷尔排除把结果清为0.
//
// 第1、2项只改调用方局部ultimate, 不清CHAR_WORKULTIMATE. 公共尾部已经触发
// 的类型则已在applyCombatUltimateTail中清零, 所以这里不能根据最终knockback
// 再统一清零, 否则会破坏暴击死亡和无生物死亡的历史累计值.
func (r *CombatRoom) applyCombatDamageWithSingleHitBasis(attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, damage uint64, singleHitBasis uint64, critical bool) combatDamageApplication {
	application := combatDamageApplication{hpBefore: defender.hp, hpAfter: defender.hp}
	overkillDamage := uint64(0)
	if damage > 0 {
		application.appliedDamage = damage
		if application.appliedDamage > defender.hp {
			application.appliedDamage = defender.hp
		}
		defender.hp -= application.appliedDamage
		application.hpAfter = defender.hp
		if damage > application.hpBefore {
			overkillDamage = damage - application.hpBefore
		}
		application.knockback = applyCombatUltimateTail(defender, singleHitBasis, overkillDamage)
	}
	if defender.hp == 0 && defender.alive {
		defender.alive = false
		application.killed = true
		if defender.inanimate {
			// CHAR_BATTLEFLG_ABIO优先于暴击分支, 无论DamageSub原本返回0、1或2,
			// 都强制覆盖成类型1且不消费暴击死亡随机数.
			application.knockback = pb.CombatKnockbackType_CombatKnockbackType_AccumulatedOverkill
		} else if critical && combatKind(defender.unit) != combatUnitKindPlayer {
			// 即使DamageSub已经返回类型1或2, 旧服仍消费这一抽. 只有成功时
			// 才覆盖为类型1; 失败必须保留公共尾部原结果.
			if r.random.rangeInt(1, 100) < 50 {
				application.knockback = pb.CombatKnockbackType_CombatKnockbackType_AccumulatedOverkill
			}
		}
		if defender.ultimateKnockbackImmune {
			// 源码在ABIO/暴击覆盖后再次检查雷尔真实基础形象, 所以排除必须
			// 位于最后. 随机数已经按上面的条件消费, 累计值也保持原样.
			application.knockback = pb.CombatKnockbackType_CombatKnockbackType_Unknown
		}
	}
	return application
}

// combatEffectResult是结算器内部的一项原子结果.
//
// 结算阶段先用该结构收集一次动作产生的多个结果, 回合出口再把同一动作的
// 全部结果按顺序转换为CombatActionStep.effect_list.
type combatEffectResult struct {
	EffectKind        combatEffectKind
	SourceUnitKeyList []*pb.CombatUnitKey
	TargetUnitKeyList []*pb.CombatUnitKey
	UnitDeltaList     []*pb.CombatUnitStateDelta
	Damage            *combatDamageDetail
	Knockdown         *pb.CombatKnockdownDetail
	Knockback         *pb.CombatKnockbackDetail
	EscapeSucceeded   bool
}

func combatAppendEffect(event *combatStepResult, effect *combatEffectResult) {
	if event == nil || effect == nil {
		return
	}
	event.EffectList = append(event.EffectList, effect)
}

func combatHitResults(roll combatAttackRoll, defender *combatUnitRuntimeState) []combatHitResult {
	results := make([]combatHitResult, 0, 2)
	if roll.dodged {
		results = append(results, combatHitResultDodge)
		return results
	}
	if roll.critical {
		results = append(results, combatHitResultCritical)
	} else if roll.damage > 0 {
		results = append(results, combatHitResultNormal)
	} else {
		results = append(results, combatHitResultMiss)
	}
	if combatGuardReductionActive(defender) && !roll.guardBypassed {
		results = append(results, combatHitResultGuard)
	}
	return results
}

func combatAppendDamageEffect(event *combatStepResult, attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, roll combatAttackRoll, application combatDamageApplication) {
	deltaList := make([]*pb.CombatUnitStateDelta, 0, 1)
	if application.appliedDamage > 0 || application.killed {
		unitDelta := &pb.CombatUnitStateDelta{
			UnitKey: cloneCombatUnitKey(defender.unit.GetKey()),
		}
		if application.appliedDamage > 0 {
			unitDelta.AssetDeltaList = []*pb.CombatAssetDelta{{
				AssetType: pb.CombatAssetType_CombatAssetType_HP,
				Delta:     combatClampDelta(application.appliedDamage),
				After:     combatClampUint32(application.hpAfter),
			}}
		}
		if application.killed {
			unitDelta.AliveChanged = true
			unitDelta.Alive = false
		}
		deltaList = append(deltaList, unitDelta)
	}
	combatAppendEffect(event, &combatEffectResult{
		EffectKind:        combatEffectKindDamage,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		UnitDeltaList:     deltaList,
		Damage: &combatDamageDetail{
			DisplayedDamage: combatClampUint32(roll.damage),
			AppliedHpDamage: combatClampUint32(application.appliedDamage),
			HpBefore:        combatClampUint32(application.hpBefore),
			HpAfter:         combatClampUint32(application.hpAfter),
			HitResultList:   combatHitResults(roll, defender),
		},
	})
}

func combatAppendDefeatEffects(event *combatStepResult, attacker *combatUnitRuntimeState, defender *combatUnitRuntimeState, application combatDamageApplication, counter bool) {
	if !application.killed {
		return
	}
	if application.knockback != pb.CombatKnockbackType_CombatKnockbackType_Unknown {
		combatAppendEffect(event, &combatEffectResult{
			EffectKind:        combatEffectKindKnockback,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
			Knockback: &pb.CombatKnockbackDetail{
				KnockbackType: application.knockback,
				FromPosition:  combatPosition(defender.unit),
			},
		})
	} else {
		combatAppendEffect(event, &combatEffectResult{
			EffectKind:        combatEffectKindKnockdown,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
			Knockdown: &pb.CombatKnockdownDetail{
				FallPosition:         combatPosition(defender.unit),
				ReturnHomeBeforeFall: counter,
			},
		})
	}
}

// combatStateAtPosition按阵营和站位查询运行态, 包含死亡和已离场单位.
func (r *CombatRoom) combatStateAtPosition(camp pb.CombatCamp, position uint32) *combatUnitRuntimeState {
	if r == nil {
		return nil
	}
	for _, state := range r.unitStates {
		if state != nil && state.unit != nil &&
			state.unit.GetCamp() == camp && state.unit.GetPosition() == position {
			return state
		}
	}
	return nil
}

type combatAttackOutcome struct {
	continueCounter bool
	defender        *combatUnitRuntimeState
}

// executeSingleAttack结算一个普通物理、破除防御或连续攻击段.
func (r *CombatRoom) executeSingleAttack(action *combatAction, counter bool, events *[]*combatStepResult) combatAttackOutcome {
	if r == nil || action == nil {
		return combatAttackOutcome{}
	}
	attacker := r.stateByKey(action.unitKey)
	if attacker == nil || !attacker.alive || attacker.escaped {
		return combatAttackOutcome{}
	}
	defender := r.resolveCombatTarget(action)
	if defender == nil {
		return combatAttackOutcome{}
	}

	event := &combatStepResult{
		EventKind:         combatStepKindAction,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
	}
	if counter {
		event.EventKind = combatStepKindCounter
	}

	roll := r.combatAttackRoll(
		attacker,
		defender,
		false,
		counter,
		action.isGuardBreak(),
	)
	if roll.dodged {
		combatAppendEffect(event, &combatEffectResult{
			EffectKind:        combatEffectKindDodge,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(attacker.unit.GetKey())},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		})
		*events = append(*events, event)
		return combatAttackOutcome{continueCounter: true, defender: defender}
	}
	if action.isGuardBreak() && !combatGuardReductionActive(defender) {
		roll.critical = false
		roll.guardBypassed = false
		roll.damage = 0
	}
	if action.usesMultiSegmentDamageDivision() {
		roll.damage = combatContinuationAttackAdjustedDamage(roll.damage, action.segmentCount)
	}

	application := r.applyCombatDamage(attacker, defender, roll.damage, roll.critical)
	combatAppendDamageEffect(event, attacker, defender, roll, application)
	combatAppendDefeatEffects(event, attacker, defender, application, counter)
	*events = append(*events, event)

	continueCounter := !roll.critical && !combatGuardReductionActive(defender) && !application.killed
	if counter && roll.damage == 0 {
		continueCounter = false
	}
	return combatAttackOutcome{
		continueCounter: continueCounter,
		defender:        defender,
	}
}

// executeContinuationAttack按8.5 BATTLE_COM_S_RENZOKU结算一整个连续攻击动作.
//
// 每段都重新调用完整物理攻击段, 因而独立执行闪避、暴击、基础伤害、Guard、最低伤害
// 和本段HP变化. segmentCount同时是计划段数和gDamageDiv; 每段步骤携带同一技能ID,
// 客户端根据相邻Action步骤的逻辑顺序自行编排多段攻击移动.
//
// 原目标在前一段倒下后, 下一段仍以原声明目标重新解析. resolveCombatTarget发现其
// 已失效时会按当前存活敌方列表执行一次scaled RAND随机换目标, 对应8.5每次重新把
// 原目标写回COM2后调用BATTLE_TargetAdjust/BATTLE_DefaultAttacker的行为.
//
// 本函数只返回最后一个实际执行段的BATTLE_Attack等价结果. 外层必须在全部段完成后
// 才使用该结果和最终目标检查一次反击链; 不得在每段后分别反击. 任一阵营已无有效
// 存活单位或攻击者自身失效时立即停止剩余段, 未执行段不生成事件.
func (r *CombatRoom) executeContinuationAttack(action *combatAction, events *[]*combatStepResult) combatAttackOutcome {
	if action == nil || action.segmentCount == 0 {
		return combatAttackOutcome{}
	}
	// 8.5在进入通用攻击循环前把连续攻击和全部WildViolentAttack的COM1改成ATTACK. 该动作在
	// 此行之前仍不能反击; 从这里开始, 如果之后发生反击的反击, 本动作可以作为
	// 普通ATTACK通过BATTLE_CounterCheck. 标记不改变合击分组, 因为分组已在
	// 整回合实际执行前按原始专用命令完成.
	action.promoteSpecialAttackCommand()
	var lastOutcome combatAttackOutcome
	for segmentIndex := uint32(1); segmentIndex <= action.segmentCount; segmentIndex++ {
		attacker := r.stateByKey(action.unitKey)
		if attacker == nil || !attacker.alive || attacker.escaped {
			break
		}
		if r.battleSettlementIfFinished() != nil {
			break
		}
		outcome := r.executeSingleAttack(action, false, events)
		if outcome.defender == nil {
			break
		}
		lastOutcome = outcome
		if r.battleSettlementIfFinished() != nil {
			break
		}
	}
	return lastOutcome
}

func (r *CombatRoom) executeCounterChain(initialAction *combatAction, initialDefender *combatUnitRuntimeState, actionByUnit map[string]*combatAction, events *[]*combatStepResult) {
	if initialDefender == nil {
		return
	}
	originalAttacker := r.stateByKey(initialAction.unitKey)
	attacker := initialDefender
	defender := originalAttacker
	for index := uint32(1); index <= combatMaximumCounter; index++ {
		if attacker == nil || defender == nil || !attacker.alive || attacker.escaped || !defender.alive || defender.escaped {
			return
		}
		attackerAction := actionByUnit[combatUnitKeyMapKey(attacker.unit.GetKey())]
		if attackerAction == nil ||
			!attackerAction.canCounter() ||
			attackerAction.comboMember || !r.combatCounterCheck(attacker, defender) {
			return
		}
		// Counter步骤保留触发反击的原技能ID.
		counterAction := &combatAction{
			unitKey:   cloneCombatUnitKey(attacker.unit.GetKey()),
			kind:      combatActionKindAttack,
			skillID:   attackerAction.skillID,
			targetKey: cloneCombatUnitKey(defender.unit.GetKey()),
		}
		outcome := r.executeSingleAttack(counterAction, true, events)
		// 8.5每次BATTLE_Counter返回后立即把本次反击者单独放入
		// aAttackList并调用BATTLE_AddProfit. 必须先于下一次双方交换处理,
		// 否则反击击杀会被外层主动动作来源错误认领.
		r.addPVEEnemyDefeatProfit([]*pb.CombatUnitKey{counterAction.unitKey})
		if !outcome.continueCounter {
			return
		}
		attacker, defender = defender, attacker
	}
}

func (r *CombatRoom) enemyAIActions() []*combatAction {
	actions := make([]*combatAction, 0, len(r.enemyUnits))
	for _, unit := range r.enemyUnits {
		if unit == nil || !r.isAlive(unit.GetKey()) {
			continue
		}
		var pet *gameconfig.PetEntry
		if gameconfig.GGameConfig != nil && gameconfig.GGameConfig.Pet != nil {
			pet = gameconfig.GGameConfig.Pet.Get(unit.GetPetId())
		}
		var ai *gameconfig.PetBattleAIEntry
		if pet != nil {
			// enemy.group.yaml直接引用宠物模板, 敌方AI与该模板共用pet.yaml battleAI.
			ai = pet.BattleAI
		}
		if pet == nil || ai == nil {
			actions = append(actions, &combatAction{
				unitKey:   cloneCombatUnitKey(unit.GetKey()),
				kind:      combatActionKindDefense,
				skillID:   combatSkillDefense,
				targetKey: cloneCombatUnitKey(unit.GetKey()),
			})
			continue
		}
		totalWeight := uint64(aiValue(ai.AttackWeight)) + uint64(aiValue(ai.DefenseWeight)) + uint64(aiValue(ai.EscapeWeight))
		for slotIndex := 0; slotIndex < combatEnemyAISkillSlotCount; slotIndex++ {
			totalWeight += uint64(enemyAISkillSlotWeight(ai, slotIndex))
		}
		action := &combatAction{unitKey: cloneCombatUnitKey(unit.GetKey()), targetKey: cloneCombatUnitKey(unit.GetKey())}
		if totalWeight == 0 {
			action.kind = combatActionKindDefense
			action.skillID = combatSkillDefense
			actions = append(actions, action)
			continue
		}
		// BATTLE_ai_normal先把at/gu/ma/es/wa[0..6]相加, 再恰好执行一次
		// RAND(0, totalWeight-1). 这里保留同样的0起点和严格小于边界,
		// 使固定随机值可以直接与8.5动作区间逐项对照. 给定8.5源码中的
		// ma虽然参与该顺序, 但没有执行分支且现有数据也没有ma字段, 因而其
		// 权重保持源码数组默认值0; 不能擅自把wa宠物技能解释成另一套魔法动作.
		roll := uint64(r.random.rangeInt(0, int64(totalWeight-1)))
		attackWeight := uint64(aiValue(ai.AttackWeight))
		defenseWeight := uint64(aiValue(ai.DefenseWeight))
		escapeWeight := uint64(aiValue(ai.EscapeWeight))
		switch {
		case roll < attackWeight:
			action.kind = combatActionKindAttack
			action.skillID = combatSkillAttack
			action.targetKey = r.enemyAITarget(unit, ai)
			if action.targetKey == nil {
				action.kind = combatActionKindDefense
				action.skillID = combatSkillDefense
				action.targetKey = cloneCombatUnitKey(unit.GetKey())
			}
		case roll < attackWeight+defenseWeight:
			action.kind = combatActionKindDefense
			action.skillID = combatSkillDefense
		case roll < attackWeight+defenseWeight+escapeWeight:
			action.kind = combatActionKindEscape
			action.skillID = 0
			action.targetKey = nil
		default:
			// 技能槽和普通攻击共用同一套目标选择流程. 目标选择已消费的随机数
			// 不会因后续技能解析失败而撤销或补偿.
			selectedTarget := r.enemyAITarget(unit, ai)
			if selectedTarget == nil {
				continue
			}
			slotRoll := roll - attackWeight - defenseWeight - escapeWeight
			slotIndex := -1
			for index := 0; index < combatEnemyAISkillSlotCount; index++ {
				weight := uint64(enemyAISkillSlotWeight(ai, index))
				if slotRoll < weight {
					slotIndex = index
					break
				}
				slotRoll -= weight
			}
			if slotIndex < 0 || slotIndex >= len(pet.SkillSlots) || pet.SkillSlots[slotIndex] == 0 {
				// 正常配置加载会提前拒绝非0权重指向空槽. 此处保护内存构造和
				// 未来热替换.
				continue
			}
			skillAction, err := r.enemyPetCombatSkillAction(unit, pet.SkillSlots[slotIndex], selectedTarget)
			if err != nil || skillAction == nil {
				// 技能解析失败时不改用其他动作, 也不插入未知动作占位.
				continue
			}
			actions = append(actions, skillAction)
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

// enemyAISkillSlotWeight安全读取8.5固定七槽wa权重. nil表示旧配置没有显式
// 开启任何技能槽; 长度异常会由配置check提前拒绝, 这里仍按源码最多读取七项.
func enemyAISkillSlotWeight(ai *gameconfig.PetBattleAIEntry, slotIndex int) uint32 {
	if ai == nil || slotIndex < 0 || slotIndex >= combatEnemyAISkillSlotCount ||
		slotIndex >= len(ai.SkillSlotWeights) {
		return 0
	}
	return ai.SkillSlotWeights[slotIndex]
}

func aiValue(value *uint32) uint32 {
	if value == nil {
		return 0
	}
	return *value
}

// enemyAITarget复刻启用_ENEMY_ATTACK_AI后的BATTLE_ai_normal普通攻击目标流程.
//
// 8.5先按单侧position 0至9收集目标. player/pet/leader范围没有候选时会把
// 局部at[1]降级为all并重新收集, 不会直接取消攻击. random策略只执行一次
// RAND(0,cnt-1); 其余六种最优策略先确定top, 再无条件执行RAND(0,rn),
// 只有结果为0才追加一次RAND(0,cnt-1)并改用随机候选. 即使只有一个候选,
// 两次随机也都不能省略, 否则后续整场随机序列会与8.5错位.
func (r *CombatRoom) enemyAITarget(source *pb.CombatUnit, ai *gameconfig.PetBattleAIEntry) *pb.CombatUnitKey {
	if source == nil || ai == nil || ai.TargetScope == nil || ai.TargetSelection == nil {
		return nil
	}
	candidates := r.enemyAITargetCandidates(source, *ai.TargetScope)
	if len(candidates) == 0 && *ai.TargetScope != gameconfig.PetBattleAITargetScopeAllOpponents {
		// 旧代码只修改本次调用的局部at[1], 配置本身保持不变. 因而下一回合
		// 仍会先尝试原范围, 本回合则在原范围产生的随机数之后回退all.
		candidates = r.enemyAITargetCandidates(source, gameconfig.PetBattleAITargetScopeAllOpponents)
	}
	if len(candidates) == 0 {
		return nil
	}

	if *ai.TargetSelection == gameconfig.PetBattleAITargetSelectionRandom {
		selected := candidates[r.random.rangeInt(0, int64(len(candidates)-1))]
		return cloneCombatUnitKey(selected.unit.GetKey())
	}

	selected := r.enemyAIBestTarget(source, candidates, *ai.TargetSelection)
	if selected == nil {
		return nil
	}
	// rn未写时8.5局部数组默认值为1. 正常生产配置的check要求所有非random
	// 策略显式保存该值; nil回退只保护内存构造和旧测试数据, 不改变配置约束.
	randomRollMax := uint32(1)
	if ai.TargetRandomRollMax != nil {
		randomRollMax = *ai.TargetRandomRollMax
	}
	if r.random.rangeInt(0, int64(randomRollMax)) == 0 {
		selected = candidates[r.random.rangeInt(0, int64(len(candidates)-1))]
	}
	return cloneCombatUnitKey(selected.unit.GetKey())
}

// enemyAITargetCandidates按8.5单侧Entry的position顺序收集仍可作为目标的单位.
//
// leader范围不是“只选队长”. 旧服对真正队长无条件加入, 对每个其他存活Entry
// 都分别执行RAND(0,2), 仅结果0时加入. 当前PVE队伍尚未接入显式PARTY_MODE,
// 因而以position最小的存活玩家角色作为当前单人/未来队伍入口的队长; 随机纳入
// 非队长的规则和抽数已经严格保留, D037接入队伍元数据时只需替换队长识别来源.
func (r *CombatRoom) enemyAITargetCandidates(source *pb.CombatUnit, scope gameconfig.PetBattleAITargetScope) []*combatUnitRuntimeState {
	if r == nil || source == nil {
		return nil
	}
	targetCamp := pb.CombatCamp_CombatCamp_Initiator
	if source.GetCamp() == pb.CombatCamp_CombatCamp_Initiator {
		targetCamp = pb.CombatCamp_CombatCamp_Defender
	}

	var partyLeader *combatUnitRuntimeState
	if scope == gameconfig.PetBattleAITargetScopePartyLeader {
		for position := uint32(0); position < 10; position++ {
			candidate := r.combatStateAtPosition(targetCamp, position)
			if candidate != nil && candidate.unit != nil && r.isAlive(candidate.unit.GetKey()) &&
				combatUnitIsPlayerCharacter(candidate.unit) {
				partyLeader = candidate
				break
			}
		}
	}

	candidates := make([]*combatUnitRuntimeState, 0, 10)
	for position := uint32(0); position < 10; position++ {
		candidate := r.combatStateAtPosition(targetCamp, position)
		if candidate == nil || candidate.unit == nil || !r.isAlive(candidate.unit.GetKey()) {
			continue
		}
		include := false
		switch scope {
		case gameconfig.PetBattleAITargetScopeAllOpponents:
			include = true
		case gameconfig.PetBattleAITargetScopePlayerCharacters:
			include = combatUnitIsPlayerCharacter(candidate.unit)
		case gameconfig.PetBattleAITargetScopePlayerPets:
			include = combatKind(candidate.unit) == combatUnitKindPet
		case gameconfig.PetBattleAITargetScopePartyLeader:
			if candidate == partyLeader {
				include = true
			} else {
				include = r.random.rangeInt(0, 2) == 0
			}
		}
		if include {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// enemyAIBestTarget返回8.5六种非random目标策略的top候选, 本函数不消费随机数.
// HP读取当前战斗HP; STR/DEX按源码使用原始四维CHAR_STR/CHAR_DEX, 不读取
// WORKATTACKPOWER、WORKQUICK或现代合成后的战斗攻击/敏捷. 所有比较都使用
// 严格大于或小于, 平局保留position更小的第一个候选.
func (r *CombatRoom) enemyAIBestTarget(source *pb.CombatUnit, candidates []*combatUnitRuntimeState, selection gameconfig.PetBattleAITargetSelection) *combatUnitRuntimeState {
	if len(candidates) == 0 {
		return nil
	}
	selected := candidates[0]
	switch selection {
	case gameconfig.PetBattleAITargetSelectionHighestHP:
		for _, candidate := range candidates[1:] {
			if candidate.hp > selected.hp {
				selected = candidate
			}
		}
	case gameconfig.PetBattleAITargetSelectionLowestHP:
		for _, candidate := range candidates[1:] {
			if candidate.hp < selected.hp {
				selected = candidate
			}
		}
	case gameconfig.PetBattleAITargetSelectionHighestAttack:
		for _, candidate := range candidates[1:] {
			if candidate.rawStrength > selected.rawStrength {
				selected = candidate
			}
		}
	case gameconfig.PetBattleAITargetSelectionHighestAgility:
		for _, candidate := range candidates[1:] {
			if candidate.rawDexterity > selected.rawDexterity {
				selected = candidate
			}
		}
	case gameconfig.PetBattleAITargetSelectionLowestAgility:
		for _, candidate := range candidates[1:] {
			if candidate.rawDexterity < selected.rawDexterity {
				selected = candidate
			}
		}
	case gameconfig.PetBattleAITargetSelectionElementalSubdue:
		elementIndex := enemyAISubdueTargetElement(combatEffectiveElementArray(r.stateByKey(source.GetKey())))
		selectedValue := combatEffectiveElementArray(selected)[elementIndex]
		for _, candidate := range candidates[1:] {
			candidateValue := combatEffectiveElementArray(candidate)[elementIndex]
			if candidateValue > selectedValue {
				selected = candidate
				selectedValue = candidateValue
			}
		}
	default:
		return nil
	}
	return selected
}

// enemyAISubdueTargetElement逐比较复刻8.5 GetSubdueAttribute.
//
// 返回值不是攻击者“最大元素”的下标, 而是源码决策树选出的克制目标元素:
// 例如纯火攻击者返回风, 随后AI选择WORKFIXWINDAT最高的候选. 相等值必须走
// 源码的else分支, 因而不能改写成先找最大元素再套普通克制环的简化算法.
func enemyAISubdueTargetElement(elements [combatElementCount]int64) int {
	earth := elements[combatElementEarth]
	water := elements[combatElementWater]
	fire := elements[combatElementFire]
	wind := elements[combatElementWind]
	if earth > fire {
		if water > wind {
			if earth > water {
				return combatElementWater
			}
			return combatElementFire
		}
		if earth > wind {
			return combatElementWater
		}
		return combatElementEarth
	}
	if water > wind {
		if fire > water {
			return combatElementWind
		}
		return combatElementFire
	}
	if fire > wind {
		return combatElementWind
	}
	return combatElementEarth
}

func (r *CombatRoom) resolveCombatTarget(action *combatAction) *combatUnitRuntimeState {
	if action == nil {
		return nil
	}
	if action.targetKey != nil && !combatUnitKeyEmpty(action.targetKey) {
		if target, err := r.validOpponentTarget(action.targetKey, action.unitKey); err == nil {
			return r.stateByKey(target)
		}
	}
	candidates := r.aliveOpponentKeys(action.unitKey)
	if len(candidates) == 0 {
		return nil
	}
	return r.stateByKey(candidates[r.random.rangeInt(0, int64(len(candidates)-1))])
}

// combatEscapeChance 按8.5 BATTLE_EscapeCheck计算本次PVE逃跑阈值, 但不消耗随机数也不修改运行态.
//
// escapeAttempts表示BATTLE_Escape已经记录完成的实际尝试次数. 8.5会先执行Entry.escape++,
// 随后在BATTLE_EscapeCheck中再次使用Entry.escape+1作为公式系数. 因此第一次实际尝试传入1,
// 公式系数却是2; 第二次实际尝试传入2, 公式系数是3. 这里保留这项看似多加一次的源码行为,
// 不能把公式系数与房间运行态记录的实际尝试次数混为一谈.
//
// 敌方平均等级沿用8.5有效Entry语义: 已倒下但尚未离场的单位仍参与平均等级计算,
// 已经escaped的单位等价于BATTLE_Exit移除Entry, 不再参与计算. 若敌方已经没有有效Entry,
// 8.5直接把阈值设为100. 阈值只限制最小值1, 不限制最大值; 大于100时必定成功是源码行为.
func (r *CombatRoom) combatEscapeChance(state *combatUnitRuntimeState, escapeAttempts uint32) int64 {
	luck := combatEffectiveLuck(state)
	if combatKind(state.unit) == combatUnitKindEnemy {
		switch state.rare {
		case 0:
			luck = 1
		case 1:
			luck = 3
		default:
			luck = 5
		}
	}
	levelSum := int64(0)
	levelCount := int64(0)
	for _, unit := range r.battleStart.GetUnitList() {
		if unit == nil || unit.GetCamp() == state.unit.GetCamp() {
			continue
		}
		opponent := r.stateByKey(unit.GetKey())
		if opponent == nil || opponent.escaped {
			continue
		}
		// TODO(D046): 8.5发现敌方Entry带CHAR_BATTLEFLG_ABIO时, 会先从敌方等级和中减100.
		// 当前生产战斗尚未建立ABIO异常状态运行态, 本轮不能用固定等级或临时布尔值伪造该分支.
		// 待状态系统接入后, 必须在这里按每个敌方单位分别扣减, 并补充混合ABIO/普通单位的平均等级测试.
		levelSum += int64(unit.GetAttribute().GetLevel())
		levelCount++
	}
	if levelCount == 0 {
		return 100
	}
	enemyAverageLevel := levelSum / levelCount
	myLevel := int64(state.unit.GetAttribute().GetLevel())
	// 8.5的BATTLE_Escape先递增Entry.escape, BATTLE_EscapeCheck又使用escape+1.
	// 因此首次尝试的公式系数是2, 但房间运行态记录的实际尝试次数仍是1.
	attempt := int64(escapeAttempts) + 1
	chance := int64(95) * attempt
	switch luck {
	case 4:
		chance = 60*attempt - 2*(enemyAverageLevel-myLevel)
	case 3:
		chance = 50*attempt - 2*(enemyAverageLevel-myLevel)
	case 2:
		chance = 40*attempt - 2*(enemyAverageLevel-myLevel)
	case 1:
		chance = 30*attempt - 2*(enemyAverageLevel-myLevel)
	}
	if chance < 1 {
		return 1
	}
	return chance
}

// combatEscapeLeavingStates 返回一次成功逃跑必须从战斗中移除的全部运行态, 且角色始终排在首位.
//
// 8.5 BATTLE_Exit处理玩家角色时, 会同时取角色Entry后方配对的战宠Entry并将两者移出战斗.
// 当前协议没有依赖固定槽位i+5查找战宠, 而是通过CombatUnitKey中的aid和character_uuid建立归属关系;
// 这样既保持角色与战宠共同离场的业务结果, 也不把8.5内存数组布局泄漏到现代房间模型.
//
// 只有玩家角色会携带战宠离场. 敌方单位由AI触发逃跑时只移除自己. 查找时不要求战宠alive,
// 因为8.5退出流程同样会清理已经倒下但仍占据配对Entry的战宠. 已经escaped的战宠不会重复下发状态变化.
func (r *CombatRoom) combatEscapeLeavingStates(escapee *combatUnitRuntimeState) []*combatUnitRuntimeState {
	leavingStates := []*combatUnitRuntimeState{escapee}
	if r == nil || escapee == nil || !combatUnitIsPlayerCharacter(escapee.unit) || r.battleStart == nil {
		return leavingStates
	}
	escapeeKey := escapee.unit.GetKey()
	if escapeeKey == nil {
		return leavingStates
	}
	for _, unit := range r.battleStart.GetUnitList() {
		if unit == nil || unit.GetCamp() != escapee.unit.GetCamp() || combatKind(unit) != combatUnitKindPet {
			continue
		}
		unitKey := unit.GetKey()
		if unitKey == nil || unitKey.GetAid() != escapeeKey.GetAid() || unitKey.GetCharacterUuid() != escapeeKey.GetCharacterUuid() {
			continue
		}
		petState := r.stateByKey(unitKey)
		if petState == nil || petState.escaped {
			continue
		}
		leavingStates = append(leavingStates, petState)
		break
	}
	return leavingStates
}

// executeEscape执行一次已经通过技能归属和参数校验的逃离技能.
//
// 处理顺序严格分为四步:
// 1. 校验出手单位仍存活且未离场. 无效动作不会增加次数、消耗随机数或生成失败事件.
// 2. 先增加实际尝试次数, 再按8.5的escape+1系数计算阈值, 最后只抽取一次RAND(1,100).
// 3. 成功时同步移除角色及其当前战宠并清除Guard; 失败时不修改两者的在场状态.
// 4. 无论成功或失败都生成一个Escape效果. source_unit_key标识主动单位; 成功时
// unit_delta_list完整下发全部实际离场单位, 失败时不生成离场delta.
//
// 当前阶段只开发PVE. PVP在8.5中会直接逃跑成功, 但匹配、双方输入、结算和客户端流程均已冻结,
// 此处不得提前加入PVP分支并把未经完整验证的行为计入PVE完成度.
func (r *CombatRoom) executeEscape(action *combatAction, events *[]*combatStepResult) bool {
	state := r.stateByKey(action.unitKey)
	if state == nil || !state.alive || state.escaped {
		return false
	}
	state.escapeAttempts++
	chance := r.combatEscapeChance(state, state.escapeAttempts)
	// rangeInt按8.5 scaled RAND语义只抽取一次[1,100]闭区间值. 此处必须保留源码
	// BATTLE_EscapeCheck的严格小于比较; 改成<=会把Esc阈值对应的整数也错误计为成功.
	succeeded := r.random.rangeInt(1, 100) < chance
	leavingStates := []*combatUnitRuntimeState(nil)
	if succeeded {
		leavingStates = r.combatEscapeLeavingStates(state)
		for _, leavingState := range leavingStates {
			leavingState.escaped = true
			leavingState.guard = false
		}
	}
	event := &combatStepResult{
		EventKind:         combatStepKindEscape,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
	}
	deltaList := make([]*pb.CombatUnitStateDelta, 0, len(leavingStates))
	if succeeded {
		// UnitDeltaList按角色、战宠顺序完整描述本次离场结果.
		// CombatActionStep.source_unit_key标识主动使用技能的角色; 客户端不得
		// 在本地自行把角色逃跑扩展成宠物状态变化.
		for _, leavingState := range leavingStates {
			deltaList = append(deltaList, &pb.CombatUnitStateDelta{
				UnitKey:        cloneCombatUnitKey(leavingState.unit.GetKey()),
				EscapedChanged: true,
				Escaped:        true,
			})
		}
	}
	combatAppendEffect(event, &combatEffectResult{
		EffectKind:        combatEffectKindEscape,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(state.unit.GetKey())},
		UnitDeltaList:     deltaList,
		EscapeSucceeded:   succeeded,
	})
	*events = append(*events, event)
	return succeeded
}

type combatActionGroup struct {
	actions []*combatAction
	combo   bool
}

// combatStepResult是服务端结算阶段生成的顺序逻辑草稿.
//
// 结算器继续按动作、反击和状态触发逐步追加结果, 回合出口再依据顶层动作边界
// 转换为CombatEvent.action_step_list及其effect_list. 这些仅用于结算编排的字段
// 不会进入客户端协议.
type combatStepKind uint8

const (
	combatStepKindUnknown combatStepKind = iota
	combatStepKindAction
	combatStepKindCounter
	combatStepKindEscape
)

type combatStepResult struct {
	EventKind         combatStepKind
	SkillId           uint32
	SourceUnitKeyList []*pb.CombatUnitKey
	TargetUnitKeyList []*pb.CombatUnitKey
	EffectList        []*combatEffectResult

	topLevelStart          bool
	topLevelSourceUnitKeys []*pb.CombatUnitKey
	topLevelTargetUnitKeys []*pb.CombatUnitKey
}

func cloneCombatUnitKeyList(unitKeys []*pb.CombatUnitKey) []*pb.CombatUnitKey {
	result := make([]*pb.CombatUnitKey, 0, len(unitKeys))
	for _, unitKey := range unitKeys {
		if unitKey != nil {
			result = append(result, cloneCombatUnitKey(unitKey))
		}
	}
	return result
}

// markCombatTopLevelEvent记录一段连续结算步骤所属的顶层声明动作.
//
// sourceUnitKeys的数量同时表达是否合击, 列表顺序就是合击成员顺序.
func markCombatTopLevelEvent(
	steps []*combatStepResult,
	sourceUnitKeys []*pb.CombatUnitKey,
	targetUnitKeys []*pb.CombatUnitKey,
) {
	for _, step := range steps {
		if step == nil {
			continue
		}
		step.topLevelStart = true
		step.topLevelSourceUnitKeys = cloneCombatUnitKeyList(sourceUnitKeys)
		step.topLevelTargetUnitKeys = cloneCombatUnitKeyList(targetUnitKeys)
		return
	}
}

func combatActionTopLevelSources(action *combatAction) []*pb.CombatUnitKey {
	if action == nil || action.unitKey == nil {
		return nil
	}
	return []*pb.CombatUnitKey{action.unitKey}
}

func combatActionTopLevelTargets(action *combatAction, steps []*combatStepResult) []*pb.CombatUnitKey {
	if declaredTargets := combatActionDeclaredTargets(action); len(declaredTargets) > 0 {
		return declaredTargets
	}
	for _, step := range steps {
		if step != nil && len(step.TargetUnitKeyList) > 0 {
			return step.TargetUnitKeyList
		}
	}
	return nil
}

func combatProtocolActionCause(stepKind combatStepKind) pb.CombatActionCause {
	switch stepKind {
	case combatStepKindAction, combatStepKindEscape:
		return pb.CombatActionCause_CombatActionCause_Active
	case combatStepKindCounter:
		return pb.CombatActionCause_CombatActionCause_Counter
	default:
		return pb.CombatActionCause_CombatActionCause_Unknown
	}
}

func combatProtocolDamageDetail(detail *combatDamageDetail) *pb.CombatDamageDetail {
	if detail == nil {
		return nil
	}
	result := &pb.CombatDamageDetail{
		DisplayedDamage: detail.GetDisplayedDamage(),
	}
	for _, hitResult := range detail.GetHitResultList() {
		switch hitResult {
		case combatHitResultNormal:
			result.Outcome = pb.CombatHitOutcome_CombatHitOutcome_Hit
		case combatHitResultCritical:
			result.Outcome = pb.CombatHitOutcome_CombatHitOutcome_Hit
			result.Critical = true
		case combatHitResultMiss:
			result.Outcome = pb.CombatHitOutcome_CombatHitOutcome_Miss
		case combatHitResultDodge:
			result.Outcome = pb.CombatHitOutcome_CombatHitOutcome_Dodge
		case combatHitResultGuard:
			result.Guarded = true
		}
	}
	return result
}

// buildCombatProtocolEffect把结算器原子结果转换为一个结构化oneof效果.
func buildCombatProtocolEffect(effect *combatEffectResult) *pb.CombatEffect {
	if effect == nil || effect.EffectKind == combatEffectKindActionOnly {
		return nil
	}
	var sourceUnitKey *pb.CombatUnitKey
	if len(effect.SourceUnitKeyList) > 0 {
		sourceUnitKey = cloneCombatUnitKey(effect.SourceUnitKeyList[0])
	}
	result := &pb.CombatEffect{
		EffectSourceUnitKey: sourceUnitKey,
		AffectedUnitKeyList: cloneCombatUnitKeyList(effect.TargetUnitKeyList),
		UnitDeltaList:       effect.UnitDeltaList,
	}
	switch effect.EffectKind {
	case combatEffectKindDamage:
		result.Detail = &pb.CombatEffect_Damage{Damage: combatProtocolDamageDetail(effect.Damage)}
	case combatEffectKindGuard:
		result.Detail = &pb.CombatEffect_Guard{Guard: &pb.CombatGuardDetail{}}
	case combatEffectKindDodge:
		result.Detail = &pb.CombatEffect_Damage{Damage: &pb.CombatDamageDetail{
			Outcome: pb.CombatHitOutcome_CombatHitOutcome_Dodge,
		}}
	case combatEffectKindKnockdown:
		result.Detail = &pb.CombatEffect_Knockdown{Knockdown: effect.Knockdown}
	case combatEffectKindKnockback:
		result.Detail = &pb.CombatEffect_Knockback{Knockback: effect.Knockback}
	case combatEffectKindEscape:
		result.Detail = &pb.CombatEffect_Escape{Escape: &pb.CombatEscapeDetail{Success: effect.EscapeSucceeded}}
	default:
		return nil
	}
	return result
}

// buildCombatProtocolActionStep把一次内部动作及其全部原子结果转换为一个协议动作步骤.
func buildCombatProtocolActionStep(step *combatStepResult) *pb.CombatActionStep {
	if step == nil {
		return nil
	}
	var sourceUnitKey *pb.CombatUnitKey
	if len(step.SourceUnitKeyList) > 0 {
		sourceUnitKey = cloneCombatUnitKey(step.SourceUnitKeyList[0])
	}
	result := &pb.CombatActionStep{
		Cause:                   combatProtocolActionCause(step.EventKind),
		SkillId:                 step.SkillId,
		ActorUnitKey:            sourceUnitKey,
		ActionTargetUnitKeyList: cloneCombatUnitKeyList(step.TargetUnitKeyList),
		EffectList:              make([]*pb.CombatEffect, 0, len(step.EffectList)),
	}
	for _, effect := range step.EffectList {
		if protocolEffect := buildCombatProtocolEffect(effect); protocolEffect != nil {
			result.EffectList = append(result.EffectList, protocolEffect)
		}
	}
	return result
}

// buildCombatProtocolEvents把内部顺序结果按顶层声明动作分组.
func buildCombatProtocolEvents(steps []*combatStepResult) []*pb.CombatEvent {
	events := make([]*pb.CombatEvent, 0, len(steps))
	var current *pb.CombatEvent
	for _, step := range steps {
		if step == nil {
			continue
		}
		if step.topLevelStart || current == nil {
			sourceUnitKeys := step.topLevelSourceUnitKeys
			targetUnitKeys := step.topLevelTargetUnitKeys
			if len(sourceUnitKeys) == 0 {
				sourceUnitKeys = step.SourceUnitKeyList
			}
			if len(targetUnitKeys) == 0 {
				targetUnitKeys = step.TargetUnitKeyList
			}
			current = &pb.CombatEvent{
				DeclaredSourceUnitKeyList: cloneCombatUnitKeyList(sourceUnitKeys),
				DeclaredTargetUnitKeyList: cloneCombatUnitKeyList(targetUnitKeys),
			}
			events = append(events, current)
		}
		if protocolStep := buildCombatProtocolActionStep(step); protocolStep != nil {
			current.ActionStepList = append(current.ActionStepList, protocolStep)
		}
	}
	result := events[:0]
	for _, event := range events {
		if event != nil && len(event.ActionStepList) > 0 {
			result = append(result, event)
		}
	}
	return result
}

// combatCanMoveForComboCheck复刻8.5 BATTLE_CanMoveCheck在合击扫描时可见的状态.
//
// ComboCheck发生在任何成员的BATTLE_StatusSeq之前, ComboCheck2扫描后续成员时
// 也只读取当时的工作槽, 两处都不会递减状态、生成事件或消费随机数. 当前运行态
// 已经接入的不可行动槽依次是麻痹、石化、Sleep、Barrier、Dizzy和Dragnet;
// 职业状态T_ENCLOSE及DOOMTIME尚无PVE运行态, 接入时必须继续补入本检查.
//
// alive和escaped对应旧服HP及BATTLEMODE边界. 调用方仍需按各自源码语义决定
// 遇到离场成员时是停止向后扫描还是只跳过, 不能把这个纯状态判断扩大成遍历规则.
func combatCanMoveForComboCheck(state *combatUnitRuntimeState) bool {
	return state != nil && state.unit != nil && state.alive && !state.escaped
}

func (r *CombatRoom) buildCombatActionGroups(actions []*combatAction) []combatActionGroup {
	groups := make([]combatActionGroup, 0, len(actions))
	for index := 0; index < len(actions); {
		action := actions[index]
		state := (*combatUnitRuntimeState)(nil)
		if action != nil {
			state = r.stateByKey(action.unitKey)
		}
		// 8.5 ComboCheck只有“普通ATTACK、非投掷武器、HP为正且CanMove”
		// 的候选起点才执行RAND(1,100). 当前尚无武器运行态, 因而投掷武器
		// 条件继续留在待开发项; 已接入的不可行动状态必须在抽数之前排除.
		if action == nil || !action.isAttack() || !combatCanMoveForComboCheck(state) {
			groups = append(groups, combatActionGroup{actions: []*combatAction{action}})
			index++
			continue
		}
		chance := int64(50)
		if combatKind(state.unit) == combatUnitKindEnemy {
			chance = 20
		}
		if r.random.rangeInt(1, 100) <= chance {
			end := index + 1
			for end < len(actions) && combatComboActionMatches(r, action, actions[end]) {
				end++
			}
			if end-index >= 2 {
				comboActions := append([]*combatAction(nil), actions[index:end]...)
				for _, comboAction := range comboActions {
					comboAction.comboMember = true
				}
				groups = append(groups, combatActionGroup{actions: comboActions, combo: true})
				index = end
				continue
			}
		}
		groups = append(groups, combatActionGroup{actions: []*combatAction{action}})
		index++
	}
	return groups
}

func combatComboActionMatches(room *CombatRoom, first *combatAction, next *combatAction) bool {
	if first == nil || next == nil || !next.isAttack() ||
		!combatCanMoveForComboCheck(room.stateByKey(next.unitKey)) ||
		!combatUnitKeyEqual(first.targetKey, next.targetKey) {
		return false
	}
	firstCamp, firstOK := room.unitCamp(first.unitKey)
	nextCamp, nextOK := room.unitCamp(next.unitKey)
	return firstOK && nextOK && firstCamp == nextCamp
}

func (r *CombatRoom) executeCombo(group combatActionGroup, actionByUnit map[string]*combatAction, events *[]*combatStepResult) {
	activeActions := make([]*combatAction, 0, len(group.actions))
	for _, action := range group.actions {
		if action == nil || !r.isAlive(action.unitKey) {
			continue
		}
		captureCombatActionDeclaredTarget(action)
		action.comboMember = true
		activeActions = append(activeActions, action)
	}
	if len(activeActions) == 0 {
		return
	}
	if len(activeActions) == 1 {
		action := activeActions[0]
		action.comboMember = false
		stepStart := len(*events)
		r.executeStandaloneAction(action, actionByUnit, events)
		steps := (*events)[stepStart:]
		markCombatTopLevelEvent(
			steps,
			combatActionTopLevelSources(action),
			combatActionTopLevelTargets(action, steps),
		)
		return
	}

	defender := r.resolveCombatTarget(activeActions[0])
	if defender == nil {
		return
	}
	stepStart := len(*events)
	r.executeComboMembers(activeActions, defender, events)
	steps := (*events)[stepStart:]
	markCombatTopLevelEvent(
		steps,
		combatActionUnitKeys(activeActions),
		combatActionTopLevelTargets(activeActions[0], steps),
	)
	r.addPVEEnemyDefeatProfit(combatActionUnitKeys(activeActions))
}

// combatActionUnitKeys按8.5 aAttackList顺序复制实际动作来源.
//
// 合击经验资格属于完整攻击列表, 不是最终造成致死伤害的单个成员. nil动作
// 不会进入BATTLE_Combo列表, 因而在这里跳过; 身份去重由死亡扫描统一处理.
func combatActionUnitKeys(actions []*combatAction) []*pb.CombatUnitKey {
	keys := make([]*pb.CombatUnitKey, 0, len(actions))
	for _, action := range actions {
		if action != nil && action.unitKey != nil {
			keys = append(keys, cloneCombatUnitKey(action.unitKey))
		}
	}
	return keys
}

// executeComboMembers结算已经完成分组的基础合击.
func (r *CombatRoom) executeComboMembers(activeActions []*combatAction, defender *combatUnitRuntimeState, events *[]*combatStepResult) {
	if r == nil || defender == nil || defender.unit == nil || !defender.alive || defender.escaped {
		return
	}

	type comboMemberRoll struct {
		action   *combatAction
		attacker *combatUnitRuntimeState
		roll     combatAttackRoll
	}
	memberRolls := make([]comboMemberRoll, 0, len(activeActions))
	totalDamage := uint64(0)
	for _, action := range activeActions {
		if action == nil {
			continue
		}
		attacker := r.stateByKey(action.unitKey)
		if attacker == nil || !attacker.alive || attacker.escaped {
			break
		}
		roll := r.combatAttackRoll(attacker, defender, true, false, false)
		if roll.damage == 0 {
			roll.damage = 1
		}
		totalDamage += roll.damage
		memberRolls = append(memberRolls, comboMemberRoll{action: action, attacker: attacker, roll: roll})
	}

	for index, member := range memberRolls {
		event := &combatStepResult{
			EventKind:         combatStepKindAction,
			SkillId:           member.action.skillID,
			SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(member.attacker.unit.GetKey())},
			TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(defender.unit.GetKey())},
		}
		if index != len(memberRolls)-1 {
			combatAppendEffect(event, &combatEffectResult{
				EffectKind:        combatEffectKindDamage,
				SourceUnitKeyList: cloneCombatUnitKeyList(event.SourceUnitKeyList),
				TargetUnitKeyList: cloneCombatUnitKeyList(event.TargetUnitKeyList),
				Damage: &combatDamageDetail{
					HitResultList: combatHitResults(member.roll, defender),
				},
			})
			*events = append(*events, event)
			continue
		}

		finalRoll := member.roll
		finalRoll.damage = totalDamage
		application := r.applyCombatDamage(member.attacker, defender, totalDamage, finalRoll.critical)
		combatAppendDamageEffect(event, member.attacker, defender, finalRoll, application)
		combatAppendDefeatEffects(event, member.attacker, defender, application, false)
		*events = append(*events, event)
	}
}

func (r *CombatRoom) appendGuardEvent(action *combatAction, events *[]*combatStepResult) {
	event := &combatStepResult{
		EventKind:         combatStepKindAction,
		SkillId:           action.skillID,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
	}
	combatAppendEffect(event, &combatEffectResult{
		EffectKind:        combatEffectKindGuard,
		SourceUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
		TargetUnitKeyList: []*pb.CombatUnitKey{cloneCombatUnitKey(action.unitKey)},
	})
	*events = append(*events, event)
}

func (r *CombatRoom) executeStandaloneAction(action *combatAction, actionByUnit map[string]*combatAction, events *[]*combatStepResult) *pb.CombatBattleSettlement {
	if action == nil || !r.isAlive(action.unitKey) {
		return nil
	}
	captureCombatActionDeclaredTarget(action)
	defer r.addPVEEnemyDefeatProfit([]*pb.CombatUnitKey{action.unitKey})

	switch {
	case action.isGuard():
		r.appendGuardEvent(action, events)
	case action.isStandby():
		appendCombatActionOnlyStep(action, events)
	case action.isContinuationAttack():
		firstEventIndex := len(*events)
		outcome := r.executeContinuationAttack(action, events)
		if outcome.continueCounter && len(*events) > firstEventIndex {
			r.executeCounterChain(action, outcome.defender, actionByUnit, events)
		}
	case action.kind == combatActionKindEscape:
		if r.executeEscape(action, events) && r.battleSettlementIfFinished() != nil {
			state := r.stateByKey(action.unitKey)
			if state != nil && state.unit != nil {
				return r.escapeSettlement(state.unit.GetCamp())
			}
		}
	case action.isAttack() || action.isGuardBreak():
		outcome := r.executeSingleAttack(action, false, events)
		if outcome.continueCounter && len(*events) > 0 {
			r.executeCounterChain(action, outcome.defender, actionByUnit, events)
		}
	}
	return nil
}

// activateRoundGuards在行动值排序和任何单位实际出手之前, 激活本回合已经锁定的防御技能.
//
// 8.5把单位本回合命令保存在CHAR_WORKBATTLECOM1. 攻击结算读取到BATTLE_COM_GUARD时会
// 立即跳过普通闪避、执行BATTLE_GuardAdjust减伤并在受击结果附加BCF_GUARD, 不要求防御者
// 先轮到自己的行动. 轮到防御者时调用BATTLE_Guard生成的bg表现是另一件事, 不能拿该表现
// 的执行时机决定减伤是否生效. 因此这里必须在排序前一次性写入运行态guard标记.
//
// 只激活仍存活、尚未离场且回合开始时未被麻痹、Sleep、石化、Dizzy或
// Dragnet禁止行动的单位.
// 已倒下、已经逃离、动作为空或本回合选择其他技能的单位不能获得防御效果.
// 本函数不清理上一回合标记; completeCombatRound必须先调用resetRoundGuards,
// 再调用本函数, 从而让防御严格保持为回合级状态.
func (r *CombatRoom) activateRoundGuards(actions []*combatAction) {
	if r == nil {
		return
	}
	for _, action := range actions {
		if action == nil || !action.isGuard() {
			continue
		}
		state := r.stateByKey(action.unitKey)
		if state != nil && state.alive && !state.escaped {
			state.guard = true
		}
	}
}

// completeCombatRound结算skill.yaml当前开放的基础战斗动作.
//
// 当前仅处理攻击、防御、逃跑、待机、破除防御和连续攻击. 其他已配置技能
// 在动作解析阶段直接返回不支持错误, 不会进入本结算器.
func (r *CombatRoom) completeCombatRound(playerActions []*combatAction) {
	if r == nil || r.roundTimer == nil || r.random == nil {
		return
	}
	actions := append([]*combatAction(nil), playerActions...)
	actions = append(actions, r.enemyAIActions()...)

	r.resetRoundGuards()
	r.activateRoundGuards(actions)
	for _, action := range actions {
		if action != nil {
			action.actionValue = r.combatActionValue(action)
		}
	}
	sortCombatActionsByValue(actions)
	groups := r.buildCombatActionGroups(actions)
	actionByUnit := make(map[string]*combatAction, len(actions))
	for _, action := range actions {
		if action != nil {
			actionByUnit[combatUnitKeyMapKey(action.unitKey)] = action
		}
	}

	currentRound := r.round
	stepResults := make([]*combatStepResult, 0, len(actions)*2)
	var settlement *pb.CombatBattleSettlement
	for _, group := range groups {
		if settlement = r.battleSettlementIfFinished(); settlement != nil {
			break
		}
		if group.combo {
			r.executeCombo(group, actionByUnit, &stepResults)
		} else if len(group.actions) == 1 {
			action := group.actions[0]
			stepStart := len(stepResults)
			settlement = r.executeStandaloneAction(action, actionByUnit, &stepResults)
			actionSteps := stepResults[stepStart:]
			markCombatTopLevelEvent(
				actionSteps,
				combatActionTopLevelSources(action),
				combatActionTopLevelTargets(action, actionSteps),
			)
		}
		if settlement != nil {
			break
		}
	}
	if settlement == nil {
		settlement = r.battleSettlementIfFinished()
	}
	result := &pb.CombatRoundResultNotify{
		BattleId:   r.battleID,
		Round:      currentRound,
		EventList:  buildCombatProtocolEvents(stepResults),
		Settlement: settlement,
	}
	if settlement != nil {
		r.finishCombat(result)
		return
	}
	for _, participantKey := range r.participantOrder {
		participant := r.participant(participantKey)
		if participant == nil {
			continue
		}
		participant.account.sendClientRes(participant.gateway, uint32(pb.MsgID_CombatRoundResultNotify_CMD), xerror.Success.Code(), result)
	}
	r.clearRoundTimer()
	r.round++
	r.beginCombatRound()
}
func (r *CombatRoom) validatePhysicalState() error {
	if r == nil || r.random == nil {
		return fmt.Errorf("combat room random is not initialized")
	}
	for key, state := range r.unitStates {
		if state == nil || state.unit == nil || state.unit.GetAttribute() == nil || state.unit.GetAttribute().GetLevel() == 0 {
			return fmt.Errorf("combat unit state invalid: %s", key)
		}

	}
	return nil
}
