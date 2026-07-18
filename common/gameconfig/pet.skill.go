package gameconfig

import (
	"strings"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type PetSkillConfig struct {
	*xmap.MapMgr[uint32, *PetSkillEntry]
}

type PetSkillEntry struct {
	// ID 来自 skill[].id, 必须处于协议宠物技能资源ID段内, 并且在 pet.skill.yaml 内唯一.
	ID *uint32 `yaml:"id"`
	// Name 来自 skill[].name, 保留技能显示名称文本, server 不解析客户端资源.
	Name *string `yaml:"name"`
	// Description 来自 skill[].description, 保留技能说明文本, 可用于查询, 日志或后续业务展示下发.
	Description *string `yaml:"description"`
	// NoGuard非nil表示该技能使用8.5 PETSKILL_NoGuard处理器.
	// 百分比来自服务端共享配置, 不能由CombatSkillAction请求携带或覆盖.
	NoGuard *PetSkillNoGuardEntry `yaml:"noGuard"`
	// ContinuationAttack非nil表示该技能使用8.5 PETSKILL_ContinuationAttack处理器.
	// 段数来自服务端共享配置, 客户端只提交目标, 不得覆盖本次动作的计划段数.
	ContinuationAttack *PetSkillContinuationAttackEntry `yaml:"continuationAttack"`
	// GuardBreak非nil表示该技能使用8.5 PETSKILL_GuardBreak处理器.
	// 该处理器没有可配置数值, 空对象只负责把技能明确映射到破除防御结算流程.
	GuardBreak *PetSkillGuardBreakEntry `yaml:"guardBreak"`
	// GuardBreak2非nil表示该技能使用8.5 PETSKILL_GuardBreak2处理器.
	// 它与第一种破除防御是两个独立命令: 防御目标先获得1.3倍伤害修正再执行
	// 普通Guard减伤, 非防御目标获得0.7倍伤害修正, 不能复用GuardBreak字段.
	GuardBreak2 *PetSkillGuardBreak2Entry `yaml:"guardBreak2"`
	// WildViolentAttack非nil表示该技能使用8.5 PETSKILL_WildViolentAttack处理器.
	// 三个修正值来自服务端共享配置, 客户端只能提交攻击目标, 不能覆盖攻防或命中修正.
	WildViolentAttack *PetSkillWildViolentAttackEntry `yaml:"wildViolentAttack"`
	// AttackCrazed非nil表示该技能使用8.5 PETSKILL_AttackCrazed处理器.
	// 段数来自旧服option, 目标列表会在攻击开始前由服务端一次性随机生成;
	// 客户端只提交初始合法目标, 不能指定每一段的攻击对象或覆盖计划段数.
	AttackCrazed *PetSkillAttackCrazedEntry `yaml:"attackCrazed"`
	// AttackShoot非nil表示该技能使用8.5 PETSKILL_AttackShoot处理器.
	// 最小和最大段数来自旧服option的两个字段. 实际段数、忠诚度100时的强制
	// 8段判定、逐段睡眠和反击禁用都由战斗层执行, 客户端只能提交初始目标.
	AttackShoot *PetSkillAttackShootEntry `yaml:"attackShoot"`
	// TearDamage非nil表示该技能使用8.5 PETSKILL_BattleTearDamage处理器.
	// 客户端只提交攻击目标; 追加比例必须由服务端技能配置提供, 不能把旧伤比例
	// 放进CombatSkillAction. 服务端结算只读取已经加载并校验的技能配置.
	TearDamage *PetSkillTearDamageEntry `yaml:"tearDamage"`
	// Sars非nil表示该技能使用8.5 PETSKILL_Sars处理器.
	// “煞”状态、默认3次持续伤害、当前HP的10%扣除和60%邻位传播都是旧处理器
	// 固定规则, 不暴露为运营配置字段, 防止配置生成与8.5不同的毒煞变体.
	Sars *PetSkillSarsEntry `yaml:"sars"`
	// Sonic非nil表示该技能使用8.5 PETSKILL_Sonic处理器.
	// 同列前排目标、贯穿段独立命中以及防御减伤前的1/2伤害都是处理器固定规则,
	// 不暴露倍率或目标偏移配置, 防止运营配置生成与8.5不同的贯穿行为.
	Sonic *PetSkillSonicEntry `yaml:"sonic"`
	// Gyrate非nil表示该技能使用8.5 PETSKILL_Gyrate处理器.
	// 攻击修正来自旧服option; 同排5格、升序预选和逐目标完整AttackSeq属于
	// 固定处理器规则, 客户端只能提交初始目标, 不能提交目标列表.
	Gyrate *PetSkillGyrateEntry `yaml:"gyrate"`
	// Hector非nil表示该技能使用8.5 PETSKILL_Hector处理器.
	// option只负责提供攻%、敏%工作值修正. 麻痹状态号2、基础成功值60、
	// 严格小于比较和持续1次行动都是处理器硬编码规则, 不允许配置覆盖.
	Hector *PetSkillHectorEntry `yaml:"hector"`
	// Retrace非nil表示该技能使用8.5 PETSKILL_Retrace处理器.
	// 旧处理器中读取option攻%修正的代码已经被注释, 实际生效的79%追击判定和
	// 第二击120%攻击均在battle.c硬编码. 现代配置把这两个数值显式保存为处理器
	// 参数, 使不同技能ID可以复用同一套DODGE后追击逻辑.
	Retrace *PetSkillRetraceEntry `yaml:"retrace"`
	// Acupuncture非nil表示该技能使用8.5 PETSKILL_Acupuncture处理器.
	// 一次性受击反应和奇数伤害补偶沿用旧战斗流程; 攻击者伤害除数由服务器
	// 技能配置提供, 当前真实技能配置为2, 对应8.5的damage/2.
	Acupuncture *PetSkillAcupunctureEntry `yaml:"acupuncture"`
}

// PetSkillNoGuardEntry保存8.5不防守战法写入CHAR_WORKBATTLECOM3的三个原始百分比.
//
// DodgePercent对应高16位, 所以接受uint16范围. CounterPercent和CriticalPercent
// 分别对应低16位中的bit 8-15和bit 0-7, 必须限制在单字节范围. 当前三个有效技能
// 均使用小于128的正值; 仍保留完整字节范围, 让战斗层可以精确复刻源码在反击字节
// 大于127时直接乘-1的异常历史行为. CriticalPercent在当前8.5有效战斗代码中没有
// 读取点, 只保留配置和审计数据, 不得擅自加入暴击公式.
type PetSkillNoGuardEntry struct {
	DodgePercent    *uint32 `yaml:"dodgePercent"`
	CounterPercent  *uint32 `yaml:"counterPercent"`
	CriticalPercent *uint32 `yaml:"criticalPercent"`
}

// check只校验NoGuard处理器自身拥有的参数.
//
// 技能ID属于外层PetSkillEntry, 不复制到处理器参数中. configure调用本方法失败时
// 会统一包装技能ID, 因此这里可以专注字段完整性和取值范围.
func (p *PetSkillNoGuardEntry) check() error {
	if p.DodgePercent == nil {
		return errors.Errorf("不防守战法缺少 noGuard.dodgePercent %v", xruntime.Location())
	}
	if *p.DodgePercent > 0xFFFF {
		return errors.Errorf("不防守战法 noGuard.dodgePercent 超出uint16范围: value:%d %v", *p.DodgePercent, xruntime.Location())
	}
	if p.CounterPercent == nil {
		return errors.Errorf("不防守战法缺少 noGuard.counterPercent %v", xruntime.Location())
	}
	if *p.CounterPercent > 0xFF {
		return errors.Errorf("不防守战法 noGuard.counterPercent 超出uint8范围: value:%d %v", *p.CounterPercent, xruntime.Location())
	}
	if p.CriticalPercent == nil {
		return errors.Errorf("不防守战法缺少 noGuard.criticalPercent %v", xruntime.Location())
	}
	if *p.CriticalPercent > 0xFF {
		return errors.Errorf("不防守战法 noGuard.criticalPercent 超出uint8范围: value:%d %v", *p.CriticalPercent, xruntime.Location())
	}
	return nil
}

// PetSkillContinuationAttackEntry保存8.5连续攻击处理器从option读取的攻击段数.
//
// PETSKILL_ContinuationAttack把合法范围限制为1至10, 非法值回退1. 当前项目在配置
// 加载边界直接拒绝缺失或越界值, 尽早暴露配置错误, 不让生产战斗悄悄退化成普通单段.
// 每段伤害除数、独立命中/暴击及动作结束后的单一反击链属于战斗结算职责, 不在配置层
// 重复保存布尔开关, 避免同一个8.5处理器出现互相矛盾的组合配置.
type PetSkillContinuationAttackEntry struct {
	SegmentCount *uint32 `yaml:"segmentCount"`
}

// check校验连续攻击处理器需要的计划段数.
func (p *PetSkillContinuationAttackEntry) check() error {
	if p.SegmentCount == nil {
		return errors.Errorf("连续攻击缺少 continuationAttack.segmentCount %v", xruntime.Location())
	}
	if *p.SegmentCount < 1 || *p.SegmentCount > 10 {
		return errors.Errorf("连续攻击 continuationAttack.segmentCount 超出1至10范围: value:%d %v", *p.SegmentCount, xruntime.Location())
	}
	return nil
}

// PetSkillGuardBreakEntry显式标记8.5第一种破除防御处理器.
//
// 旧技能9000004的option为空, 所以这里不保存攻击倍率等并不存在的参数. 目标是否正在
// 防御、是否跳过Guard减伤、非防御目标强制MISS及后续反击入口都依赖本回合运行态,
// 必须由战斗层在技能实际执行时判断, 不能提前固化成配置布尔值.
type PetSkillGuardBreakEntry struct{}

// check保留统一的处理器参数校验入口.
//
// 第一种破除防御当前没有数值参数, 非nil空对象本身就是完整配置.
func (p *PetSkillGuardBreakEntry) check() error {
	return nil
}

// PetSkillGuardBreak2Entry显式标记8.5第二种破除防御处理器.
//
// 旧服PETSKILL_GuardBreak2的option为空, 1.3和0.7是处理器本身的固定规则,
// 不是运营配置参数. 这里只保留空对象标记, YAML和CombatSkillAction都没有倍率字段. 目标是否
// Guard以及混乱是否取消Guard效果都属于执行时运行态, 必须留在战斗层判断.
type PetSkillGuardBreak2Entry struct{}

// check保留统一的处理器参数校验入口.
//
// 第二种破除防御当前没有数值参数, 非nil空对象本身就是完整配置.
func (p *PetSkillGuardBreak2Entry) check() error {
	return nil
}

// PetSkillWildViolentAttackEntry保存8.5狂暴攻击处理器从option解析出的三个数值.
//
// AttackPercentModifier和DefensePercentModifier不是最终百分比, 而是分别加到固定
// 攻击力和固定防御力上的有符号修正. 例如+200会使最终攻击力成为基础值的300%,
// -60会使最终防御力成为基础值的40%. TargetDodgePercentBonus对应旧服COM3高位:
// 它在本技能每一段攻击的BATTLE_DuckCheck中加到目标闪避率, 所以正数表示本技能
// 更容易未命中, 不是给施术者增加闪避或敏捷.
//
// 随机3至10段是PETSKILL_WildViolentAttack处理器的固定规则, 不属于option参数,
// 因此不在配置中重复保存上下限, 避免把不可变的8.5行为误暴露成可调数值.
type PetSkillWildViolentAttackEntry struct {
	AttackPercentModifier   *int32 `yaml:"attackPercentModifier"`
	DefensePercentModifier  *int32 `yaml:"defensePercentModifier"`
	TargetDodgePercentBonus *int32 `yaml:"targetDodgePercentBonus"`
}

// check校验狂暴攻击处理器需要的攻防和目标闪避修正.
func (p *PetSkillWildViolentAttackEntry) check() error {
	if p.AttackPercentModifier == nil {
		return errors.Errorf("狂暴攻击缺少 wildViolentAttack.attackPercentModifier %v", xruntime.Location())
	}
	if p.DefensePercentModifier == nil {
		return errors.Errorf("狂暴攻击缺少 wildViolentAttack.defensePercentModifier %v", xruntime.Location())
	}
	if p.TargetDodgePercentBonus == nil {
		return errors.Errorf("狂暴攻击缺少 wildViolentAttack.targetDodgePercentBonus %v", xruntime.Location())
	}
	return nil
}

// PetSkillAttackCrazedEntry保存8.5狂乱暴走处理器从option读取的攻击段数.
//
// PETSKILL_AttackCrazed没有像连续攻击那样在处理器内限制1至10, 但目标数组固定为
// BATTLE_ENTRY_MAX*2, 而8.5真实技能表中的AttackCrazed最大配置就是20. 现代配置
// 因此明确接受1至20并拒绝越界, 避免复刻旧服可能写越目标数组的内存错误.
//
// 攻击力变为FIXSTR*0.8、防御力变为FIXTOUGH*0.7以及每段不使用gDamageDiv,
// 都是处理器固定行为, 不暴露为运营可调参数, 防止配置出与8.5不同的技能变体.
type PetSkillAttackCrazedEntry struct {
	SegmentCount *uint32 `yaml:"segmentCount"`
}

// check校验狂乱暴走处理器需要的计划段数.
func (p *PetSkillAttackCrazedEntry) check() error {
	if p.SegmentCount == nil {
		return errors.Errorf("狂乱暴走缺少 attackCrazed.segmentCount %v", xruntime.Location())
	}
	if *p.SegmentCount < 1 || *p.SegmentCount > 20 {
		return errors.Errorf("狂乱暴走 attackCrazed.segmentCount 超出1至20范围: value:%d %v", *p.SegmentCount, xruntime.Location())
	}
	return nil
}

// PetSkillAttackShootEntry保存8.5丢栗子处理器从option解析出的随机段数闭区间.
//
// 旧服PETSKILL_AttackShoot先执行RAND(min,max), 再根据有效忠诚度和当前HP决定
// 是否强制覆盖为8段. 因此配置只保存原始上下界, 不能直接保存最终段数或把8段
// 特例改成运营开关. BATTLE_TargetListSet使用固定20槽目标数组, 但旧数据另有30段、
// 60段配置会越界写内存; 现代服务端明确限制1至20, 不复刻未定义的内存破坏行为.
type PetSkillAttackShootEntry struct {
	MinSegmentCount *uint32 `yaml:"minSegmentCount"`
	MaxSegmentCount *uint32 `yaml:"maxSegmentCount"`
}

// check校验栗子连激处理器的随机段数闭区间.
func (p *PetSkillAttackShootEntry) check() error {
	if p.MinSegmentCount == nil {
		return errors.Errorf("栗子连激缺少 attackShoot.minSegmentCount %v", xruntime.Location())
	}
	if p.MaxSegmentCount == nil {
		return errors.Errorf("栗子连激缺少 attackShoot.maxSegmentCount %v", xruntime.Location())
	}
	minSegmentCount := *p.MinSegmentCount
	maxSegmentCount := *p.MaxSegmentCount
	if minSegmentCount < 1 || minSegmentCount > 20 {
		return errors.Errorf("栗子连激 attackShoot.minSegmentCount 超出1至20范围: value:%d %v", minSegmentCount, xruntime.Location())
	}
	if maxSegmentCount < minSegmentCount || maxSegmentCount > 20 {
		return errors.Errorf("栗子连激 attackShoot.maxSegmentCount 必须处于minSegmentCount至20范围: min:%d max:%d %v", minSegmentCount, maxSegmentCount, xruntime.Location())
	}
	return nil
}

// PetSkillTearDamageEntry保存8.5撕裂伤口处理器option中的已损失生命追加比例.
//
// MissingHPPercent不是“普通物理伤害倍率”. 旧服先完成一次AttackSeq, 再取目标
// maxHP-currentHP, 按C float乘以本字段百分比后向零截断, 最后才把结果追加到
// AttackSeq伤害. 当目标没有损失生命时, 旧服会把整次伤害强制改为0; 该特殊
// 结果属于处理器固定行为, 不能由配置关闭.
//
// 8.5完整技能表中该处理器的有效值范围为20至450. 加载器接受1至450, 既覆盖
// 全部原始技能, 又拒绝0、负数无法由uint32表达的输入和超出原表的异常倍率.
// 攻击90%、防御80%、DODGE不追加及骑乘目标缺血合计不重复写入配置.
type PetSkillTearDamageEntry struct {
	MissingHPPercent *uint32 `yaml:"missingHpPercent"`
}

// check校验撕裂伤口处理器使用的目标缺失生命追加比例.
func (p *PetSkillTearDamageEntry) check() error {
	if p.MissingHPPercent == nil {
		return errors.Errorf("撕裂伤口缺少 tearDamage.missingHpPercent %v", xruntime.Location())
	}
	missingHPPercent := *p.MissingHPPercent
	if missingHPPercent < 1 || missingHPPercent > 450 {
		return errors.Errorf("撕裂伤口 tearDamage.missingHpPercent 超出1至450范围: value:%d %v", missingHPPercent, xruntime.Location())
	}
	return nil
}

// PetSkillSarsEntry显式标记8.5毒煞蔓延处理器.
//
// 旧技能617的option只有状态字“煞”和通用目标范围, 没有可调的伤害、概率或回合数.
// PETSKILL_Sars内部把初始持续值固定为3, BATTLE_StatusSeq再按SARS状态号执行
// 10%当前生命伤害与60%传播. 因此这里只保留空对象, 所有固定规则由战斗层实现.
type PetSkillSarsEntry struct{}

// check保留统一的处理器参数校验入口.
//
// 毒煞蔓延当前没有数值参数, 非nil空对象本身就是完整配置.
func (p *PetSkillSarsEntry) check() error {
	return nil
}

// PetSkillSonicEntry显式标记8.5音波冲击处理器.
//
// 旧技能618的option为空. PETSKILL_Sonic只记录玩家选择的攻击目标, 实际战斗分派
// 在目标位于后排5至9号位时额外攻击同阵营position-5的前排单位. 第二段重新执行
// 完整AttackSeq, 并在普通/暴击基础伤害完成后、GuardAdjust之前把C int伤害乘0.5.
// 因此这里只保留空对象标记, 贯穿位置与伤害倍率不作为YAML或请求字段.
type PetSkillSonicEntry struct{}

// check保留统一的处理器参数校验入口.
//
// 音波冲击当前没有数值参数, 非nil空对象本身就是完整配置.
func (p *PetSkillSonicEntry) check() error {
	return nil
}

// PetSkillGyrateEntry保存8.5回旋攻击处理器从option解析出的攻击百分比修正.
//
// 旧技能619的`攻%-20`表示WORKATTACKPOWER=FIXSTR+int(FIXSTR*-0.20),
// 最终为基础攻击的约80%. sscanf、除100和乘法均使用C float, 战斗层必须保留
// float32舍入. 这里只保存原始有符号修正; 同排范围和攻击顺序不允许配置覆盖.
type PetSkillGyrateEntry struct {
	AttackPercentModifier *int32 `yaml:"attackPercentModifier"`
}

// check校验回旋攻击处理器使用的攻击百分比修正.
func (p *PetSkillGyrateEntry) check() error {
	if p.AttackPercentModifier == nil {
		return errors.Errorf("回旋攻击缺少 gyrate.attackPercentModifier %v", xruntime.Location())
	}
	return nil
}

// PetSkillHectorEntry保存8.5威吓攻击处理器从option解析出的攻敏百分比修正.
//
// 旧技能620的`攻%-30 敏%-30`分别执行:
//
//	WORKATTACKPOWER = FIXSTR + int(FIXSTR * -0.30)
//	WORKQUICK = FIXDEX + int(FIXDEX * -0.30)
//
// sscanf、除100和乘法都使用C float, 战斗层必须逐步保留float32舍入. 麻痹概率
// 和回合数不是option可调参数, 因此本结构只保留这两个有符号修正值.
type PetSkillHectorEntry struct {
	AttackPercentModifier  *int32 `yaml:"attackPercentModifier"`
	AgilityPercentModifier *int32 `yaml:"agilityPercentModifier"`
}

// check校验威吓攻击处理器使用的攻击和敏捷百分比修正.
func (p *PetSkillHectorEntry) check() error {
	if p.AttackPercentModifier == nil {
		return errors.Errorf("威吓攻击缺少 hector.attackPercentModifier %v", xruntime.Location())
	}
	if p.AgilityPercentModifier == nil {
		return errors.Errorf("威吓攻击缺少 hector.agilityPercentModifier %v", xruntime.Location())
	}
	return nil
}

// PetSkillRetraceEntry保存8.5追迹攻击处理器的追击阈值和第二击攻击修正.
//
// 旧技能621虽然在数据表写有`攻%+20`, 但PETSKILL_Retrace内解析该option并提前
// 修改攻击力的整段代码已经被注释. 真正行为位于通用攻击主循环: 第一击严格返回
// DODGE时才执行`RAND(1,100)<80`; 成功后硬编码把WORKATTACKPOWER改成FIXSTR
// 加20%, 并对同一个defNo再调用一次BATTLE_Attack. RetryThresholdExclusive保存
// 严格比较右值80, 不是实际成功率79; SecondAttackPercentModifier只应用于成功后
// 的第二击, 绝不能提前修改第一击攻击力.
type PetSkillRetraceEntry struct {
	RetryThresholdExclusive     *uint32 `yaml:"retryThresholdExclusive"`
	SecondAttackPercentModifier *int32  `yaml:"secondAttackPercentModifier"`
}

// check校验追迹攻击处理器的严格追击阈值和第二击攻击修正.
func (p *PetSkillRetraceEntry) check() error {
	if p.RetryThresholdExclusive == nil {
		return errors.Errorf("追迹攻击缺少 retrace.retryThresholdExclusive %v", xruntime.Location())
	}
	if *p.RetryThresholdExclusive < 1 || *p.RetryThresholdExclusive > 100 {
		return errors.Errorf("追迹攻击 retrace.retryThresholdExclusive 超出1至100范围: value:%d %v", *p.RetryThresholdExclusive, xruntime.Location())
	}
	if p.SecondAttackPercentModifier == nil {
		return errors.Errorf("追迹攻击缺少 retrace.secondAttackPercentModifier %v", xruntime.Location())
	}
	return nil
}

// PetSkillAcupunctureEntry显式标记8.5针刺外皮处理器.
//
// 旧技能622的option为空. PETSKILL_Acupuncture只提交专用命令, 主循环在单位实际
// 行动时把CHAR_WORKACUPUNCTURE写1, 随后贯穿到普通攻击. 首次受到正数、非投掷
// 物理伤害时, BATTLE_DamageSub先把奇数伤害加1, 让原目标承受补偶后的完整伤害,
// 再让攻击者承受damage/2并把工作槽清零. 现代服务端把这个除数放在共享技能
// 配置中并在提交动作时锁定; CombatSkillAction不携带或参与该数值计算.
type PetSkillAcupunctureEntry struct {
	AttackerDamageDivisor *uint32 `yaml:"attackerDamageDivisor"`
}

// check校验针刺处理器用于计算攻击者承伤的除数.
func (p *PetSkillAcupunctureEntry) check() error {
	if p.AttackerDamageDivisor == nil {
		return errors.Errorf("针刺外皮缺少 acupuncture.attackerDamageDivisor %v", xruntime.Location())
	}
	if *p.AttackerDamageDivisor == 0 {
		return errors.Errorf("针刺外皮 acupuncture.attackerDamageDivisor 必须大于0 %v", xruntime.Location())
	}
	return nil
}

func newPetSkillConfig() *PetSkillConfig {
	return &PetSkillConfig{
		MapMgr: xmap.NewMapMgr[uint32, *PetSkillEntry](),
	}
}

func (p *PetSkillConfig) load(dir string) error {
	var root struct {
		Skill []*PetSkillEntry `yaml:"skill"`
	}
	if err := loadYAMLFile(dir, FilePetSkill, &root); err != nil {
		return err
	}
	return p.configure(root.Skill)
}

func (p *PetSkillConfig) configure(entries []*PetSkillEntry) error {
	for _, skill := range entries {
		if skill.ID == nil {
			return errors.Errorf("宠物技能缺少 id %v", xruntime.Location())
		}
		if !isPetSkillID(*skill.ID) {
			return errors.Errorf("宠物技能ID超出范围: %d %v", *skill.ID, xruntime.Location())
		}
		if skill.Name == nil {
			return errors.Errorf("宠物技能缺少 name: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Name) == "" {
			return errors.Errorf("宠物技能 name 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if skill.Description == nil {
			return errors.Errorf("宠物技能缺少 description: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if strings.TrimSpace(*skill.Description) == "" {
			return errors.Errorf("宠物技能 description 不能为空: ID:%d %v", *skill.ID, xruntime.Location())
		}
		// 一个技能只能选择一个服务端处理器. GuardBreak、GuardBreak2、
		// ContinuationAttack、NoGuard、WildViolentAttack、AttackCrazed、
		// AttackShoot、TearDamage、Sars、Sonic、Gyrate、Hector、Retrace和
		// Acupuncture分别进入
		// 不同的动作、参数和结算流程; 同时配置
		// 会让解析顺序决定实际行为, 属于必须在配置加载阶段暴露的歧义, 不能由
		// 战斗代码静默选择.
		handlerCount := 0
		// 参数检查在各自处理器分支内执行, 但先暂存第一个错误. 这样同一技能错误地
		// 配置多个处理器时优先报告处理器歧义; 只有处理器唯一时才报告其字段错误.
		var handlerParameterError error
		if skill.NoGuard != nil {
			handlerCount++
			if err := skill.NoGuard.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.ContinuationAttack != nil {
			handlerCount++
			if err := skill.ContinuationAttack.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.GuardBreak != nil {
			handlerCount++
			if err := skill.GuardBreak.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.GuardBreak2 != nil {
			handlerCount++
			if err := skill.GuardBreak2.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.WildViolentAttack != nil {
			handlerCount++
			if err := skill.WildViolentAttack.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.AttackCrazed != nil {
			handlerCount++
			if err := skill.AttackCrazed.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.AttackShoot != nil {
			handlerCount++
			if err := skill.AttackShoot.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.TearDamage != nil {
			handlerCount++
			if err := skill.TearDamage.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Sars != nil {
			handlerCount++
			if err := skill.Sars.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Sonic != nil {
			handlerCount++
			if err := skill.Sonic.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Gyrate != nil {
			handlerCount++
			if err := skill.Gyrate.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Hector != nil {
			handlerCount++
			if err := skill.Hector.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Retrace != nil {
			handlerCount++
			if err := skill.Retrace.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if skill.Acupuncture != nil {
			handlerCount++
			if err := skill.Acupuncture.check(); err != nil && handlerParameterError == nil {
				handlerParameterError = errors.Wrapf(err, "宠物技能参数错误: ID:%d", *skill.ID)
			}
		}
		if handlerCount > 1 {
			return errors.Errorf("宠物技能不能同时配置多个战斗处理器: ID:%d %v", *skill.ID, xruntime.Location())
		}
		if handlerParameterError != nil {
			return handlerParameterError
		}
		if !p.AddIfNotExist(*skill.ID, skill) {
			return errors.Errorf("宠物技能ID重复: %d %v", *skill.ID, xruntime.Location())
		}
	}
	return nil
}

func (p *PetSkillConfig) check() error {
	return nil
}

func (p *PetSkillConfig) assemble() error {
	return nil
}
