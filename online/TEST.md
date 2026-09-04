# Online 服务测试指南

## 适用范围

修改 Account actor、共享配置加载、角色、宠物、商店或邮箱业务、`CombatRoom`、回合动作、gateway stream 路由或 online gRPC handler 时使用本文档.

## 快速检查

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./online
```

涉及 gateway、cache、login 或协议契约时追加:

```bash
GOCACHE="$PWD/.gocache" go test ./gateway ./cache ./login
```

## 配置检查

`custom.gameConfigDir` 必须包含:

```text
character.yaml
skill.yaml
ai.yaml
enemy.group.yaml
enemy.exp.yaml
exp.yaml
item.yaml
item.weapon.yaml
item.accessory.yaml
reward.yaml
task.yaml
pet.yaml
scene/*.yaml
```

验证要点:

- 缺少任一必需文件时, online 明确启动失败.
- `skill.yaml` 的 ID、名称、说明、可选非负 `cost` 和连续攻击段数非法时加载失败; `cost: 0` 必须仍被识别为可学习.
- `mightyAttack` 必须为对象, 同时提供整数倍率1-655和目标闪避加值0-32767; 空值、缺项、小数、越界和与 `continuationAttack` 并存均加载失败.
- `poisonAttack` 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0); 与 `continuationAttack` 或 `mightyAttack` 并存, 空值, 缺项, 字符串, 小数及越界均加载失败. 8100061必须保持5次, -30%, 4000石币.
- `item.yaml` 只允许普通道具分组, `item.weapon.yaml` 只允许武器分组, `item.accessory.yaml` 只允许首饰分组, 三者必须使用 `items.<group>.<id>` 结构并合并为统一运行期索引; 文件缺失、分组放错文件、未知分组, 空普通道具或武器分组、ID超出分组区间、非法 `atlas` 路径、武器分组混用图集、15项固化数值范围或攻击次数范围倒置、非法职业、非法武器类型或非法元素配置均应加载失败.
- 普通道具 `sprite` 为0时不能配置 `atlas`, `sprite` 大于0时必须配置以 `item/` 开头的无扩展名 `atlas`; 武器和首饰还必须使用非零 `sprite` 且不能配置 `use`. 首饰允许空发布组, 有条目时必须配置合法 `accessory_type`, 并与 proto 子区间相符.
- `pet.yaml skill` 的非 0 ID 不存在于 `skill.yaml` 时加载失败.
- `ai.yaml` 的 ID 必须为非零唯一整数. `skills[]` 必须非空, 技能 ID 必须存在且不重复, 每项必须显式提供正整数权重, 单项和总权重不超过2147483647且总权重大于0; 目标范围和目标策略保持原有校验. 权重为0或缺失时必须加载失败, 列表允许超过7项.
- `pet.yaml creationMode` 只允许省略或填写 `fusionEgg`. 普通宠物应用品阶偏移后的 `SavedBase*` 和原版公式计算出的 `Raw*` 使用 `int32`, 允许为0或负数; protobuf 往返、档案绑定和战斗构造不得发生无符号下溢. 派生后的最大生命和攻击仍必须大于0. 融合蛋允许保留原版占位成长, 但 `common/pet.NewRecord` 和 GM 普通创建入口必须拒绝, 且拒绝时不得修改 UUID 游标或角色宠物列表.
- `scene/*.yaml` 不得设置格式版本字段; 地图 ID 必须是客户端正整数 `map_id`, 地图尺寸和 `collision.blockedRows` 必须合法. 当前配置目录包含阿布洞窟10007, 80000、80001、80010、80020、80030、80040、80050、80060、80070这9张测试地图, 90001和90010至90130按10递增这14张练级地图, 以及1000701至1000703这3张任务地图; `CharacterMapEnterReq` 必须接受已配置且遇敌有效的测试、练级或任务地图, 并拒绝范围外的10007.
- 阻挡区、传送起点越界或已存在目标地图的落点越界时加载失败.
- `encounter.enabled` 必须设置; 启用遇敌时 `encounter.enemyGroups` 必须引用至少一个已存在的敌人组, 总权重必须大于0.
- `enemy.group.yaml enemies[].id` 必须引用存在的宠物模板; 每个敌人的 `battleAI` 必填且引用存在的 AI. 缺失、0或未知 AI 均加载失败. `pet.yaml` 可以独立于 AI 表加载, 其技能只用于出生模板.
- 旧 `pet.yaml battleAI`、`enemies[].skill`、`attackWeight`、`defenseWeight`、`escapeWeight` 和 `skillSlotWeights` 必须明确报错. AI 中不支持的 NPC 技能必须使 Online 在注册 etcd 和 gRPC 前启动失败.
- 90001和90010至90130的14个练级组共30个敌人必须引用 AI 1, 攻击、防御、逃跑权重保持 `10:1:1`. 85个普通敌人引用 AI 18, 攻击、防御权重保持 `10:1`.
- 多数宠物出生技能保持 `[8000001,8000002,0,0,0,0,0]`, 查罕·乌尔夫和查罕·吉鲁的出生技能继续包含 `8100003`. 两个守护者敌人引用 AI 19, 仅按 `10:1` 权重攻击或防御.
- 配置加载失败后不注册 etcd, 不启动 gRPC 业务入口.

## 敌人成员等级

- Boss 成员必须且只能配置 `level` 或 `levelRange`; 普通组成员允许省略二者并使用组级规则, 但同样拒绝同时配置.
- 验证固定等级、闭区间及单点区间加载成功; 缺失Boss等级、互斥字段同时出现、范围倒置、元素数量错误和协议等级越界必须失败.
- 五名阿布洞窟守护者必须保持原顺序, 只配置原版 `[39,41]`、`[38,40]`、`[37,39]`、`[35,38]`、`[40,43]`, 不保留固定 `level`.
- 建房等级测试验证成员范围的两个端点、每次创建独立抽取、成员范围优先于组级范围, 并保留固定等级不消耗随机数和玩家等级偏移边界检查.

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run 'EnemyGroupMemberLevelModes|EnemyGroupProjectGuardianLevelRanges|CombatPVEEnemyLevel' -count=1
```

## 角色任务

聚焦验证命令:

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb -run 'Task|Reward' -count=1
```

- task.yaml必须校验正整数唯一任务ID、从1连续编号的非空步骤、非空完成条件、合法条件字段及现有任务/道具/宠物/敌群/奖励包引用. reward.yaml允许空rewards数组, 非空奖励包必须至少包含合法且不重复的道具/装备或宠物; 所有数量为正, 奖励宠物必须配置合法等级和`grade: random`.
- completionMode省略时为automatic. consumeItems和consumePets仅允许submit; itemPossession与petPossession只检查持有, 后者按宠物ID和实际等级精确匹配. taskRewardsClaimed要求前置任务全部步骤完成且奖励全部领取. battleVictory不能用于接取、开始或submit完成条件.
- 阿布洞窟60必须是非主线、无接取条件、单步骤、enemyGroupId 1000701、rewardId 0. 其他敌群胜利、失败或未接取均不得完成.
- 卡坦任务61必须按两批扣除4只25级宠物, 第一批领取火难的戒指, 第二批交回戒指并领取1级随机品质修宝. 任务62必须以任务61全部领奖为前置, 按两批扣除另外4只25级宠物并领取1级随机品质朵拉比斯.
- 多任务并行, 单任务步骤串行. 无奖励步骤完成后completed_at_ms与reward_claimed_at_ms相同, 后补奖励不产生历史可领取状态.
- 步骤奖励独立领取, 重复领取必须拒绝. 背包满、宠物栏满、数量溢出、UUID耗尽、扣除不足、Cache失败必须保留原任务、完整库存和UUID游标.
- 提交和领奖成功后, `CharacterTaskInventoryChangedNotify`必须携带完整角色背包、随身宠物、角色资产和账号UUID游标; 客户端先原子应用该库存快照, 再合并任务记录增量. 任务记录增量只含变化任务ID. 任务/步骤数量、时间顺序或串行状态非法时拒绝绑定.
- 非循环任务完成后仍拒绝重复接取. 任务62最后一步领奖后必须重置接取时间和全部步骤记录并自动开始新一轮首步, 同一轮奖励不得重复领取.
- PVE胜利进度由服务端结算调用, 不依赖客户端战斗结束消息; 持久化失败必须和经验一同回滚.

任务挑战与重打验证:

- `TaskBattleChallengeReq/Res`使用`0x007007/0x007008`, 请求只携带角色UUID, 任务ID和步骤ID. 服务端按所选步骤读取配置敌群, 已开始的步骤可挑战, 已完成步骤也可再次挑战; 未接取, 未开始, 未知任务或步骤以及非法ID必须拒绝, 且不得修改任务记录.
- `TestCharacterTaskChallengeUsesConfiguredStartedStep`覆盖串行任务中已完成步骤的重打与当前步骤挑战. `TestCharacterTaskCompletedChallengePreservesRecordsAndRewards`覆盖无奖励, 待领取和已领取三种完成状态, 连续重打后任务记录, 领奖状态和道具持有量不变, 不产生重复任务增量.
- `TestCharacterTaskCombatPersistenceAndRecordValidation`覆盖真实战斗持久化入口: 重打仍增加正常战斗经验, 已完成任务的完成时间和领奖时间不变, `changedTaskRecordMap`不包含该任务. 重打不是重新接取或循环任务, 不解除已有的重复领奖保护.

## 角色声望

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./online -run 'TestApply(Character|Pet)Experience|TestPersistCombatParticipantResultWritesExperienceAndRollsBackAtomically|TestValidateAccountRecord' -count=1
```

- `CharacterBaseRecord.reputation` 使用内部整数值, `100` 表示显示 `1.00`, 最大值为 `100000000`. 上限由 Online 校验和封顶, Cache 不执行声望业务规则.
- 人物从旧等级跨到每个新等级 `L` 时, 分别结算 `GetLevelMinExp(L) / 20000`. 小于门槛不增加声望, 精确达到门槛和超过门槛都必须结算.
- 人物连续跨越多级时必须使用每个新等级的原版表值, 不得把最终等级的声望重复乘以升级数. 接近上限时只增加到上限.
- 宠物到达30级及以下不增加主人声望; 从30级到31级开始, 每个新等级 `L` 使用 `GetLevelMinExp(L-1) / 20000`.
- 宠物主人缺失时必须在修改宠物经验和成长前拒绝. 宠物不持有声望字段, 也不得把奖励增加给其他角色.
- 战斗中的宠物经验, 成长和主人声望必须共用一次 Cache 写入并支持完整回滚. 宠物经验道具增加主人声望时必须同时标记角色基础变化和宠物变化.
- Online 账号档案校验接受恰好等于上限的声望, 拒绝超过上限的声望.

## 全局装备商店购买

- `ShopPurchaseReq` 和 `ShopPurchaseRes` 的消息 ID 分别保持为 `0x003007` 和 `0x003008`.
- 仅允许购买武器或首饰 ID 且对应配置 `cost > 0` 的条目, 并按服务端当前配置价格结算; `cost = 0`, 非武器或首饰 ID, 首饰类型与 ID 不符和数量为0必须拒绝.
- 角色必须属于当前账号、已经在线且不在战斗中. `asset_count_map[AssetID_Stone]` 不足、背包满、数量超过剩余格数或 UUID 游标耗尽时不得修改账号档案.
- 成功购买多件装备时, 新装备 UUID 必须从旧 `used_uuid + 1` 连续递增. 每件记录除 `uuid` 和 `asset_id` 外, 必须完整保存15个 `EquipmentRecordBase` 固化 key; 每项值在对应配置闭区间内且不同实例分别创建, 并一次性扣除总价.
- cache 持久化失败必须保留原角色资产、背包和 UUID 游标; 成功响应的 `affected_item` 必须返回 `AssetID_Stone` 和扣除后的最终持有数量, 同时返回最新游标和完整的本次新增装备列表.

角色道具与独立资产还必须验证:

- 普通道具通过统一管理器写入背包并受容量限制, `[3490000,3499999]` 通过同一接口写入 `asset_count_map` 且不初始化背包.
- 角色资产扣减至 0 时删除键, 查询缺失键返回 0, 增加后不得超过 `math.MaxInt64`.
- 角色资产禁止在角色背包和账号仓库中出现, 仓库存取请求必须拒绝该范围.
- cache 档案中的角色资产 ID 越界、数量超限或缺少统一道具配置时, `validateAccountRecord` 必须拒绝.

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./online -run ShopPurchase
```

## 角色装备换装

- `CharacterEquipmentReplaceReq` 和 `CharacterEquipmentReplaceRes` 的消息 ID 分别保持为 `0x00100F` 和 `0x001010`; 当前接受 `EquipmentType_Weapon`, `EquipmentType_Accessory1` 和 `EquipmentType_Accessory2`, `equipment_uuid=0` 表示卸下.
- 只允许当前账号中已上线且不在战斗中的角色换装. 装备 UUID 必须位于该角色背包, 记录必须恰好含15个固化 key 且值位于当前配置闭区间; 缺项、多项、额外 key、UUID/配置不匹配均应拒绝.
- 装备时校验角色等级. 当前角色没有职业档案字段, 所有 `neprof != 0` 的装备都必须明确拒绝, 不得把未知职业当成满足要求.
- 替换时新装备进入目标部位, 旧装备回到背包; 卸下时目标部位装备回到背包且必须有剩余容量. 计划构造和 cache 持久化失败不得修改原背包、穿戴记录或运行中角色引用.
- 成功响应必须同时返回完整 `item_bag`、本次替换部位的 `equipment` 和同一候选档案计算出的 `effective_attribute`; 卸下时 `equipment` 不设置. 连续请求由 Account actor 按收到顺序串行处理, 不依赖客户端禁止重复操作.
- 账号校验必须覆盖背包、仓库和已穿戴武器及两个首饰位之间的 UUID 全局唯一性, 并拒绝尚未开放的六个防具部位.
- 两个首饰位允许六类不同类型的全部组合, 同类型即使 ID 和 UUID 不同也必须拒绝. 满背包允许同一部位的原子替换, 卸下仍要求空位. 原枚举值1和3及档案字段 tag 1和3必须保持不变.
- 运行 `GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run TestAccessory -count=1` 验证类型和 ID 边界, 穿戴互斥, 购买实例, 持久化失败, 属性叠加和战斗快照.

## 统一技能测试

角色:

- `8000001` 生成普通攻击动作并校验敌方目标.
- `8000002` 生成以自身为目标的防御动作.
- `8000003` 生成逃跑动作.
- `8000004` 生成捕获动作, 缺少目标、同阵营目标或未知目标必须拒绝. 捕获不开放给玩家宠物和敌方 NPC.
- `8000005-8000007` 已配置但未开放, 必须返回业务错误且不写入本回合动作.
- `skill.yaml` 不存在的 ID 必须拒绝.

宠物学习与档案:

- 20 个开放宠物技能的 `cost` 必须逐项等于原版 8.0 `petskill2.txt` 基础价格; 一击必杀40-42映射 `8100040-8100042`, 价格为2500, 25000, 25000; 突击30/31价格为4000/8000, 猛毒攻击61为4000, 手下留情626为10000. 原版 8.5 服务端源码用于校验 `PETSKILL_COST` 的权威读取与扣款流程, 未配置 `cost` 的技能不可学习.
- 新宠物必须从 `pet.yaml skill` 深拷贝恰好 7 个实例槽位; 修改实例不得污染模板. 旧 cache 缺槽、槽位数不为 7 或引用未知技能时账号绑定失败.
- 学习和替换按新技能基础价扣除石币, 不退款、不抵扣旧技能价格; `skill_id=0` 免费遗忘且石币不变.
- 宠物技能和石币必须在同一 `AccountRecord` 候选档案中一次持久化; cache 失败、余额不足、非法槽位、非随身宠物、离线或战斗中请求都不得产生部分修改.
- `PetSkillSetReq/Res` 消息 ID、字段及 protobuf 往返保持稳定.

玩家宠物战斗:

- 技能必须同时存在于 `skill.yaml` 和该玩家宠物开战时的 `CombatUnit.skill_id_list` 快照.
- `8000001`、`8000002` 分别生成攻击和防御动作.
- 测试配置分配后, `8100000`, `8100003`, 连续攻击, 一击必杀, 猛毒攻击, 突击和手下留情应由统一解析器生成对应动作.
- `8100000` 不需要目标, 必须生成携带技能ID和来源单位、但没有效果的实际动作步骤, 不得伪造成未执行动作.
- `8100003` 对有效Guard目标必须跳过GuardAdjust及其随机数, 保留普通物理防御、元素和暴击公式并且不返回Guard结果; 对非Guard目标仍先消费原版AttackSeq随机链, 再把Dodge、Critical和伤害统一覆盖为0伤害MISS.
- `8100003` 不得清除目标Guard、参加合击、成为反击者或触发受击目标反击. 运行 `go test ./online -run 'TestCombat(GuardBreak|Standby)' -count=1` 覆盖成功、失败、随机数和动作步骤.
- 一击必杀只生成一个主动步骤, 三档倍率必须作用于暴击、Guard和最低伤害后的结果, 保持C float舍入、0伤害MISS和普通暴击表现. 30/40/50点闪避先加再按基础75%封顶, Guard跳过闪避, 后置装备闪避继续独立判定.
- 一击必杀不得参加合击或消费合击资格随机; 行动前不能反击, 行动后可以反击. 后续反击必须与普通Attack的伤害、闪避和随机序列一致, 同时保留声明技能ID. 参数及技能归属使用冻结快照, 非持有者、非法目标和角色指令必须拒绝.
- 突击30/31保留原版描述及4000/8000学习价格, 验证 `chargeAttack` 必填整数, 1-10/0-32767边界和五种攻击机制互斥. 玩家宠物和NPC均须持有技能, 角色和非法目标必须拒绝; 提交后修改请求对象或配置不得改变蓄力参数及目标.
- 突击前1/2次行动只蓄力, 不提前命中或消费攻击随机; 第2/3次行动的单次物理结果与基础攻击力按原版C double公式修正后的普通攻击完全一致, 包括随机抽取次数. 释放读取当时基础攻击力, 目标失效只在释放时重选, NPC后续回合不得再次抽AI.
- 突击不参加合击, 自身等待和释放时都不能反击; 目标按普通资格和概率反击. 普通非致死伤害保留蓄力, 每次行动前正常处理一次毒伤, 死亡和离场清理续招. 玩家角色全灭仍正常结束战斗.
- 真实请求和序列化战报应覆盖首次选招确认, 后续宠物自动锁定, 角色仍需选招, 重复请求拒绝, 超时保留续招, 旧超时回调失效, 释放后的下一回合恢复选择. `next_round_auto_action_unit_key_list` 必须包含剩余计数为0但尚未释放的宠物, 不包含释放完成的宠物.
- `go test -buildvcs=false ./common/gameconfig ./online -run Charge -count=1` 覆盖突击配置, 结算和跨回合协议; `python -X utf8 -m unittest tool.test_skill_catalog` 检查目录30/31映射及原描述.

- 手下留情626保留原版描述及10000学习价. `showMercy` 必须为空对象, 拒绝null, 布尔, 数组, 字符串, 未知参数和其他四种机制并存. 已学习宠物和显式配置NPC使用同一解析器, 冻结技能归属, 拒绝角色指令和非法目标.
- 手下留情覆盖非致死, 恰好致死, 过量伤害, 1HP目标, 暴击, Guard, 闪避及最低伤害0/1. 与同种子的普通物理比较随机抽取次数; 只有致死伤害被限制, Normal/Critical/Guard不因限伤成0而丢失. 原始伤害0不保留暴击, 有效Guard对应ALLGUARD. 保留1HP时不倒地, 不击飞, 不清理中毒或累计过量伤害; 后续普通攻击仍能击杀.
- 手下留情不得参加合击或消费合击资格随机; 使用者行动前后不能反击, 目标仍可反击. `go test -buildvcs=false ./common/gameconfig ./online -run ShowMercy -count=1` 覆盖上述服务端边界, `python -X utf8 -m unittest tool.test_skill_catalog` 检查626映射及生成稳定性.

- 猛毒攻击验证玩家宠物和NPC的技能归属及冻结参数, 拒绝角色施放和非法目标. 主动攻击力101按原版负修正截断为71, 当回合反击保留减攻但不附毒, 下一回合恢复.
- 猛毒附加概率验证等级差上下限, 幸运, 毒抗, 基础体力比例和严格小于边界; 已中毒不刷新且不抽数, MISS或0伤害不进行附毒概率判定.
- 普通毒伤验证人物整点和宠物100倍固定点的一致结果, 先求和再截断, 最低1点伤害, HP最低留1, HP=1时Damage 0, 连续5次扣血且第5次Damage同时Remove; 单次持续和HP=1的最后一次也必须立即解除, 下一次行动不产生额外解除步骤. 固定先后手下, 最后一次毒伤前禁止重复附毒, 之后允许再次附毒. 连击只扣一次, 反击不扣, 合击每名成员的毒伤必须在成员攻击前出现. 状态步骤不得污染顶层声明目标, 包括原本无目标的待机.
- `go test ./common/gameconfig ./online -run Poison -count=1` 覆盖上述猛毒链路及现有protobuf序列化往返, 同时检查死亡或离场不继续扣血, 物理致死时清理中毒.
- 未持有技能即使存在于 `skill.yaml` 也必须拒绝.
- 未实现行为即使已配置并持有也必须拒绝, 不允许回退成普通攻击、防御或待机.
- 玩家宠物快照必须来自 `PetRecord.skill_id_list`; 开战后修改模板或档案不得改变本场单位技能.

敌方 NPC 战斗:

- 敌人技能、权重和目标策略全部来自 `enemies[].battleAI` 引用的 AI, 并在建房时深拷贝到服务端运行态. 同一宠物模板的多个敌人可使用不同 AI, 修改全局模板、AI或另一个敌人的快照不得影响本敌人.
- 敌方 `CombatUnit.skill_id_list` 必须为空, 客户端不接收敌人技能槽; AI 和技能归属校验只读服务端运行态.
- AI 按 `skills[]` 顺序划分权重区间, 一次 `RAND(0,totalWeight-1)` 选择技能, 单技能 AI 也必须恰好消费一次随机抽取; 区间边界及目标选择的随机抽数保持可复现. 逃跑动作和结果必须携带真实技能 ID `8000003`.
- 待机、破除防御、连续攻击和一击必杀通过显式技能 ID 与权重选择; 捕获、换宠、使用道具、更换装备及其他未实现 NPC 行为必须在启动阶段直接报错. 玩家宠物不得从敌人 AI 获得额外技能.

一击必杀聚焦验证:

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run 'TestSkillConfig|TestCombatMightyAttack|TestValidateEnemyCombatSkillConfig' -count=1
python -B -m unittest tool.test_skill_catalog
python -B tool/skill_catalog.py
```

目录生成与编辑器测试同时验证宠物39已移除, 重新生成不会恢复, 40-42映射及实现状态正确, 原始资料和其他来源的39不受影响.

错误响应:

- 非法动作返回 `CombatRoundActionRes` 对应错误码.
- gateway 不因 online 的普通业务错误码断开连接.
- 同一连接修正请求后仍可继续提交其他业务包.

## 角色交互设置

- `CharacterSettingSetReq` 和 `CharacterSettingSetRes` 的消息 ID 分别保持为 `0x001009` 和 `0x00100A`.
- 每次创建新的角色管理器时, 所有角色的组队和决斗状态都必须为关闭, 不继承上一次登录会话状态.
- 修改状态只更新 Account actor 当前会话内存, 不修改 `CharacterRecord`, 不调用 `CacheSetAccountRecord`.
- 角色 UUID 不属于当前账号或消息无法解析时必须返回业务错误, 不修改原状态.
- 客户端仅在成功回复后应用服务端返回值, 失败时保留请求前状态.

## NPC挑战

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run 'Scene.*NPC|SceneBattleChallenge|NPCInteraction|StartCombatPVE' -count=1
```

- `NpcInteractionReq` 和 `NpcInteractionRes` 的消息 ID 分别保持为 `0x006000` 和 `0x006001`; 挑战请求只提交当前账号角色 UUID、当前地图 NPC 实体 ID、option ID 和 `battle_challenge Start`, 不提交敌人组或 BGM.
- 服务端必须按角色当前运行态 scene 和 Presence 查找启用的 `BattleChallenge` option, 再从 `scene/<map_id>.yaml` 读取唯一 `enemyGroupId`; 客户端提交不存在、禁用、类型不符的 NPC option 均不得开战.
- `scene/10007.yaml` 的 entity 8、9、10 option 2 都引用敌人组1000701. 服务端 scene 配置不得包含 `presentation` 或 `battleBgmIndex`; BGM由客户端 `map.entity.yaml` 的 NPC表现配置决定.
- NPC挑战与自动遇敌必须复用同一PVE建房流程, 包括队长限制、队员入场、敌方生成、CombatRoom绑定和开战通知; 任一校验或建房失败不得留下角色CombatRoom指针.


## 基础队伍

聚焦测试命令:

```bash
go test ./online -run 'TestCharacterTeam|TestTryBindCombatRoom|TestCombatRoomDetach|TestCombatResultDischarges' -count=1
```

- `CharacterTeamOperationReq` 和 `CharacterTeamOperationRes` 必须恰好设置一个对应的 `join`、`leave`、`disband` 或 `kick` oneof 分支. 未设置、请求与回复分支不匹配、缺少完整 `join.target` 或 `kick.target` 必须拒绝; 四种成功回复的分支都必须为空结构.
- 加入目标只允许是请求中完整指定、位于同一非0地图、开启组队且不在战斗的未组队角色或实际队长; 加入不校验面对、朝向、格坐标或距离. 普通成员、地图0角色、关闭组队开关和战斗中角色必须拒绝.
- 加入和踢出目标按 `aid + character_uuid` 区分, 同一 aid 的不同角色 UUID 不得互相覆盖.
- 新成员追加到队尾; 普通成员离开或被踢后列表必须紧凑前移. 队伍只剩队长时自动删除, 队长主动解散或被强制移除时清除全部成员.
- 加入成功必须立即关闭新队员的自动遇敌并清除 timer; 普通队员后续提交开启或关闭请求都必须返回 `FailedPrecondition` 且保持关闭, 队长不受该限制.
- 战斗中禁止切换组队开关以及加入、解散和踢出. 普通成员主动离队必须成功且只改变队伍关系, 原 CombatRoom 指针和本场参战资格保持不变. 普通倒地不得解除队伍; 成功逃跑和角色 Ultimate 击飞必须解除, 宠物击飞不得误删角色队伍.
- 队长自动遇敌必须冻结同地图队伍顺序. 每名成员由所属 Account actor 在一次同步消息中完成读取、满 HP 快照和 CombatRoom 指针绑定; 无战宠合法. 成功成员按冻结顺序紧凑占用0至4号位, 战宠占用对应位置加5.
- 普通成员入场失败不得阻塞有效成员开战. 仍在线的失败成员仅在仍属于原队长当前队伍时被踢出; 已离线成员不要求踢队, 任何失败成员都不得进入 `CombatBattleStartNotify`.
- 手工联调2-5个在线角色: 目标在队伍页开启组队, 请求者输入完整目标 aid 和角色 UUID 后点击加入. 成功后新成员、队长、原队员和同地图无关角色都收到各自 UUID 的 `team_join`, 事件中的完整队伍顺序一致; 受影响角色另收各自 UUID 的 `CharacterTeamChangedNotify`. 验证离开、踢出和解散不弹确认框, 测试、练级或任务地图中的显示分组继续由地图事件独立更新.

## 测试、练级与任务地图角色列表

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./online -run "^(TestCharacterMap|TestScenePresenceCharacterMap|TestCharacterTeamMutationCarriesSmallMapEvents|TestSelectCombatPVEMapEnemyGroupUsesFlatEncounter)" -count=1
```

- `CharacterBaseRecord` 必须保留字段号18和字段名 `scene_id`, 但不得定义或持久化该字段. 测试、练级或任务地图 ID 只保存在 `character.sceneID` 运行态; 进入非0地图后离线必须归零并移除旧 Presence, 再次上线不得恢复旧地图, 且应允许重新进入同一地图. 单人从非0地图请求 `map_id=0` 时应将运行态设为0、从旧 Presence 移除、只向旧地图广播 `map_leave`, 并返回 `map_id=0` 和空分组列表; 地图0本身不得创建 Presence 或发送地图事件. 已在0再次请求0必须返回 `AlreadyExists`.
- 战斗中或已组队角色请求地图0必须返回 `FailedPrecondition`, 不自动离队或解散. 非0目标只接受测试范围`[80000,89999]`、练级范围`[90000,99999]`或任务范围`[1000000,1999999]`中配置存在且 `encounter.enabled=true`、`encounter.enemyGroups`有效的地图. 单人进入只迁移自己; 队长进入迁移完整队伍; 普通成员主动进入和任一成员战斗中必须拒绝.
- 非0地图 Presence 只保存成员展示数据, 不保存坐标、朝向或格子索引; 移动和转向请求必须拒绝. 自动遇敌只使用当前地图平铺的 `encounter.enabled` 和 `encounter.enemyGroups`. 开启自动遇敌时角色必须存在于当前非0地图 Presence; 离开到地图0必须取消 timer 和开关, 并只向本人回复 `CombatAutoEncounterSetRes(enabled=false)`.
- 非0地图的 `CharacterMapEnterRes.map_group_list` 必须排除自己的队伍. `CharacterOnlineRes.map_group_list` 必须为空, 因为角色上线时运行态地图固定为0. 分组长度为1时按单人展示, 大于1时第一项是队长、后续项是队员.
- `map_join` 携带完整新分组; `team_join` 向同地图全部观察者携带合并后的完整队伍, 客户端据成员身份原子移除旧分组并把新队伍追加到末尾, 接收者属于该队时不得显示自己的队伍. `map_leave`、`team_leave` 和 `team_disband` 只携带定位所需的角色键. 队长直接离开地图只发送队长 `map_leave`; 队伍解散后队长单人离开依次发送 `team_disband` 和 `map_leave`.
- 列表保持加入顺序. 成员离队后追加为单人, 解散后原成员依次追加为单人, 离开地图只删除条目且不重排其他条目.
- `MapCharacterInfo.in_combat` 必须反映 Presence 当前战斗状态; 单人或队长组队开战、个人脱离及战斗结束都发送完整 `character_update`. 队长遇敌必须只拉入冻结名单中通过入场校验的成员.
- 角色可见资料变化发送完整 `character_update`; 原始 exp 改变但显示等级不变时不得发送, 跨显示等级时必须发送.

## 角色属性加点与重置

- `CharacterAttributeAddReq` 和 `CharacterAttributeAddRes` 的消息 ID 分别保持为 `0x00100B` 和 `0x00100C`.
- 体力、腕力、耐力和速度分别只增加目标字段 1 点并扣除 1 点 `available_point`; 魅力及未指定枚举必须拒绝.
- 非当前账号角色、未上线角色、战斗中角色、无可加点角色和目标字段 `uint32` 溢出必须拒绝且不得修改档案.
- cache 保存失败必须回滚账号槽位和角色内存档案; 保存成功后必须先发送 `CharacterBaseChangedNotify`, 再发送成功响应.
- `CharacterAttributeResetReq` 和 `CharacterAttributeResetRes` 的消息 ID 分别保持为 `0x00100D` 和 `0x00100E`.
- 重置请求只提交最终四项属性; 当前权威总点数必须在 20 至 1000 之间, 最终四项至少分配 20 点且不得超过权威总点数, 剩余 `available_point` 必须由服务端计算并保持总点数守恒.
- 重置使用独立候选账号档案写 cache; cache 返回前不得修改 online 权威账号槽位或运行中角色档案, cache 失败时操作失败且原档案保持不变, cache 成功后才提交并发送权威变化通知和成功响应.
- 运行 `go test ./online -run 'CharacterAttribute(Add|Reset)|PrepareCharacterAttribute|PersistCharacterAttribute'` 覆盖消息 ID、字段映射、克隆隔离、边界校验、总点数守恒及持久化提交/失败不提交.

## 角色邮箱

- `CharacterMailboxGet/CharacterMailRead/CharacterMailDelete` 请求和响应 ID 保持为 `0x005000-0x005005`, `CharacterSystemMailNotify` 保持为 `0x005006`。
- get/read/delete 只接受当前账号已上线角色; 非法参数、非本账号角色和离线角色分别返回对应业务错误, 不导致 gateway 断线。
- `GMCommandReq.mail_add` oneof tag 保持为 12. Cache 持久化成功后必须先发送完整邮件通知, 再发送 GM 成功响应; Cache 失败时两者都不得发送成功结果。
- 主题最多 30 个 Unicode 字符, 正文最多 500 个 Unicode 字符; 正文 CRLF/CR 规范化为 LF, 非法控制字符必须拒绝。
- 运行 `go test ./common ./cache ./online -run 'Mail|Mailbox'` 覆盖文本规范化、邮箱 hash 解码、消息序列化和 GM oneof。

## 基础战斗回归

- 攻击、闪避、暴击、伤害、权威 HP 和死亡结果一致.
- 10级以上且装备快照武器槽为空的玩家普通攻击按BaseLuck生成1、2、3或5至10段, 永不生成4段; 每段保持完整普通攻击伤害, 原目标死亡后剩余段改选存活敌方, 攻击者死亡、敌方全灭或段数耗尽时停止, 整组只检查一次反击.
- 玩家已装备武器时, 每次实际行动必须在命令分派前只从 `attacknum_min/attacknum_max` 闭区间抽取一次段数; 非普通攻击仍消耗该随机值, 但不使用其段数或爪分摊. 爪普通攻击按计划总段数分摊正伤害, 原目标死亡后每个剩余段独立重选存活敌方; 弓以外的其他当前武器不分摊. 缺失装备快照不得误触发武器路径, 合击成员与反击仍保持单段.
- 开战快照使用服务器有效属性和实例固化的运气/暴击值, 元素及额外伤害/防御来自当前武器和两个首饰位配置; 更换同配置但固化值不同的实例时, 战斗数值必须采用实际实例而不是重新随机.
- 回旋镖必须按声明目标所在排扫描. Initiator排内顺序为`4,2,0,1,3`, Defender为`3,1,0,2,4`; 后排在各偏移上加5. 声明目标死亡仍保留原排, 整排无有效目标时才从全部存活敌方随机一次决定回退排. 空位、死亡和离场单位跳过, `attacknum`随机仍消费但不限制同行实际目标数, 每个有效位置最多攻击一次.
- 回旋镖每个目标先完整执行物理攻击, 再用C `float`乘0.3并向零截断, 不补最低1点. 缩放前1至3点正伤害得到0后仍保持Normal或Critical表现, 缩放前0伤害保持Miss/Guard. 回旋镖在合击资格随机前排除且攻守任一方装备时不进入反击随机. 运行`GOCACHE="$PWD/.gocache" go test ./online -run TestCombatBoomerang -count=1`覆盖目标表、伤害边界、死亡声明目标、空排回退、攻击次数忽略、合击和反击.
- 弓的两个 `aBowW` 分支必须按声明目标列生成“同行位置后紧跟另一排同列”的十站位表, 每个站位只出现一次. 空位、死亡和离场单位不消耗 `attacknum`; 3箭只攻击顺序中前3个有效站位, 10箭最多各攻击10个有效站位一次, 击倒目标后继续扫描剩余站位. 固定随机向量必须覆盖两张精确候选表、跳空位、跳死亡、无重复目标和候选耗尽.
- 弓暴击必须保留普通暴击概率、随机次数和 `critical=true`, 但伤害只能使用普通伤害, 不得追加非弓暴击的防御与等级增伤. 弓在合击随机前排除且不消费该随机数; 角色反击使用实际武器类型的原版相性表, 弓、回旋镖、投掷斧和投掷石任一方参与时不得进入反击随机. 当前尚未实现的装备魔法、变身禁远程、守护代挡、状态抗性及其他投掷武器专用行为不得在回归说明中宣称完成.
- 防御在行动排序前生效, 并按独立动作返回.
- 合击只生成真实成员动作; 每个成员都携带独立 Damage 表现结果, 前序成员不显示伤害且没有 HP 差量, 首成员记录完整顶层来源, 最后一名成员显示并统一应用累计伤害.
- 反击链、最大深度和行动顺序稳定.
- 逃跑成功必须先由 Escape 的 `UnitDeltaList` 下发角色及可选战宠的 `escaped=true`, 再由 `UnitLeave(Escape)`实际移除; 无战宠时不得生成虚构宠物键, 其余参与者继续战斗.
- 宠物离场或死亡前已获得的战斗经验仍参与结算.
- PVE 经验持久化失败时不发布部分内存状态.
- 同一战斗中每个参与角色必须收到独立的 `CombatRoundResultNotify`, `recipient_character_uuid` 必须等于当前接收角色; A 的经验、DP、道具及战宠结算不得出现在 B 的消息中.
- 正常战斗结束后取消 timer、清理 CombatRoom actor 指针, `CombatFlowCompleteReq` 后按开关恢复自动遇敌.
- 角色下线或运行态清理后立即清除自己的 CombatRoom actor 指针并通知房间. 房间必须把该角色及战宠的 `UnitLeave(Detached)`放在下一条回合结果的其他战斗效果之前, 只取消该参与者的未执行动作; 其余参与者继续, 房间为空时才无结算关闭.
- 玩家角色 Ultimate 击飞必须在 `Knockback` 后追加 `UnitLeave(Defeated)`, 同时清除其战宠、把角色运行态地图设为0并移出旧 Presence; 其他参与者继续战斗.

## Docker 验证

1. 重新构建 online 镜像, 确保代码和 `/app/config` 来自同一工作树.
2. 启动 online 容器.
3. 查看容器日志, 确认配置加载成功、gRPC 监听成功且服务完成注册.
4. 若启动失败, 先修复日志中首个缺失文件或配置错误, 不用临时空数据绕过后续校验.
5. 发送一个未开放技能请求, 确认客户端收到业务错误且 gateway 连接保持可用.

## 捕获

`combat.capture_test.go` 验证角色请求目标和宠物技能边界, 原版 HP 平方公式、float32 阈值和严格小于判定, 敌群权限与等级限制, 账号消息确认前不得移除敌人, 容量不足及保存失败不得产生成功结果或新 UUID. 个体测试使用独立固定向量验证 SavedBase 在十点初始加点之前保存, 捕获档案的 Raw、等级、品阶、出生技能和成长基线不重抽, 修改模板或已创建记录不会污染快照. 持久化测试覆盖完整候选记录提交、容量检查前置、UUID 耗尽、角色槽缺失和 cache 失败回滚.

```bash
GOCACHE="$PWD/.gocache" go test ./online -run Capture -count=1
GOCACHE="$PWD/.gocache" go test ./common/pet ./online ./proto/pb
```

