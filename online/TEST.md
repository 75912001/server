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
技能.yaml
ai.yaml
enemy.group.yaml
enemy.exp.yaml
exp.yaml
item.yaml
道具.素材.yaml
item.currency.yaml
道具.天工司.yaml
道具.武器.爪.yaml
道具.武器.斧.yaml
道具.武器.棍.yaml
道具.武器.枪.yaml
道具.武器.弓.yaml
道具.武器.回旋镖.yaml
道具.武器.投掷斧.yaml
道具.武器.投掷石.yaml
item.equipment.chest.yaml
item.equipment.helmet.yaml
item.equipment.shield.yaml
item.equipment.gloves.yaml
item.equipment.belt.yaml
item.equipment.boots.yaml
item.equipment.accessory.yaml
商店.yaml
reward.yaml
task.yaml
pet.yaml
scene/*.yaml
```

验证要点:

- 缺少任一必需文件时, online 明确启动失败.
- `技能.yaml` 的 ID、`usableBy` 和连续攻击段数非法时加载失败; 服务端配置不包含客户端展示用的名称和说明, 残留旧 `cost` 字段必须加载失败.
- `mightyAttack` 必须为对象, 同时提供整数倍率1-655和目标闪避加值0-32767; 空值、缺项、小数、越界和与 `continuationAttack` 并存均加载失败.
- `poisonAttack` 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0); 与 `continuationAttack` 或 `mightyAttack` 并存, 空值, 缺项, 字符串, 小数及越界均加载失败. RAW中8100060必须保持3次/-30%/2500石币且为`pending_test`, 正式双端、商店、出生技能和AI不得包含它; 8100061仍为已发布的5次/-30%/4000石币.
- `stoneAttack` 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0); 空值、缺项、字符串、小数、越界、其他行为块并存、角色可用、`mpCost`或`targetScope`均加载失败. RAW中8100080/8100509必须保持3/-30和9/-30、2300石币及`pending_test`, 原版708保持未实现; 正式双端、商店、出生技能和AI不得包含两个现代ID.
- `confusionAttack` 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0); 空值、缺项、字符串、小数、越界、未知字段、其他行为块并存、角色可用、`mpCost`或`targetScope`均加载失败. RAW中8100090/8100510必须保持3/-30和9/-30、2000石币及`pending_test`, 原版709保持未实现; 正式双端、商店、出生技能和AI不得包含两个现代ID.
- `sleepAttack` 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0); 空值、缺项、字符串、小数、越界、未知字段、其他行为块并存、角色可用、`mpCost`或`targetScope`均加载失败. RAW中8100110/8100511必须保持3/-30和9/-30、2500石币及`pending_test`, 原版710保持未实现; 正式双端、商店、出生技能和AI不得包含两个现代ID.
- 五系异常精灵必须分别使用 `poisonSpirit`, `stoneSpirit`, `confusionSpirit`, `drunkSpirit`, `sleepSpirit` 独立参数块, 状态ID固定为1、4、6、5、3. `mpCost` 必须属于技能且为非负整数, `targetScope` 只允许 `singleOpponent` 或 `opponentCamp`; 装备配置通过 `magicid` 提供自带技能, 装备实例通过 `additional_skill_id_list` 保存配置之外后期添加的技能, 两者都不接受装备侧耗蓝或成功率.
- 治愈、滋润、恩惠必须分别使用 `healingSpirit`, `moistureSpirit`, `graceSpirit` 独立参数块, `targetScope` 固定为 `self`, `singleAlly`, `allyCamp`. 每个目标独立执行基础治疗量90%-110%随机和体力倍率, 排除死亡单位并限制到最大HP; 角色体力点按1%/点, 宠物原始百倍体力按0.005%/点折算.
- `item.yaml`、`道具.素材.yaml`和`item.currency.yaml`分别只允许普通道具、素材和货币分组, 8个`道具.武器.<类型>.yaml`和7个`item.equipment.<类型>.yaml`必须各自只包含文件名对应的唯一分组; 所有文件使用`items.<group>.<id>`结构并合并为统一运行期索引. 文件缺失、分组放错文件、未知分组, 空非装备或武器分组、ID超出分组区间、非法 `atlas` 路径、18项固化数值范围或攻击次数范围倒置、非法职业、非法武器类型或非法元素配置均应加载失败. 七类装备文件允许空发布分组.
- 旧 `item.synthesis.yaml` 对应功能已暂停, Online 运行时不读取该文件; 其独立解析和算法单元测试仍保留. `道具.天工司.yaml`的基础配方必须有1-8种不重复素材, 数量1-999, 产物必须是已配置武器且素材必须存在; 不同产物允许共享素材签名, 候选按武器ID升序建立索引并在命中时等概率随机一个. `attributes`只允许`ground/water/fire/wind`, 每件装备最多4种元素, 每种元素的素材规则与基础配方一致, 且同一装备下素材签名不得重复.
- 8个`道具.武器.<类型>.yaml`的`attacknum`, `attack`, `defence`, `quick`, `hp`, `mp`, `luck`, `charm`, `avoid`, `poison`, `paralysis`, `sleep`, `stone`, `drunk`, `confusion`, `critical`, `counter_modifier`, `damage_bonus_percent`, `crit_damage_bonus_percent`必须使用恰好两个整数的`[min, max]`数组; 后三项按万分比配置且当前不接入战斗公式, 七类装备继续兼容旧`_min/_max`字段.
- 普通道具 `sprite` 为0时不能配置 `atlas`, `sprite` 大于0时必须配置以 `item/` 开头的无扩展名 `atlas`; 七类装备在服务端允许按 C/S 归属独立省略展示字段, 但 sprite 出现时必须为正数, atlas 出现时必须合法. 武器和七类装备不能配置 `use`. `equipmentAccessory` 有条目时必须配置合法 `accessory_type`, 并与 proto 子区间相符.
- `pet.yaml skill` 的非 0 ID 不存在于 `技能.yaml` 时加载失败.
- `ai.yaml` 的 ID 必须为非零唯一整数. `skills[]` 必须非空, 技能 ID 必须存在且不重复, 每项必须显式提供正整数权重, 单项和总权重不超过2147483647且总权重大于0; 目标范围和目标策略保持原有校验. 权重为0或缺失时必须加载失败, 列表允许超过7项.
- `pet.yaml creationMode` 只允许省略或填写 `fusionEgg`. 普通宠物应用品阶偏移后的 `SavedBase*` 和原版公式计算出的 `Raw*` 使用 `int32`, 允许为0或负数; protobuf 往返、档案绑定和战斗构造不得发生无符号下溢. 派生后的最大生命和攻击仍必须大于0. GM 增宠请求必须校验1-140级目标等级, 并生成对应等级的经验、成长基线和四维; 0级或超过上限必须无损拒绝. 融合蛋允许保留原版占位成长, 但 `common/pet.NewRecord` 和 GM 普通创建入口必须拒绝, 且拒绝时不得修改 UUID 游标或角色宠物列表.
- `scene/*.yaml` 不得设置格式版本字段; 地图 ID 必须是客户端正整数 `map_id`, 地图尺寸和 `collision.blockedRows` 必须合法. 当前配置目录包含70000至70002这3张任务地图, 80000、80001、80010、80020、80030、80040、80050、80060、80070这9张测试地图, 以及90001和90010至90130按10递增的14张练级地图; `CharacterMapEnterReq` 必须接受已配置且遇敌有效的任务、测试或练级地图.
- 阻挡区、传送起点越界或已存在目标地图的落点越界时加载失败.
- `encounter.enabled` 必须设置; 启用遇敌时 `encounter.enemyGroups` 必须引用至少一个已存在的敌人组, 总权重必须大于0.
- `enemy.group.yaml enemies[].id` 必须引用存在的宠物模板; 每个敌人的 `battleAI` 必填且引用存在的 AI. 缺失、0或未知 AI 均加载失败. `normalDrops`可省略, 配置时最多10项, 每项必须引用正式道具并提供`[1,10000]`万分比; 相同道具允许作为多个独立槽位重复出现. `pet.yaml` 可以独立于 AI 表加载, 其技能只用于出生模板.
- 旧 `pet.yaml battleAI`、`enemies[].skill`、`attackWeight`、`defenseWeight`、`escapeWeight` 和 `skillSlotWeights` 必须明确报错. AI 中不支持的 NPC 技能必须使 Online 在注册 etcd 和 gRPC 前启动失败.
- 除阿布洞窟敌群70000外, 当前敌人必须引用 AI 1, 攻击、防御、逃跑权重保持 `10:1:1`; 敌群70000的五名守护者按顺序引用 AI 13-17并保持各自配置权重.
- 多数宠物出生技能保持 `[8000001,8000002,0,0,0,0,0]`, 查罕·乌尔夫和查罕·吉鲁的出生技能继续包含 `8100003`; 敌人选择只读取`enemy.group.yaml`引用的AI, 不回退宠物出生技能.
- 配置加载失败后不注册 etcd, 不启动 gRPC 业务入口.

## 敌人成员等级

- Boss 成员必须且只能配置 `level` 或 `levelRange`; 普通组成员允许省略二者并使用组级规则, 但同样拒绝同时配置.
- 验证固定等级、闭区间及单点区间加载成功; 缺失Boss等级、互斥字段同时出现、范围倒置、元素数量错误和协议等级越界必须失败.
- 五名阿布洞窟守护者必须保持原顺序, 只配置原版 `[39,41]`、`[38,40]`、`[37,39]`、`[35,38]`、`[40,43]`, 不保留固定 `level`.
- 建房等级测试验证成员范围的两个端点、每次创建独立抽取、成员范围优先于组级范围, 并保留固定等级不消耗随机数和玩家等级偏移边界检查.

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run 'EnemyGroupMemberLevelModes|EnemyGroupProjectGuardianLevelRanges|CombatPVEEnemyLevel' -count=1
```

## 敌人普通掉落

- 每个`enemies[].normalDrops[]`在敌人实例创建时按配置顺序独立执行一次`RAND(0,9999) < probability`; 未配置掉落不消费随机数.
- 抽中的现代道具ID冻结到敌人运行态, 后续继续复用击杀动作归属、玩家临时三格和战后背包/装备实例持久化流程. DP战不分配普通掉落.
- 武器、首饰和六类普通防具掉落都必须创建独立`EquipmentRecord`并进入背包. 阿布洞窟70000的`3500574`头盔与`3510568`胸甲必须和经验、任务60进度共用一次原子持久化; 失败时背包、UUID、经验和任务记录全部回滚.
- 聚焦验证命令: `GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run 'EnemyGroupNormalDrop|CombatPVEEnemyDrop|PVEEnemyDrop|CombatDrop' -count=1`.

## 角色任务

聚焦验证命令:

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb -run 'Task|Reward' -count=1
```

- task.yaml必须校验正整数唯一任务ID、从1连续编号的非空步骤、非空完成条件、合法条件字段及现有任务/道具/宠物/敌群/奖励包引用. reward.yaml允许空rewards数组, 非空奖励包必须至少包含合法且不重复的道具/装备或宠物; 所有数量为正, 奖励宠物必须配置合法等级和`grade: random`.
- completionMode省略时为automatic. consumeItems和consumePets仅允许submit; itemPossession与petPossession只检查持有, 后者按宠物ID和实际等级精确匹配. taskRewardsClaimed要求前置任务全部步骤完成且奖励全部领取. battleVictory不能用于接取、开始或submit完成条件.
- 阿布洞窟60必须是非主线、无接取条件、单步骤、enemyGroupId 70000、rewardId 0. 其他敌群胜利、失败或未接取均不得完成.
- 卡坦任务61必须按两批扣除4只25级宠物, 第一批领取火难的戒指, 第二批交回戒指并领取1级随机品质修宝. 任务62必须以任务61全部领奖为前置, 按两批扣除另外4只25级宠物并领取1级随机品质朵拉比斯.
- 多任务并行, 单任务步骤串行. 无奖励步骤完成后completed_at_ms与reward_claimed_at_ms相同, 后补奖励不产生历史可领取状态.
- 步骤奖励独立领取, 重复领取必须拒绝. 仅显式配置`rewardReissue.whenItemAbsent`的纯单道具奖励允许补领, 且必须同时满足任务未完成、步骤奖励已领取、指定道具当前持有量为0; 仍持有任意数量或任务已完成时拒绝. 背包满、宠物栏满、数量溢出、UUID耗尽、扣除不足、Cache失败必须保留原任务、完整库存和UUID游标.
- 提交和领奖成功后, 服务端连发 `CharacterNotify` 各变化域分支: `container_changed`携带完整角色背包与账号UUID游标并原子替换, `item_changed`增量合并货币资产, `pet_changed`处理随身宠物增删; 客户端按到达顺序应用这些权威快照后再合并任务记录增量. 任务记录增量只含变化任务ID. 任务/步骤数量、时间顺序或串行状态非法时拒绝绑定.
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

## 全局道具商店购买

- `ShopPurchaseReq` 和 `ShopPurchaseRes` 的消息 ID 分别保持为 `0x003007` 和 `0x003008`.
- 服务端 `商店.yaml` 使用无 `type` 的扁平 `items`, 只包含开放购买商品. 请求以道具 ID 直接定位同 ID 商品, 并按 `costs[]` 同时结算全部资源; 空 `costs` 表示免费, 重复资源、非正数量、余额不足和购买数量为0必须拒绝.
- 角色必须属于当前账号、已经在线且不在战斗中. 任一支付资源不足或背包满时不得修改账号档案; 装备购买数量超过剩余格数或 UUID 游标耗尽时同样拒绝.
- 素材等可堆叠商品按 `item_count × quantity` 增加普通背包最终数量, 已持有同种商品时不新增背包格, 且不得分配装备 UUID 或创建 `EquipmentRecord`. 成功响应通过 `item_remaining_count` 返回商品最终持有量.
- 成功购买多件装备时, 新装备 UUID 必须从旧 `used_uuid + 1` 连续递增. 每件记录只在 `record_base_map` 保存18项随机基础属性中的非0值, `record_modifier_map` 和 `additional_skill_id_list` 初始为空; 配置元素数值非0时通过 `element_attribute` 同时保存元素类型和原版1-100百分比值. 每项基础属性在对应配置闭区间内且不同实例分别创建, 并一次性扣除总价.
- cache 持久化失败必须保留原角色资源、背包和 UUID 游标; 成功响应的 `cost_result_list` 必须完整返回每项实际消耗及最终余额, 同时返回最新游标和完整的本次新增装备列表.

## 宠物加工

- `ItemSynthesisReq/Res` 消息 ID 保持 `0x003009/0x00300A`, 请求使用宠物 UUID 和聚合后的 `material_list`.
- 成功覆盖精确配方、重复签名候选的等概率随机、单候选不抽取产物随机数、完整 `EquipmentRecord`、随机基础属性范围、UUID 更新和全部素材消耗. 候选随机索引越界必须无损拒绝; 未命中覆盖按单位数量权重选择并仅返还 1 个素材.
- 29/30 个背包位置允许加工; 30/30 拒绝且不扣材料, 即使投入会被完全消耗也不例外.
- 离线、战斗中、未学习 `8100200`、素材不足均按 `FailedPrecondition` 拒绝. 还覆盖非法或重复素材、宠物不存在、UUID 耗尽和持久化失败回滚.
- `EquipmentSkillAttachReq/Res`消息ID固定为`0x00300B/0x00300C`, 请求使用装备UUID和聚合后的素材列表. 目标只允许位于当前角色背包, 且复用宠物加工资格、角色状态及操作前至少一个空位的约束.
- 附技能精确命中时必须保留装备UUID及全部既有档案字段; 不同系技能追加, 同系异常精灵技能原位替换, 消耗全部投入且不推进账号UUID. 未命中时消耗全部投入并返回完整原装备; 重复附加同一技能必须无损拒绝.
- 装备附加技能只要求技能存在, 不要求`usableBy`包含`character`; 不存在、重复、两个同系异常精灵技能或等于装备配置自带技能的实例技能仍必须被档案校验拒绝. cache失败必须回滚装备和全部素材变化.
- 五系异常精灵抗性必须只按`additional_skill_id_list`实时派生: 已拥有类别固定`+10`, 未拥有类别按不同类别数`N`取`-10*N`; 麻痹、配置自带技能和普通技能不参与. 派生值与`record_base_map`和`record_modifier_map`相加, 不新增协议或存档字段.
- `EquipmentElementAttachReq/Res`消息ID固定为`0x00300D/0x00300E`, 与附技能共享目标装备、宠物加工资格、非战斗状态、素材及操作前空位校验.
- 附元素精确命中时写入固定值20, 保留装备UUID和其他档案字段; 无元素直接写入, 异系元素覆盖, 同系元素无损拒绝. 未命中时消耗全部投入并返回完整原装备; 命中和未命中均不推进账号UUID, cache失败必须回滚装备和素材变化.

聚焦命令:

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb -run 'Tiangong|ItemSynthesis|EquipmentSkillAttach|EquipmentElementAttach|EquipmentRecord' -count=1
```

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
- 只允许当前账号中已上线且不在战斗中的角色换装. 装备 UUID 必须位于该角色背包; `record_base_map`基础属性必须非0且位于当前配置闭区间, `record_modifier_map`附加修正必须非0、使用已知属性key且处于int32范围, 缺失key按0校验. `additional_skill_id_list`只允许不重复、不同系、配置表之外的已知技能, 不以`usableBy`限制档案写入; `element_attribute`必须同时包含合法元素和原版1-100百分比值. 任一结构非法或UUID/配置不匹配均应拒绝.
- 装备时校验角色等级. 当前角色没有职业档案字段, 所有 `neprof != 0` 的装备都必须明确拒绝, 不得把未知职业当成满足要求.
- 替换时新装备进入目标部位, 旧装备回到背包; 卸下时目标部位装备回到背包且必须有剩余容量. 计划构造和 cache 持久化失败不得修改原背包、穿戴记录或运行中角色引用.
- 成功响应必须同时返回完整 `item_bag`、本次替换部位的 `equipment` 和同一候选档案计算出的 `effective_attribute`; 卸下时 `equipment` 不设置. 角色创建、角色档案、宠物配置、`effective_attribute.elemental`和战斗快照全链路统一使用原版0-100百分比值, 基础分配总和为100, 装备值直接叠加后封顶100. 连续请求由 Account actor 按收到顺序串行处理, 不依赖客户端禁止重复操作.
- 账号校验必须覆盖背包、仓库和已穿戴武器及两个首饰位之间的 UUID 全局唯一性, 并拒绝尚未开放的六个防具部位.
- 六类防具允许作为合法装备实例保存在背包或仓库, 但当前换装请求和角色档案仍必须拒绝把防具写入尚未开放的穿戴部位.
- 两个首饰位允许六类不同类型的全部组合, 同类型即使 ID 和 UUID 不同也必须拒绝. 满背包允许同一部位的原子替换, 卸下仍要求空位. 原枚举值1和3及档案字段 tag 1和3必须保持不变.
- 运行 `GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run TestAccessory -count=1` 验证类型和 ID 边界, 穿戴互斥, 购买实例, 持久化失败, 属性叠加和战斗快照.

## 统一技能测试

角色:

- `8000001` 生成普通攻击动作并校验敌方目标.
- `8000002` 生成以自身为目标的防御动作.
- `8000003` 生成逃跑动作.
- `8000004` 生成捕获动作, 缺少目标、同阵营目标或未知目标必须拒绝. 捕获不开放给玩家宠物和敌方 NPC.
- `8000005-8000007` 已配置但未开放, 必须返回业务错误且不写入本回合动作.
- `技能.yaml` 不存在的 ID 必须拒绝.
- `8200130-8200139`、`8200150-8200159`、`8200160-8200169`、`8200170-8200179`、`8200180-8200189` 只允许玩家角色从九个部位的开战装备快照取得. 未装备、MP不足和非法单体目标必须拒绝; 群体技能由服务端展开敌方存活单位, 每个目标独立判定且整次只扣一次技能MP.
- 异常命中拒绝覆盖任何已有普通异常. 睡眠和石化在最后一次计数仍阻止原动作; 混乱有80%概率改为攻击随机存活单位且不选自己; 酒醉期间敏捷减半并增加20-30点被闪避率, 到期恢复; 中毒沿用行动前毒伤且不能致死. 物理死亡必须清除五系状态.
- `go test ./common/gameconfig ./online -run StatusSpirit -count=1` 覆盖生产配置、装备授权、技能耗蓝、状态成功率、范围和控制时序.

宠物学习与档案:

- `商店.yaml` 中开放购买的宠物技能商品 `costs` 必须逐项等于原版 8.0 `petskill2.txt` 基础价格; 原版 8.5 服务端源码用于校验 `PETSKILL_COST` 的权威读取与扣款流程, 没有同 ID 技能商品的技能不可学习.
- 新宠物必须从 `pet.yaml skill` 深拷贝恰好 7 个实例槽位; 修改实例不得污染模板. 旧 cache 缺槽、槽位数不为 7 或引用未知技能时账号绑定失败.
- 学习和替换按新技能基础价扣除石币, 不退款、不抵扣旧技能价格; `skill_id=0` 免费遗忘且石币不变, 允许遗忘最后一个有效技能并形成全空的固定 7 槽.
- 宠物技能和全部支付资源必须在同一 `AccountRecord` 候选档案中一次持久化; 多资源任一余额不足、cache 失败、非法槽位、非随身宠物、离线或战斗中请求都不得产生部分修改.
- `PetSkillSetReq/Res` 消息 ID、字段及 protobuf 往返保持稳定.

玩家宠物战斗:

- 技能必须同时存在于 `技能.yaml` 和该玩家宠物开战时的 `CombatUnit.skill_id_list` 快照.
- 服务端必须先拒绝未实现的生活或其他非战斗技能, 再校验技能持有关系. `8000002` 防御是玩家宠物的唯一持有例外, 技能栏没有防御或完全为空时也必须生成防御动作; 敌方 AI 不享受该例外.
- `8000001`、`8000002` 分别生成攻击和防御动作.
- 测试配置分配后, `8100000`, `8100003`, 连续攻击, 一击必杀, 毒攻击/猛毒攻击, 突击, 忠犬, 不防守战法, 背水之战和手下留情应由统一解析器生成对应动作.
- 旅程伙伴130/607/608通过测试配置注入`8100130/8100607/8100608`. `abduct`必须为对象且只允许可选整数`loyaltyThreshold` 1-100, 与其他行为块互斥, 只允许宠物使用且不接受`mpCost/targetScope`.
- 提交阶段拒绝人物、己方、死亡和不存在目标且不消费随机; 执行期目标失效时只在敌对非角色单位中重选. 普通公式覆盖C向零截断、最低50、严格`<`和超过100; 607/608覆盖玩家战宠忠诚度60/80严格边界、NPC忽略阈值及Boss强制失败但仍只抽一次.
- 成功步骤效果固定为`Abduct(success)`后`UnitLeave(Abducted)`, 离场键顺序为目标、施法者; 失败只携带施法者. 两者清理Guard、蓄力和不防守姿态, 不改HP/alive、宠物档案或`PetCarryStatus`, 不产生Damage、Capture、奖励或额外事件. 运行`go test ./common/gameconfig ./online ./proto/pb -run Abduct -count=1`覆盖该链路.
- `8100000` 不需要目标, 必须生成携带技能ID和来源单位、但没有效果的实际动作步骤, 不得伪造成未执行动作.
- `8100003` 对有效Guard目标必须跳过GuardAdjust及其随机数, 保留普通物理防御、元素和暴击公式并且不返回Guard结果; 对非Guard目标仍先消费原版AttackSeq随机链, 再把Dodge、Critical和伤害统一覆盖为0伤害MISS.
- `8100003` 不得清除目标Guard、参加合击、成为反击者或触发受击目标反击. 运行 `go test ./online -run 'TestCombat(GuardBreak|Standby)' -count=1` 覆盖成功、失败、随机数和动作步骤.
- 一击必杀只生成一个主动步骤, 三档倍率必须作用于暴击、Guard和最低伤害后的结果, 保持C float舍入、0伤害MISS和普通暴击表现. 30/40/50点闪避先加再按基础75%封顶, Guard跳过闪避, 后置装备闪避继续独立判定.
- 一击必杀不得参加合击或消费合击资格随机; 行动前不能反击, 行动后可以反击. 后续反击必须与普通Attack的伤害、闪避和随机序列一致, 同时保留声明技能ID. 参数及技能归属使用冻结快照, 非持有者、非法目标和角色指令必须拒绝.
- 突击30/31和三重突击605保留原版描述及4000/8000/8500学习价格证据, 验证 `chargeAttack` 必填整数, 1-10/0-32767边界和五种攻击机制互斥. 605使用测试配置注入 `8100605 -> 3/+250`; 玩家宠物和NPC均须持有技能, 角色和非法目标必须拒绝; 提交后修改请求对象或配置不得改变蓄力参数及目标.
- 三档突击前1/2/3次行动只蓄力, 不提前命中或消费攻击随机; 第2/3/4次行动的单次物理结果与基础攻击力按原版C double公式修正后的普通攻击完全一致, 包括随机抽取次数. 释放攻击力约为190%/210%/350%, 读取当时基础攻击力; 目标失效只在释放时重选, NPC后续回合不得再次抽AI.
- 突击不参加合击, 自身等待和释放时都不能反击; 目标按普通资格和概率反击. 普通非致死伤害保留蓄力, 每次行动前正常处理一次毒伤, 死亡和离场清理续招. 玩家角色全灭仍正常结束战斗.
- 真实请求和序列化战报应覆盖首次选招确认, 后续宠物自动锁定, 角色仍需选招, 重复请求拒绝, 超时保留续招, 旧超时回调失效, 释放后的下一回合恢复选择. `next_round_auto_action_unit_key_list` 必须包含剩余计数为0但尚未释放的宠物, 不包含释放完成的宠物.
- `go test -buildvcs=false ./common/gameconfig ./online -run Charge -count=1` 覆盖三档突击配置, 结算和跨回合协议; `python -B tool/test_skill_catalog.py` 检查目录30/31/605映射、605的3/+250运行参数及 `pending_test` 人工状态.

- 地球一周121通过测试配置注入`8100121 -> earthRound.damagePercentModifier: 200`. 配置测试覆盖必填整数、0-32767边界、字符串、小数和行为块互斥, 且只允许宠物使用并拒绝`mpCost/targetScope`; 目录测试确认原版120不出现在编辑器RAW, 121保持`pending_test`且正式C/S配置不包含现代ID.
- 首次行动必须冻结技能ID、目标、参数和行动值, 只输出`Visibility(hidden=true)`及`hidden_changed/hidden`权威增量. 隐藏单位保持存活和在场, 但敌对单体、群体、物理、异常魔法及AI候选都必须排除. 下一回合玩家宠物和NPC直接续招, 原目标失效时按普通目标调整规则重选.
- 释放步骤必须先输出`Visibility(hidden=false)`, 再输出普通单段Damage; 最终伤害严格等于相同随机序列普通物理结果的3倍. 两阶段均不参加合击且施放者不取得反击资格, 目标仍走普通反击; 死亡和离场清理续招与隐藏. 运行`go test ./common/gameconfig ./online ./proto/pb -run EarthRound -count=1`覆盖该链路.

- 忠犬20/21/22通过测试配置注入`8100020 -> 攻-20%`、`8100021 -> 攻-10%/防+40%`和`8100022 -> 攻+50%/防+100%`. 验证必填/可选int32、行为块互斥、玩家宠物/NPC解析、参数冻结和仅允许敌方主动目标.
- 直接普通物理必须先判主人闪避, 未闪避才转移到守护宠; 战报声明目标仍为主人, Damage目标为守护宠并携带Guardian Reaction. 合击和反击不得触发, 多段逐段复查, 守护宠死亡后的后续段恢复攻击主人. 死亡、离场和麻痹、睡眠、石化、混乱、障壁、晕眩、天罗地网立即使关系失效. `go test ./common/gameconfig -run Guardian -count=1`和`go test ./online -run CombatGuardian -count=1`覆盖该链路.

- 不防守战法150/151/152通过测试配置注入`8100150-8100152`. 验证三个必填整数、0-32767/0-255边界、行为块互斥、玩家宠物/NPC归属、无目标输入及参数冻结. 姿态必须在排序前生效, 自身行动只生成无效果步骤且不进入合击.
- 闪避测试覆盖30/40/50加值、75%封顶和Guard短路; 非玩家反击覆盖0/50/127/128/255字节边界、`<=`比较和最低阈值1. `criticalPercent`不得改变暴击阈值. 下一回合、死亡、离场、Sleep/Stone及混乱改写动作必须清理姿态. 运行`go test ./common/gameconfig -run NoGuard -count=1`和`go test ./online -run CombatNoGuard -count=1`.

- 背水之战50-54通过测试配置注入`8100050-8100054`. 验证两个必填int32字段、空值/类型/溢出/互斥、玩家宠物与NPC归属、敌方目标、参数冻结及52/53并存.
- 五组攻防必须按C float32及向零截断得到`125/65`、`145/45`、`180/50`、`165/40`、`200/30`. 防御在自身行动前生效, 专用命令不参加合击且出手前不能反击; 转为普通攻击后可反击并继承修正攻击, 下一回合恢复基础值. 运行`go test ./common/gameconfig -run PowerBalance -count=1`和`go test ./online -run CombatPowerBalance -count=1`.

- 手下留情626保留原版描述及10000学习价. `showMercy` 必须为空对象, 拒绝null, 布尔, 数组, 字符串, 未知参数和其他四种机制并存. 已学习宠物和显式配置NPC使用同一解析器, 冻结技能归属, 拒绝角色指令和非法目标.
- 手下留情覆盖非致死, 恰好致死, 过量伤害, 1HP目标, 暴击, Guard, 闪避及最低伤害0/1. 与同种子的普通物理比较随机抽取次数; 只有致死伤害被限制, Normal/Critical/Guard不因限伤成0而丢失. 原始伤害0不保留暴击, 有效Guard对应ALLGUARD. 保留1HP时不倒地, 不击飞, 不清理中毒或累计过量伤害; 后续普通攻击仍能击杀.
- 手下留情不得参加合击或消费合击资格随机; 使用者行动前后不能反击, 目标仍可反击. `go test -buildvcs=false ./common/gameconfig ./online -run ShowMercy -count=1` 覆盖上述服务端边界, `python -X utf8 -m unittest tool.test_skill_catalog` 检查626映射及生成稳定性.

- 毒攻击60和猛毒攻击61验证玩家宠物与NPC共用解析器但分别冻结3/-30和5/-30, 拒绝角色施放和非法目标. 主动攻击力101按原版负修正截断为71, 当回合反击保留减攻但不附毒, 下一回合恢复.
- 中毒附加概率验证等级差上下限, 幸运, 毒抗, 基础体力比例和严格小于边界; 已有毒、石化、睡眠、混乱或酒醉等任一普通异常时在抽数前拒绝附毒, MISS或0伤害不进行附毒概率判定.
- 普通毒伤验证人物整点和宠物100倍固定点的一致结果, 先求和再截断, 最低1点伤害, HP最低留1, HP=1时Damage 0. 毒攻击turn3写运行态4, 连续3次扣血后第4次只Remove; 猛毒turn5写运行态6, 连续5次扣血后第6次只Remove. 单次持续也必须先扣1次、再于下一次行动解除. 最后一次毒伤后仍禁止重复附毒, 纯解除后才允许再次附毒. 异常精灵继续直接写turn, 不沿用状态攻击的turn+1. 连击只扣一次, 反击不扣, 合击每名成员的状态处理必须在成员攻击前出现. 状态步骤不得污染顶层声明目标, 包括原本无目标的待机.

- 石化攻击80/509验证玩家宠物与NPC分别冻结3/-30和9/-30, 拒绝角色、未持有和非法目标. 攻击力101应向零截断为71, 主动段按普通命中、暴击、Guard、最低伤害、守护转移、死亡、击飞和反击链执行; 当回合反击保留减攻但不附石化, 下一回合恢复.
- 正伤害附石化必须先解除Sleep, 再检查其余普通异常; 冲突不抽随机, MISS和0伤害不唤醒也不判定. 概率覆盖等级差、幸运、石化抗性、体力比例、80上限、无下限及严格`<`. 成功分别写4/10, 每一点都阻止行动, 中间只Update、最后只Remove且没有空Action/Wait. 防御翻倍, 受伤不解除, 不刷新, 死亡和离场清理. 合击在执行前排除本回合新受控成员, 单人降级为普通攻击. 运行`go test ./common/gameconfig ./online -run 'StoneAttack|CombatControlStatusesFollowOriginalActionTiming|CombatComboDropsNewlyControlledMember' -count=1`覆盖该链路.
- 混乱攻击90/510验证双端配置冻结3/-30和9/-30, 玩家宠物/NPC入口、角色和未持有拒绝、101攻击向零截断为71、守护转移、反击及回合恢复. 正伤害后按Damage、Sleep Remove、Confusion Add顺序输出; 已有其他普通异常时不抽状态随机, 成功分别写4/10.
- 混乱运行态验证前3/9次行动先Update再按80%判定改写, 最终1->0只Remove并执行原动作. 改写严格按判定、阵营、0-9起点三次随机顺序, 从下一格循环扫描, 排除自身、隐藏、死亡和离场, 无候选才回退普通敌方目标. 覆盖友伤、Guard/NoGuard/Guardian清理、首名脱离合击和尾成员继续合击. 运行`go test ./common/gameconfig ./online -run 'Confusion|CombatControlStatusesFollowOriginalActionTiming' -count=1`覆盖该链路.
- 催眠攻击110/511验证玩家宠物与NPC冻结3/-30和9/-30、非法入口和目标拒绝、101攻击向零截断为71、反击及回合恢复. 覆盖`Damage -> Sleep Remove -> Sleep Add`重施顺序、其他异常拒绝且不抽数、睡眠抗性和严格概率边界、4/10次行动取消、最终Remove、姿态清理及合击排除. 公共唤醒回归覆盖普通单段、多段、反击、合击和其他特殊物理正伤害; MISS/0伤害不唤醒, PoisonAttack主动正伤害仍不唤醒. 运行`go test ./common/gameconfig ./online -run 'SleepAttack|Physical.*Wake|CombatControlStatusesFollowOriginalActionTiming' -count=1`覆盖该链路.
- `go test ./common/gameconfig ./online -run Poison -count=1` 覆盖上述60/61中毒链路及现有protobuf序列化往返, 同时检查死亡或离场不继续扣血, 物理致死时清理中毒.
- 未持有技能即使存在于 `技能.yaml` 也必须拒绝.
- 未实现行为即使已配置并持有也必须拒绝, 不允许回退成普通攻击、防御或待机.
- 角色提交逃跑后, 同一请求必须把仍存活战宠的本回合动作锁定为防御并确认两个单位; 已提前到达的宠物动作也由该联动规则覆盖.
- 玩家宠物快照必须来自 `PetRecord.skill_id_list`; 开战后修改模板或档案不得改变本场单位技能.

敌方 NPC 战斗:

- 敌人技能、权重和目标策略全部来自 `enemies[].battleAI` 引用的 AI, 并在建房时深拷贝到服务端运行态. 同一宠物模板的多个敌人可使用不同 AI, 修改全局模板、AI或另一个敌人的快照不得影响本敌人.
- 敌方 `CombatUnit.skill_id_list` 必须为空, 客户端不接收敌人技能槽; AI 和技能归属校验只读服务端运行态.
- AI 按 `skills[]` 顺序划分权重区间, 一次 `RAND(0,totalWeight-1)` 选择技能, 单技能 AI 也必须恰好消费一次随机抽取; 区间边界及目标选择的随机抽数保持可复现. 逃跑动作和结果必须携带真实技能 ID `8000003`.
- 待机、破除防御、连续攻击、一击必杀和旅程伙伴通过显式技能 ID 与权重选择; 旅程伙伴AI目标范围应配置为玩家战宠, 执行期仍复核非角色约束. 捕获、换宠、使用道具、更换装备及其他未实现 NPC 行为必须在启动阶段直接报错. 玩家宠物不得从敌人 AI 获得额外技能.

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
- 请求和回复的 `action` oneof 必须且只能设置一个分支; 非队长组员提交 `team_enabled` 且值变化时返回 `FailedPrecondition`, `duel_enabled` 不受此限制.
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
- `scene/70000.yaml` 的平铺遇敌配置必须唯一引用敌人组70000. 服务端 scene 配置不得包含 `presentation` 或 `battleBgmIndex`; 任务挑战入口和BGM分别读取`task.yaml`的`challenge.enemyGroupId`与`battleBgmIndex`.
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
- 手工联调2-5个在线角色: 目标在队伍页开启组队, 请求者输入完整目标 aid 和角色 UUID 后点击加入. 成功后新成员、队长、原队员和同地图无关角色都收到各自 UUID 的 `team_join`, 事件中的完整队伍顺序一致; 离开、踢出或解散只通过 `team_leave`/`team_disband` 更新队伍节点, 客户端据事件命中自己触发队伍页刷新, 不再有单独的点对点队伍变化通知. 验证离开、踢出和解散不弹确认框, 测试、练级或任务地图中的显示队伍节点继续由地图事件独立更新.

## 测试、练级与任务地图角色列表

聚焦测试命令:

```bash
GOCACHE="$PWD/.gocache" go test ./online -run "^(TestCharacterMap|TestScenePresenceCharacterMap|TestCharacterTeamMutationCarriesSmallMapEvents|TestSelectCombatPVEMapEnemyGroupUsesFlatEncounter)" -count=1
```

- `CharacterBaseRecord` 必须保留字段号18和字段名 `scene_id`, 但不得定义或持久化该字段. 测试、练级或任务地图 ID 只保存在 `character.sceneID` 运行态; 进入非0地图后离线必须归零并移除旧 Presence, 再次上线不得恢复旧地图, 且应允许重新进入同一地图. 单人从非0地图请求 `map_id=0` 时应将运行态设为0、从旧 Presence 移除、只向旧地图广播 `map_leave`, 并返回 `map_id=0` 和空队伍列表; 地图0本身不得创建 Presence 或发送地图事件. 已在0再次请求0必须返回 `AlreadyExists`.
- 战斗中或已组队角色请求地图0必须返回 `FailedPrecondition`, 不自动离队或解散. 非0目标只接受任务范围`[70000,79999]`、测试范围`[80000,89999]`或练级范围`[90000,99999]`中配置存在且 `encounter.enabled=true`、`encounter.enemyGroups`有效的地图. 单人进入只迁移自己; 队长进入迁移完整队伍; 普通成员主动进入和任一成员战斗中必须拒绝.
- 非0地图 Presence 只保存成员展示数据, 不保存坐标、朝向或格子索引; 移动和转向请求必须拒绝. 自动遇敌只使用当前地图平铺的 `encounter.enabled` 和 `encounter.enemyGroups`. 开启自动遇敌时角色必须存在于当前非0地图 Presence; 离开到地图0必须取消 timer 和开关, 并只向本人回复 `CombatAutoEncounterSetRes(enabled=false)`.
- 非0地图的 `CharacterMapEnterRes.team_list` 必须排除自己的队伍. 队伍节点长度为1时按单人展示, 大于1时第一项是队长、后续项是队员.
- `map_join` 携带完整新队伍节点; `team_join` 向同地图全部观察者携带合并后的完整队伍, 客户端据成员身份原子移除旧队伍节点并把新队伍追加到末尾, 接收者属于该队时不得显示自己的队伍. `map_leave`、`team_leave` 和 `team_disband` 只携带定位所需的角色键. 队长直接离开地图只发送队长 `map_leave`; 队伍解散后队长单人离开依次发送 `team_disband` 和 `map_leave`.
- 列表保持加入顺序. 成员离队后追加为单人, 解散后原成员依次追加为单人, 离开地图只删除条目且不重排其他条目.
- `MapCharacterInfo.in_combat` 必须反映 Presence 当前战斗状态; 单人或队长组队开战、个人脱离及战斗结束都发送完整 `character_update`. 队长遇敌必须只拉入冻结名单中通过入场校验的成员.
- 角色可见资料变化向同地图全部观察者(含同队成员)发送完整 `character_update`; 原始 exp 改变但显示等级不变时不得发送, 跨显示等级时必须发送. 同队接收者把该消息应用到队伍成员资料, 不得把同队角色插入地图他人列表.

## 角色属性加点与重置

- `CharacterAttributeAddReq` 和 `CharacterAttributeAddRes` 的消息 ID 分别保持为 `0x00100B` 和 `0x00100C`.
- 体力、腕力、耐力和速度分别只增加目标字段 1 点并扣除 1 点 `available_point`; 魅力及未指定枚举必须拒绝.
- 非当前账号角色、未上线角色、战斗中角色、无可加点角色和目标字段 `uint32` 溢出必须拒绝且不得修改档案.
- cache 保存失败必须回滚账号槽位和角色内存档案; 保存成功后必须先发送 `CharacterNotify.base_changed`, 再发送成功响应.
- `CharacterAttributeResetReq` 和 `CharacterAttributeResetRes` 的消息 ID 分别保持为 `0x00100D` 和 `0x00100E`.
- 重置请求只提交最终四项属性; 当前权威总点数必须在 20 至 1000 之间, 最终四项至少分配 20 点且不得超过权威总点数, 剩余 `available_point` 必须由服务端计算并保持总点数守恒.
- 重置使用独立候选账号档案写 cache; cache 返回前不得修改 online 权威账号槽位或运行中角色档案, cache 失败时操作失败且原档案保持不变, cache 成功后才提交并发送权威变化通知和成功响应.
- 运行 `go test ./online -run 'CharacterAttribute(Add|Reset)|PrepareCharacterAttribute|PersistCharacterAttribute'` 覆盖消息 ID、字段映射、克隆隔离、边界校验、总点数守恒及持久化提交/失败不提交.

## 角色邮箱

- `CharacterMailboxGet/CharacterMailRead/CharacterMailDelete` 请求和响应 ID 保持为 `0x005000-0x005005`; 新增系统邮件通知已并入 `CharacterNotify`(`0x001011`)的 `system_mail_changed` 分支.
- get/read/delete 只接受当前账号已上线角色; 非法参数、非本账号角色和离线角色分别返回对应业务错误, 不导致 gateway 断线。
- `GMCommandReq.mail_add` oneof tag 保持为 12. Cache 持久化成功后必须先发送完整邮件通知, 再发送 GM 成功响应; Cache 失败时两者都不得发送成功结果。
- 主题最多 30 个 Unicode 字符, 正文最多 500 个 Unicode 字符; 正文 CRLF/CR 规范化为 LF, 非法控制字符必须拒绝。
- 运行 `go test ./common ./cache ./online -run 'Mail|Mailbox'` 覆盖文本规范化、邮箱 hash 解码、消息序列化和 GM oneof。

## 基础战斗回归

- 剧毒攻击513/707使用现代ID `8100513/8100707`, 测试作用域注入`deepPoisonAttack`参数9/-30和6/+20; RAW必须保持`pending_test`, 正式技能配置必须排除两项. 运行`GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online -run DeepPoison -count=1`覆盖宠物所有权和敌方单体目标、角色误用、C float攻击修正、睡眠解除、普通异常零抽数拒绝、运行态10/7、普通跳伤、1HP与第9/6次强制死亡、行动取消、状态清理、反击链和Counter不附毒.
- 攻击、闪避、暴击、伤害、权威 HP 和死亡结果一致.
- 10级以上且装备快照武器槽为空的玩家普通攻击按BaseLuck生成1、2、3或5至10段, 永不生成4段; 每段保持完整普通攻击伤害, 原目标死亡后剩余段改选存活敌方, 攻击者死亡、敌方全灭或段数耗尽时停止, 整组只检查一次反击.
- 玩家已装备武器时, 每次实际行动必须在命令分派前只从 `attacknum: [min, max]` 闭区间抽取一次段数; 非普通攻击仍消耗该随机值, 但不使用其段数或爪分摊. 爪普通攻击按计划总段数分摊正伤害, 原目标死亡后每个剩余段独立重选存活敌方; 弓以外的其他当前武器不分摊. 缺失装备快照不得误触发武器路径, 合击成员与反击仍保持单段.
- 开战快照使用服务器有效属性和装备实例基础属性与附加修正合计后的运气/暴击值, 元素来自实例 `element_attribute`, 额外伤害/防御继续合并当前武器和两个首饰位配置; 更换同配置但实例值不同的装备时, 战斗数值必须采用实际实例而不是重新随机.
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

`combat.capture_test.go` 验证角色请求目标和宠物技能边界, 原版 HP 平方公式、float32 阈值和严格小于判定, 敌群权限与等级限制, 账号消息确认前不得移除敌人, 携带宠物已满返回 `CombatCaptureFailureReason_Capacity=4`, 且容量不足及保存失败不得产生成功结果或新 UUID. 个体测试使用独立固定向量验证 SavedBase 在十点初始加点之前保存, 捕获档案的 Raw、等级、品阶、出生技能和成长基线不重抽, 修改模板或已创建记录不会污染快照. 持久化测试覆盖完整候选记录提交、容量检查前置、UUID 耗尽、角色槽缺失和 cache 失败回滚.

```bash
GOCACHE="$PWD/.gocache" go test ./online -run Capture -count=1
GOCACHE="$PWD/.gocache" go test ./common/pet ./online ./proto/pb
```
