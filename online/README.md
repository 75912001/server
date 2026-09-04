# Online 服务

Online 服务负责 Account actor、账号业务、角色业务和 PVE 战斗. gateway 负责连接、登录会话、顶号和断线编排.

部署、端口、容器启动和验证命令见 `deploy/online/README.md`.

## 能力边界

Online 当前负责:

- `OnlineBindAccount` 和 `OnlineUnbindAccount`.
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包并返回业务响应.
- 读取、校验和持久化 `AccountRecord`.
- 角色创建、上线、下线、场景切换和自动遇敌.
- 同一 Online 实例内的基础队伍加入、离开、解散和踢出.
- 宠物携带状态、仓库存取和昵称.
- 道具使用、背包和仓库存取、全局武器与首饰商店购买, 以及角色武器和两个首饰位的装备、卸下和替换.
- 在线角色邮箱的获取、已读、删除和新增邮件通知.
- GM 结构化命令, 当前支持增加道具、宠物和系统邮件.
- 每场独立 `CombatRoom` actor 的单人或1-5人组队 PVE 回合、动作和结算.

Online 不负责:

- 写入或续期 cache account session.
- 顶号和连接断开.
- 客户端图片、sprite、`.tpsheet` 或动画帧完整性校验.

业务失败只返回对应业务错误码. Online 返回的 `InvalidArgument`、`FailedPrecondition`、`NotFound` 或 `Internal` 不表示要求 gateway 断开用户连接. gateway 只有在连接协议、会话安全或服务自身明确进入断线流程时才断开.

Online 运行配置将 `grpc.maxReceiveMessageBytes` 和 `grpc.maxSendMessageBytes` 都设为 `67108864`, 即服务端和生成客户端的单条消息收发上限均为 64MiB.

## 共享游戏配置

`custom.gameConfigDir` 指向共享配置目录, 未配置时默认读取当前工作目录下的 `config`.

Online 启动必需文件:

```text
character.yaml
skill.yaml
ai.yaml
enemy.group.yaml
enemy.exp.yaml
exp.yaml
item.yaml
item.weapon.yaml
pet.yaml
scene/*.yaml
```

Docker 镜像会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`.

配置加载依次执行单表 `load`、跨表 `check` 和 `assemble`. 文件缺失、字段非法或引用无效时直接启动失败, 不继续注册 etcd 或启动业务服务.

`scene/*.yaml` 不设置格式版本字段, 地图 ID 与客户端 `map_id` 完全一致. 当前目录包含80000、80001、80010、80020、80030、80040、80050、80060、80070这9张测试地图, 从90001开始的14张练级地图: 90001和90010至90130按10递增, 以及1000701至1000703这3张任务地图. 角色正常进入接受协议测试地图范围`[80000,89999]`、练级地图范围`[90000,99999]`和任务地图范围`[1000000,1999999]`. 每张地图通过平铺的 `encounter.enabled` 和 `encounter.enemyGroups` 定义全地图遇敌规则, 自动遇敌不接收或读取角色坐标. 自动遇敌通过 `scene/*.yaml -> enemy.group.yaml` 取得敌人条目, 再分别读取宠物模板与 AI 生成单位; 服务端按权重选择敌人组, 敌人组的 `enemies[].id` 直接引用宠物模板, 数量和等级来自敌人组, 属性来自宠物模板, AI 由敌人条目的 `battleAI` ID 显式引用, 经验由 `enemy.exp.yaml` 结合宠物模板计算. 每个敌人的技能、权重和目标策略均来自 `ai.yaml`, 建房时深拷贝到服务端战斗运行态, 不下发客户端.

Boss 每个敌人成员必须在 `enemies[].level` 和 `enemies[].levelRange` 中二选一, 同时配置、两者缺失或等级越界时加载失败. 建房时固定等级直接使用配置值, 区间等级按该成员的闭区间独立抽取一次, 并用于本场敌人属性和战斗运行态; 普通组未指定成员等级时继续使用组级等级规则. 阿布洞窟1000701的五名守护者使用原版区间 `[39,41]`、`[38,40]`、`[37,39]`、`[35,38]`、`[40,43]`, 不再固定为区间中间等级.

## 角色任务

运行配置来自`task.yaml`和`reward.yaml`, 原版调研资料不进入Online运行时. `CharacterRecord.task_record_map`按任务ID保存已接记录, 每项只包含接取时间和完整的有序步骤数组. 步骤使用`step_id/started_at_ms/completed_at_ms/reward_claimed_at_ms`, 由第一个未完成步骤推导当前步骤, 不另存当前步骤ID或总任务状态.

`TaskAcceptReq/Res`、`TaskSubmitReq/Res`、`TaskStepRewardClaimReq/Res`和`CharacterTaskChangedNotify`使用消息范围`0x007000-0x007006`. `CharacterTaskInventoryChangedNotify`使用`0x007009`, 在提交宠物/装备或领取复合奖励后下发角色完整背包、完整随身宠物列表、角色资产表和账号UUID游标. 接取、提交和领取只允许当前账号内已上线且不在战斗中的角色. 请求只携带角色、任务和必要的步骤ID, 不接收客户端自报进度、条件结果或奖励内容.

`TaskBattleChallengeReq/Res`使用`0x007007/0x007008`. 挑战只允许当前账号内已上线且不在战斗中的角色, 请求携带角色UUID, 任务ID和所选步骤ID. 服务端要求任务已接取, 所选步骤已经开始且配置了`challenge`, 再从配置读取`enemyGroupId`进入既有PVE建房流程; 不要求地图NPC在场, 不接收客户端自报敌群或BGM. 已完成步骤也可再次挑战, 不限于当前未完成步骤.

任务管理器在候选账号档案上校验条件并推进串行步骤. `itemPossession`支持普通道具、角色资产和装备实例, `petPossession`按宠物ID与实际等级精确匹配. 提交先核对全部完成条件, 再选定并扣除对应装备UUID和宠物UUID. 奖励可以包含道具、随机属性装备及指定等级的随机品质宠物. 任务记录、库存、UUID游标与领取标记共用一次账号持久化; 任一校验、背包或宠物栏容量、数量溢出、UUID耗尽或Cache失败均不得提交部分状态. 步骤奖励在该步骤完成后即可领取, 不要求整个任务结束, 也不阻塞后续步骤; 无奖励步骤在完成时直接把领取时间设为完成时间.

成功持久化后, 库存变化操作先发送`CharacterTaskInventoryChangedNotify`, 再发送任务变化通知并回复成功. 客户端用复合库存通知原子替换背包、宠物、资产和UUID游标, 避免消费与发奖之间出现中间状态. `CharacterTaskChangedNotify.changed_task_record_map`仅包含本次实际变化的任务, 每个变化任务携带完整步骤记录, 未变化任务不下发. 客户端用该增量按ID合并, 可接任务和页签状态由客户端配置与已有记录推导.

PVE建房时冻结敌群ID. 正常胜利结算由服务端直接调用角色任务管理器, 与经验等战斗结果共用持久化/回滚流程, 不依赖客户端播放完成消息或再次提交战斗凭据. 已逃跑、击飞或脱离战斗的角色不获得后续胜利进度. 阿布洞窟任务60只判断敌群1000701, 不额外绑定NPC实体ID; 任务挑战BGM由客户端读取`task.yaml`步骤的`challenge.battleBgmIndex`, 场景NPC挑战使用客户端NPC表现配置, 服务端不负责音乐选择或播放.

重打只创建新的战斗, 不重新接取任务, 不重置任务接取时间及已完成步骤的开始, 完成或领奖记录. 无奖励, 待领取奖励和已领取奖励三种状态均保持原样, 不重复发放步骤奖励. 普通战斗经验照常结算, 其他任务仍按原有条件判定; 已完成任务没有实际变化时不发送其任务记录增量.

任务记录绑定时校验任务存在、步骤数量与ID、时间戳顺序、串行状态和无奖励已领取标记. 普通任务不支持重接或放弃; `repeatable: true`的任务在最后一步奖励领取并确认全步骤均已领奖后, 使用当前时间重置接取时间和全部步骤记录, 随后立即尝试启动新一轮首步. 任务61`[任务][卡坦的愿望][1]`完成并领取全部奖励后才可接任务62; 任务62按该循环规则无限重复. 当前不迁移旧任务档案, 也不离线补发奖励. 阿布水购买资格以后直接读取任务60是否完成, 购买入口尚待接入.

## 角色道具与独立资产

角色道具统一通过 `characterItemManager.Count/Add/Consume` 访问. `[3000000,3489999]` 的普通可堆叠道具存入 `item_bag.item_count_map` 并占用背包种类容量; `[3490000,3499999]` 的角色独立资产存入 `CharacterRecord.asset_count_map`, 不占背包容量且禁止存入账号共享仓库. 两类数据都必须存在于 `item.yaml`, 数量为 0 时删除映射键, 查询缺失键返回 0. 角色独立资产的合法数量上限为 `math.MaxInt64`, 避免超出 Godot 有符号 64 位整数范围.

协议 `AssetID` 枚举当前定义 `AssetID_Stone`、`AssetID_Silver` 和 `AssetID_Gold`. `CharacterItemChangedNotify` 对两类道具都携带最终数量, 调用方按 ID 范围选择目标容器. cache 档案中的角色资产若范围非法、数量超限、缺少配置或误存入角色背包/账号仓库, 账号绑定直接失败; 项目不迁移旧 `stone/shell/diamond` 字段.

## 全局装备商店

`ShopPurchaseReq` 购买 `item.weapon.yaml` 和 `item.accessory.yaml` 中 `cost > 0` 的武器或首饰, `cost` 当前固定使用 `AssetID_Stone` 计价. 商店没有 NPC、场景、库存或限购条件; 服务端按当前配置价格结算, 并校验角色属于当前账号、已经在线且不在战斗中, 以及数量、石币资产、背包剩余格数和账号 UUID 游标. `cost = 0`, 非武器或首饰 ID, 类型与 ID 不匹配或数量非法都必须拒绝.

购买时先在账号档案副本中通过统一道具接口扣除石币资产, 为每件装备创建 `EquipmentRecord`. 每个实例保存 `uuid`、`asset_id`, 并把攻击、防御、敏捷、HP、MP、运气、魅力、闪避、六种状态抗性和暴击共15项配置闭区间分别随机一次, 完整固化到 `record_base_map`; 即使实际值为0也保留对应 key. 随后调用 cache 持久化完整 `AccountRecord`. cache 成功后才提交 Online 内存档案; 任一校验或持久化失败都不修改资产、背包和 `used_uuid`. `ShopPurchaseRes.affected_item` 返回 `AssetID_Stone` 及扣除后的最终持有数量, 同时返回总价、最新 UUID 游标和本次新增装备列表, 供客户端原子替换权威快照.

## 角色装备换装

`CharacterEquipmentReplaceReq` 和 `CharacterEquipmentReplaceRes` 的消息 ID 分别为 `0x00100F` 和 `0x001010`. 请求携带角色 UUID、目标部位和背包装备 UUID; 当前接受 `EquipmentType_Weapon`, `EquipmentType_Accessory1` 和 `EquipmentType_Accessory2`, `equipment_uuid=0` 表示卸下目标部位装备. 角色必须属于当前账号、已经在线且不在战斗中. 装备时还校验实例确实位于该角色背包、装备配置及穿戴部位匹配、角色等级满足要求; 当前角色档案没有职业字段, 因此带 `neprof` 限制的装备明确拒绝.

首饰位1和首饰位2分别保留原部位枚举数值1和3, 穿戴字段保留原 protobuf tag 1和3. 耳环, 护身符, 戒指, 乐器, 手环和项链可以进入任一首饰位, 但两个部位的 `accessory_type` 必须不同, 即使道具 ID 和实例 UUID 不同也不能同时穿戴同类. 同一部位替换同类允许. 账号绑定, 换装候选和有效属性计算均校验这条约束.

换装在 Account actor 内串行构造完整账号候选档案. 装备或替换时从背包移出新装备, 并把目标部位的旧装备放回背包; 卸下时要求背包仍有空位. cache 持久化成功后才同时提交角色背包、穿戴记录和运行中角色引用, 失败时权威档案不发生任何变化. 成功后刷新场景 Presence 中的武器类型, 并通过统一场景表现变化通知把 `weapon_type` 广播给当前可见的其他角色; 换装角色本人使用成功响应返回的完整 `item_bag`、本次替换部位的 `equipment` 和服务器重算后的 `effective_attribute` 原子更新本地档案, 卸下时 `equipment` 不设置.

账号绑定、角色上线和角色基础数据变化都会下发服务器派生的有效属性. 有效属性以角色基础值和当前装备实例的固化值计算, 并包含最大HP、攻防敏、最大MP、有效运气/魅力/闪避、暴击、六种状态抗性、元素、额外伤害/防御以及实际武器类型. 该快照不持久化; 每次换装或基础属性变化后重新计算. 当前不兼容旧装备实例: `record_base_map` 缺项、多项、越界或包含额外 key 时, 账号绑定、仓库存取或业务使用直接失败.

## 宠物创建与品阶

创建宠物时显式传入 `Common` 到 `Mythic`, 会分别对四项 SavedBase 统一应用 `-2`、`-1`、`0`、`1`、`2` 偏移. 传入 `Unknow` 表示随机品阶: 四项分别独立等概率随机整数偏移 `[-2,2]`, 再按总偏移计算并保存实际品阶, `PetRecord.grade` 不会保存 `Unknow`.

宠物实例的四项 `SavedBase*`、四项 `Raw*` 以及成长基线中的防御、敏捷统一以 `int32` 持久化和传输, 品阶偏移或原版公式产生0、负数时直接保留, 不做“必须大于0”的额外保护. 创建、绑定档案和进入战斗仍会检查转换溢出, 并要求派生后的最大生命和攻击大于0; 防御、敏捷允许保持有符号结果.

`pet.yaml creationMode` 省略时表示普通宠物; `fusionEgg` 表示原版融合蛋模板. 融合蛋保留原始 `growth` 数据, 包括海宠蛋的 `10/0/0/0`, 但 `common/pet.NewRecord`、GM 增加宠物和角色默认宠物链路都不得用这些占位值生成普通宠物. 原版融合蛋的实际四维由亲本继承并带随机偏移, 后续接入融合系统时必须使用独立创建流程.

随机品阶按总偏移划分为 `Common=-8~0`、`Rare=1~2`、`Epic=3~4`、`Legendary=5~6`、`Mythic=7~8`, 对应概率为 `56.80%`、`23.68%`、`13.92%`、`4.80%`、`0.80%`.

## 角色声望

`CharacterBaseRecord.reputation` 保存角色声望内部值, `100` 表示显示 `1.00`, 上限为 `100000000`. 声望规则和上限全部由 Online 负责; Cache 只持久化字段. Online 绑定账号档案时拒绝超过上限的声望, 新角色依靠 protobuf 零值从 `0` 开始.

人物每到达新等级 `L`, 按 `GetLevelMinExp(L) / 20000` 增加声望. 宠物每到达大于30的新等级 `L`, 按 `GetLevelMinExp(L-1) / 20000` 增加所属角色声望. `GetLevelMinExp` 对应8.5原版 `LevelUpTbl` 的同级值, 不能改用相邻累计等级门槛的差值. 精确达到升级门槛仍然结算; 连续提升多级时逐级计算; 整数除法直接舍弃小数且不设置最低奖励.

人物经验, 人物升级副作用和声望共用一次角色档案持久化. 宠物经验, 成长和主人声望也在同一个角色聚合根中提交. 战斗或经验道具持久化失败时不得发布部分状态; 宠物经验道具实际增加主人声望时, 同时发送角色基础变化和宠物变化.

## 统一技能模型

`skill.yaml` 是角色和宠物共用的唯一技能定义来源. 可选 `cost` 字段决定宠物能否学习及基础石币价格; 字段缺失不可学习, `cost: 0` 表示免费开放. `pet.yaml skill` 只保存新宠物的出生技能槽; `ai.yaml skills[]` 直接保存敌人的技能 ID 和选择权重, `enemy.group.yaml enemies[].battleAI` 必填且显式引用 AI. 同一种宠物可以在不同敌人条目中使用不同 AI.

玩家宠物创建时把模板固定 7 槽复制到 `PetRecord.skill_id_list`, 之后实例技能栏独立于模板. `PetSkillSetReq` 只允许当前账号的随身宠物在非战斗状态修改槽位: `skill_id=0` 免费遗忘且不退款, 非 0 技能按 `skill.yaml cost` 学习或直接覆盖原槽位, 替换不抵扣旧技能价格. 服务端不信任客户端价格; 它在账号候选档案中同时修改宠物技能和石币, 一次写入完整 `AccountRecord`, cache 失败时两者都不提交. `PetSkillSetRes` 返回宠物 UUID、目标槽位、最终技能、实际基础价和石币最终数量.

当前角色动作:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8000003`: 逃跑.
- `8000004` 捕获: 角色选择存活敌方目标, 服务端结算 Capture, 成功保存宠物后追加 UnitLeave(Captured).
- `8000005-8000007`: 配置保留, 当前未开放. 请求返回 `InvalidArgument`, 不执行动作.

当前玩家宠物解析器支持:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8100000`: 待机.
- `8100003`: 破除防御.
- 配置了 `continuationAttack.segmentCount` 的连续攻击.
- 配置了 `chargeAttack` 的突击, 当前为 `8100030/8100031`.
- 配置了 `mightyAttack` 的一击必杀, 当前为 `8100040-8100042`.
- 配置了 `poisonAttack` 的猛毒攻击, 当前为 `8100061`.
- 配置了空对象 `showMercy` 的手下留情, 当前为 `8100626`.

待机不需要目标, 生成一个携带技能ID和出手单位、但没有效果的实际动作步骤. 破除防御先执行一次完整普通物理判定: 目标本回合采取有效防御时跳过 `BATTLE_GuardAdjust` 及其随机数, 使用未经过防御姿态减伤的物理伤害, 但不会清除目标的防御命令或降低防御属性; 目标未防御时覆盖为0伤害MISS. 两种结果都不参加合击或反击链.

手下留情原版626映射 `8100626`, 学习价10000石币, 无单次施放费用. 玩家宠物使用冻结的7槽技能, NPC必须由冻结AI显式配置; 两者共用解析器, 角色指令及非法目标拒绝. `showMercy` 只接受空对象并与其他四种攻击机制互斥. 技能复用一段普通物理攻击, 不新增随机抽取. 在闪避, 暴击, Guard和最低伤害计算后, 实际扣血及死亡/击飞判断前, 把致死伤害限制为目标当前HP减1. 1HP目标受命中时可以产生0伤害, 保留限伤前的Normal/Critical及Guard; 原始伤害已为0时按原版MISS/ALLGUARD处理. 不积累虚假的过量伤害, 不触发倒地, 击飞或毒状态清理. 此规则只约束本次攻击, 后续其他攻击仍可击杀目标.

手下留情不参加合击, 始终保留专用命令, 使用者行动前后均无反击资格; 普通命中或闪避后, 目标仍按原有资格及概率反击, 暴击和有效Guard仍中止反击. 战报使用现有Damage和HP delta, 不扩展协议或资源. 原版限伤早于针刺等特殊反应及骑宠分摊, 后续接入这些机制时需保留顺序; 当前不提供持续免死或捕获能力.

突击30和双重突击31分别映射 `8100030/8100031`, 使用 `chargeAttack: {chargeRounds: 1/2, attackPercentModifier: 90/110}`, 学习基础价4000/8000石币. 编辑器和客户端的 `description` 保留原版文字; 本段为内部机制说明. 30在首次行动蓄力, 第二次行动释放一击; 31在前两次行动蓄力, 第三次行动释放一击. 剩余计数减到0的当次行动仍只蓄力, 不提前出招. 释放时按原版 `pow += pow * Per * 0.01` 修正当时的基础攻击力, 再走普通物理公式; 约190%/210%攻击力不等于最终伤害固定乘1.9/2.1.

服务端在单位运行态保存技能ID, 目标和首次提交时复制的参数, 不把蓄力写成异常状态. 后续回合直接锁定宠物续招动作, 超时只给仍缺少动作的单位补防御, 重复请求不能覆盖蓄力. NPC等待和释放时沿用保存的命令, 不重新抽技能或目标. 原目标死亡或离场时, 只在实际释放的一击按普通物理规则重选存活敌人. 蓄力不额外抽攻击随机数, 不参加合击, 不产生新特效或声音; 释放复用普通攻击. 突击者在等待和释放回合均不能反击, 受击目标仍按普通资格及概率反击.

非结束战报通过 `CombatRoundResultNotify.next_round_auto_action_unit_key_list` 下发下一回合已锁定的玩家单位. 客户端在战报播放完成后先应用本角色的锁定, 再启动自动战斗和超时补交, 因而蓄力和释放回合都跳过宠物选择; 释放后的下一回合恢复选择, 角色仍正常操作. 普通非致死受击不取消蓄力, 已中毒者在每次蓄力或释放行动前正常结算一次毒伤. 死亡, 逃跑, 击飞, 外部脱离和战斗结束清理续招. 当前玩家阵营仍按角色全灭判负, 不让战宠独自续打. 本次保留当前仅选敌方目标的规则, 不自动修改宠物出生模板或敌方AI; 原版不可行动状态取消蓄力和低忠诚分支待对应运行态接入.

一击必杀三档参数分别为2倍伤害/+30目标闪避、3倍/+40、4倍/+50. 解析动作时复制配置参数, 只执行一段普通物理攻击: 闪避加值先计入基础公式再封顶75%, 最终伤害在暴击、Guard和最低伤害之后按C float倍率计算. 原版后置装备闪避仍独立判定. 该动作不参与合击, 行动前不能反击, 开始出手后取得普通反击资格; 反击使用新建的普通Attack动作, 保留声明技能ID但不继承一击必杀参数. 客户端复用普通物理步骤, 不强制暴击或额外技能特效.

猛毒攻击按原版8.0实际加载的 `petskill2.txt` 使用 `poisonAttack: {durationActions: 5, attackPercentModifier: -30}`, 学习基础价4000石币. 玩家宠物和敌方NPC共用解析器, 技能归属和参数均取本场快照. 施放者执行单段普通物理攻击, 攻击力按 `FIXSTR + int(float32(FIXSTR) * -0.30)` 修正, 不参加合击; 主动行动开始后取得反击资格, 当回合后续反击仍使用该攻击修正, 但反击不附毒. 下一回合恢复攻击力.

主动段造成正伤害后才尝试附毒. PVE阈值为 `min(80, int(30 + clamp(2*等级差, -40, 40) + 施放者幸运 - 目标毒抗 - 40*目标基础体力/目标基础四维总和))`, 保留原版C float的中间舍入和 `RAND(1,100) < 阈值` 的严格比较. 已中毒不叠加, 不刷新, 不额外抽数. 目标基础四维和毒抗均在开战时冻结; 四维总和为0属于原版除零输入, 当前明确拒绝附毒.

附毒写计数5, 表示剩余毒伤次数. 中毒者每次正常行动开始先结算毒伤再减1, 前4次下发Damage和状态Update, 第5次在同一Damage中附带Remove, 立即解除中毒和头顶标记. 解除后允许再次附毒, 不再保留原版等待下一次行动才解除的间隔. 合击成员各结算一次, 连击各段和反击不重复结算. 基础毒伤为 `max(1, (基础四维整点总和 - 20) / 4)`, 整数除法向零截断; 角色档案四维已是整点, 宠物四维必须先相加再除100. 实际扣血不超过 `当前HP-1`, HP=1时仍下发Damage 0并照常消耗次数, 最后一次同样立即Remove. 毒伤不经过物理命中, 防御, 暴击或随机伤害公式. 服务端以现有Status步骤及HP/状态delta下发, 客户端只播放结果. 当前覆盖人物, 战斗宠物和敌人; 独立骑宠HP承伤模型尚未接入.

敌方 NPC 解析器额外支持 `8000003` 逃跑. 所有敌人动作统一按 `skills[].id` 和 `skills[].weight` 选择, 每项权重必须是正整数, 总权重不超过2147483647; 不使用的技能不配置. 技能列表不限制7项, 同一 AI 不得重复配置技能 ID. 回合中只使用开战时冻结的 AI, 不读取宠物出生技能. 捕获、换宠、使用道具、更换装备或其他尚无 NPC 执行器的 AI 技能会使 Online 在注册 etcd 和 gRPC 服务前直接启动失败.

玩家宠物使用技能必须同时满足:

1. 技能存在于 `skill.yaml`.
2. 技能 ID 出现在来自 `PetRecord` 的开战快照 `CombatUnit.skill_id_list` 中.
3. Online 已实现对应行为.
4. 请求目标和阵营参数合法.

当前 `pet.yaml` 中多数宠物初始配置 `[8000001,8000002,0,0,0,0,0]`; 阿布洞窟守护者中的查罕·乌尔夫和查罕·吉鲁额外在第3槽配置 `8100003`. 14个练级组引用 AI 1, 按 `10:1:1` 权重随机攻击、防御或逃跑; 其余普通敌人引用 AI 18, 按 `10:1` 攻击或防御. 两个守护者引用 AI 19, 仅按 `10:1` 权重攻击或防御. 玩家可在设置页改变自己的实例技能, 本场玩家战宠始终使用开战时复制到 `CombatUnit` 的不可变快照, 战斗中不允许学习或遗忘; 敌方 `CombatUnit.skill_id_list` 始终为空.

未知技能、未开放技能、未持有技能和非法目标都返回 `CombatRoundActionRes` 的 `InvalidArgument`. 该响应不会触发 gateway 断线.

## 角色交互设置

`CharacterSettingSetReq` 修改当前账号登录会话内指定角色的组队和决斗开关. 状态保存在 Account actor 的角色管理器中, 不进入 `CharacterRecord`, 也不调用 cache 持久化.

每次 `OnlineBindAccount` 创建新的角色管理器时, 所有角色的组队和决斗开关都初始化为关闭. 客户端发送修改请求后保持原显示状态, 仅在收到 `CharacterSettingSetRes` 成功回复后应用服务端返回的权威值; 请求失败时继续显示原状态.


## 基础队伍

`CharacterTeamOperationReq` 提供加入、离开、解散和踢出四种基础操作, 队伍最多5人. 加入请求必须提交完整的 `join.target`, 服务端只从同一非0地图的场景成员索引核验该目标, 不读取朝向、坐标或距离: 未组队角色或实际队长必须开启组队开关且不在战斗中; 普通成员不能作为加入目标. 操作目标统一使用 `aid + character_uuid`, 同一账号的不同角色可分别加入. 当前队伍只存在于同一 Online 实例的运行内存, 成员顺序使用紧凑列表, 队长固定为第一项, 离队或踢出后后续成员前移, 新成员追加到末尾.

`CharacterTeamOperationReq` 和 `CharacterTeamOperationRes` 均通过 `join`、`leave`、`disband`、`kick` oneof 表达具体操作, 每个请求必须且只能设置一个分支. `join.target` 和 `kick.target` 都必须提供完整 `aid + character_uuid`; 四种成功回复只返回对应的空 oneof 分支, 不返回候选或队伍状态.

所有操作都在服务端成功后向受影响角色发送 `CharacterTeamChangedNotify`, 通知只携带该接收者的 `target_character_uuid`, 不下发队伍快照或地图成员列表. 客户端只把它视为本角色队伍关系已变化的信号, 不预改服务端权威状态. 战斗中禁止切换组队开关以及加入、解散和踢出; 普通成员仍可主动离队, 该操作只改变队伍关系, 不修改其 CombatRoom 指针或本场战斗参与资格. 普通成员下线、成功逃跑或 Ultimate 击飞时移出队伍; 队长发生这些情况时解散整队. 普通倒地不移出或解散队伍.

队伍加入成功时, 服务端向同地图每个观察者发送 `team_join`, 包括新成员、队长、原队员和无关角色; 每个事件只替换接收者自己的 `target_character_uuid`, 但都携带按队长在首、新成员在末排列的完整 `CharacterMapGroup`. 离开、踢出、解散或强制移除继续通过对应队伍地图事件更新可见分组, 受影响角色另收 `CharacterTeamChangedNotify`. 加入成功时只关闭新队员的自动遇敌, 队长保持原状态. 服务端只允许把整队迁移到同一非0目标地图并保持队伍关系, 队伍不得进入地图0. 当前不实现队伍聊天.
普通队员在队伍期间不能操作自动遇敌开关, 服务端对开启或关闭请求统一返回 `FailedPrecondition`; 队长仍可独立控制自动遇敌.


## 测试、练级与任务地图角色列表

测试、练级或任务地图 ID 只存在于 Account actor 的 `character.sceneID` 运行态, 不属于 `CharacterRecord`, 也不写入 cache. 新建角色、角色上线和角色离线后的运行态地图均为0, 且不加入任何 Presence. `CharacterMapEnterReq(map_id=0)` 表示离开当前地图: 战斗中或已组队角色必须返回 `FailedPrecondition`; 单人离开成功后将运行态地图设为0, 从旧地图 Presence 移除并向旧地图广播 `map_leave`, 但不创建或加入地图0的 Presence, 也不产生地图0事件. 已在地图0时再次请求0返回 `AlreadyExists`, 不修改状态或发送事件.

非0目标只允许当前测试范围`[80000,89999]`、练级范围`[90000,99999]`或任务范围`[1000000,1999999]`内、配置存在且 `encounter.enabled=true`、`encounter.enemyGroups`有效的地图. 单人直接进入; 队长进入时整队进入; 普通成员不能主动切换地图; 任一相关角色处于战斗中时拒绝. 请求进入当前地图返回 `AlreadyExists`, 不修改状态, 也不发送场景通知.

非0测试、练级或任务地图是只维护成员关系的逻辑场景, Presence 仅保存角色展示数据, 不保存坐标、朝向或格子索引; 角色不能移动或转向, 自动遇敌直接使用当前地图的平铺 `encounter` 配置. 地图成员与加入顺序由当前 Online 的 `GScenePresenceMgr` 按地图加锁维护, 不跨 Online 共享. 非0地图的 `CharacterMapEnterRes` 返回除自己队伍外的全部角色分组; 地图0成功响应返回 `map_id=0` 和空分组列表. 离开到地图0时若自动遇敌已开启, 服务端同时关闭 timer 和开关, 只向本人发送 `CombatAutoEncounterSetRes(enabled=false)`.

`MapCharacterInfo` 包含 `CharacterKey` 角色标识、角色 ID、名称、转生次数、原始 exp、骑宠 ID、组队开关和战斗状态. 角色形象、名称、转生、骑宠、组队开关、战斗状态或显示等级发生变化时, 服务端发送完整 `character_update`; 仅 exp 改变但显示等级不变时不发送列表更新.

`CharacterMapEventNotify` 只维护非0地图角色列表: `map_join` 携带完整新分组, `team_join` 携带合并后的完整队伍, `map_leave` 携带离开角色键, `team_leave` 携带离队成员键, `team_disband` 携带队长键. 队长直接离开非0地图时只发送队长的 `map_leave`, 客户端据此移除整组; 队伍先解散再由队长单人离开时, 依次发送 `team_disband` 和队长的 `map_leave`. 单人进入地图0时只向旧地图发送 `map_leave`, 地图0不发送任何地图事件. 队伍变化事件先于 `CharacterTeamChangedNotify` 发送. 角色离线时从旧 Presence 移除并将运行态地图归零; 再次上线从地图0开始, 不自动重新加入上次的测试、练级或任务地图.

## 角色属性加点与重置

`CharacterAttributeAddReq` 每次只为当前账号的一名在线且非战斗角色增加 1 点基础属性. 可选属性严格限定为体力、腕力、耐力和速度, 魅力不在本协议范围内. 服务端验证角色归属、在线状态、战斗状态、可加点数和 `uint32` 溢出后, 克隆角色档案并减少 1 点 `available_point`; cache 持久化失败时保留原档案.

持久化成功后先发送 `CharacterBaseChangedNotify` 权威快照, 再回复 `CharacterAttributeAddRes`. 客户端不得乐观修改属性, 成功响应仅用于解除请求等待状态.

`CharacterAttributeResetReq` 只提交体力、腕力、耐力和速度的最终值. 服务端只接受当前账号中已上线且不在战斗中的角色, 当前权威总点数必须在 20 至 1000 之间, 最终四项至少分配 20 点且不得超过重置前的权威总点数; 剩余 `available_point` 由服务端根据权威总点数计算.

重置时先构造独立的候选账号档案并交给 cache 持久化, cache 返回失败则此次操作失败且 online 权威档案保持不变; cache 成功后才提交 online 角色档案, 随后发送 `CharacterBaseChangedNotify` 和 `CharacterAttributeResetRes`. 装备属性要求等后续限制可在持久化前增加校验并返回明确错误.

## 角色邮箱

角色邮箱由 Cache 的独立 Redis hash 持久化, 不进入 Online 内存中的 `AccountRecord`. Online 只为当前 Account actor 内已经上线且属于该账号的角色同步转发以下请求:

```text
CharacterMailboxGetReq    -> CacheGetCharacterMailbox    -> CharacterMailboxGetRes
CharacterMailReadReq      -> CacheMarkCharacterMailRead  -> CharacterMailReadRes
CharacterMailDeleteReq    -> CacheDeleteCharacterMail    -> CharacterMailDeleteRes
```

客户端在角色上线成功后读取一次完整邮箱, 之后依靠 `CharacterSystemMailNotify` 合并新邮件. 邮件不存在或已经过期时, read/delete 返回 `NotFound`; Online 不把这类业务错误升级为断线。

当前系统邮件发送入口是在线角色自己执行的结构化 GM 命令 `gm mail add <主题>|<正文>`. Online 校验角色在线和文本后同步调用 `CacheAddSystemMail`; Cache 持久化成功后, Online 先发送包含完整 `MailRecord` 的 `CharacterSystemMailNotify`, 再发送 `GMCommandRes.mail_add`. 当前不限制系统邮件发送频率, 允许登录等业务连续补发多封; 不实现每日/每周/每年计数, 角色事件管理完成后再扩展. 当前不实现玩家互发、离线投递、附件、幂等键或发送记录。

## 战斗流程

```text
CombatBattleStartNotify
  -> CombatRoundActionReq
  -> CombatRoundActionRes
  -> CombatRoundResultNotify
  -> CombatFlowCompleteReq
```

- `CombatBattleStartNotify` 下发 `battle_id` 和初始单位列表.
- 单人遇敌直接创建房间. 队长遇敌时先冻结当前同地图队伍顺序, 再由每名角色所属 Account actor 在一条同步消息中校验在线、场景和战斗状态, 读取角色及可选战宠档案, 生成满 HP 快照并绑定同一个 CombatRoom 指针.
- 入场成功的角色按冻结顺序紧凑占用0至4号位, 其可选战宠占用角色位加5. 某个普通成员入场失败时不进入战场; 仍在线的失败成员仅在仍属于原队长当前队伍时被踢出, 其他已成功成员组成残缺队伍继续开战.
- 角色和当前参战宠物分别提交动作.
- 每个单位每回合只能提交一次合法动作.
- 所有存活可控单位提交后立即结算; 超时单位使用默认防御.
- 每个参与角色分别收到一条带 `recipient_character_uuid` 的 `CombatRoundResultNotify`; `event_list` 和其中的动作、效果数组顺序就是客户端播放顺序, `settlement` 只包含该接收角色及其战宠的个人结算.
- 角色在战斗运行中下线或被清理时, Account actor 清除自己的 CombatRoom 指针并把脱离投递给房间. 房间在下一条回合结果中先下发该角色及战宠的 `UnitLeave(Detached)`, 再下发本回合其他效果; 其余参与者继续战斗, 最后一名参与者脱离时关闭房间且不生成结算或奖励.
- 逃跑成功和角色 Ultimate 击飞分别通过 `UnitLeave(Escape)`、`UnitLeave(Defeated)`只移除对应角色及战宠, 其他参与者继续. Ultimate 击飞还将该角色运行态地图设为0并移出旧地图 Presence.
- 战斗结束后结算敌人经验, 再等待客户端完成表现流程.

基础物理战斗当前保留普通攻击、防御、逃跑、待机、破防、连续攻击、合击、反击、伤害、死亡和 PVE 结算. 开战快照复制角色完整装备和服务器有效属性. 玩家装备武器时, 每次实际行动都在命令分派前从配置 `attacknum_min/attacknum_max` 闭区间锁定一次段数; 非普通攻击只保留这次随机消耗, 不使用该段数. 爪普通攻击按本次计划总段数分摊每段正伤害, 其他当前武器每段保持完整普通攻击伤害. 武器和两个首饰位实例固化的攻防敏, HP, MP, 运气, 暴击和配置元素, 额外伤害/防御进入当前开战与物理结算; 武器类型还参与反击相性, 弓、回旋镖、投掷斧和投掷石禁止双方反击. 武器槽明确为空且等级达到10的玩家角色执行普通攻击时, 继续按BaseLuck锁定1、2、3或5至10段, 不会出现4段; 真正空手的每段都使用完整普通攻击伤害. 除弓和回旋镖专用目标展开外, 原目标死亡后每个剩余段都独立重新随机选择当时的存活敌方, 上一段的随机目标不会被锁定; 攻击者死亡、敌方全灭或段数耗尽时结束, 整组完成后仅按最后实际段判断一次反击. 合击和反击不展开为武器或空手多段. 合击每个成员都下发自己的`CombatEffect.damage`, 前序成员以`displayed_damage=0`和空`unit_delta_list`只表达命中、暴击及防守表现, 最后一名成员才携带合击总表现伤害和累计 HP 差量. 装备附带魔法当前仅作为展示元数据, 尚不执行施法; 投掷斧、投掷石的专用目标/消耗、状态抗性和其他特殊装备行为仍未完整接入.

### 回旋镖普通攻击

回旋镖按8.5服务端 `BATTLE_COM_BOOMERANG` 和 `BoomerangVsTbl` 扫描声明目标所在的一排. 声明目标只选择前排0至4或后排5至9; 即使声明目标在本动作执行前已经死亡, 仍保留其原排. 只有该排已经没有任何存活且未离场的敌方单位时, 才从全部存活敌方中随机选择一次回退目标, 并改扫回退目标所在排. 随机数继续使用当前每场战斗独立PCG, 不要求与旧服全局`rand()`算法或相同seed序列一致.

| 攻击方阵营 | 排内position偏移顺序 |
| --- | --- |
| Initiator, 原版side 0 | 4, 2, 0, 1, 3 |
| Defender, 原版side 1反向遍历 | 3, 1, 0, 2, 4 |

服务端按表中五个位置依次扫描, 空位、死亡和已离场单位只跳过; 每个有效位置完整执行一次独立物理攻击并产生一个有序`CombatActionStep`. 回旋镖不会使用配置`attacknum`限制实际命中目标数: 装备动作前仍按通用规则消费一次攻击次数随机, 但只要同行存在5个有效单位就会各攻击一次. 扫描途中击倒某个目标后继续后续位置, 不回头重复攻击, 也不把剩余次数集中补到单体.

每个目标先执行完整的闪避、暴击、Guard和普通伤害结算, 再按8.5的C `float`语义乘`gBattleDamageModyfy=0.3`并向零截断. 该缩放不补最低1点, 因而缩放前1至3点正伤害会得到0, 但仍保留原本的Normal或Critical命中表现; 缩放前已经是0伤害则保持Miss/Guard语义. 每个目标独立应用HP和死亡差量. 回旋镖属于投掷武器, 在合击资格随机前排除且攻守任一方装备回旋镖时不进入反击随机.

当前PVE尚无原版守护代挡、光镜守、陷阱、针刺及狐狸/猪变身的可达运行态. 后续接入这些系统时, 回旋镖必须继续按原版投掷分支跳过守护替换和普通反应降级链, 不能把当前不可达输入误写成已完成能力.

### 弓普通攻击

弓按8.5服务端 `BATTLE_TargetListSet` 的 `aBowW` 生成一次十站位候选表. 声明目标的阵营和站位只决定候选表, 不把全部箭锁在声明目标上. 当前协议每个阵营使用0至9站位, `column = targetPosition % 5`; 每场攻击只从当前战斗PCG抽取一次 `variant = RAND(0,1)` 等价分支, 随机算法本身不要求与旧服全局 `rand()` 逐位一致.

| column | variant 0同行顺序 | variant 1同行顺序 |
| --- | --- | --- |
| 0 | 0, 2, 1, 4, 3 | 0, 1, 2, 3, 4 |
| 1 | 1, 0, 3, 2, 4 | 1, 3, 0, 2, 4 |
| 2 | 2, 4, 0, 1, 3 | 2, 0, 4, 1, 3 |
| 3 | 3, 1, 0, 2, 4 | 3, 1, 0, 2, 4 |
| 4 | 4, 2, 0, 1, 3 | 4, 2, 0, 1, 3 |

同行顺序中的每个位置后立即插入另一排的同列位置, 最终十个站位各出现一次. 服务端从头扫描候选表: 空位、死亡或已离场单位只跳过, 不消耗 `attacknum`; 每次实际执行 `BATTLE_Attack` 等价结算才消耗一箭. 达到锁定的 `attacknum`、十站位扫描完毕、攻击者失效或任一阵营全灭时停止. 因为每个站位最多出现一次, 同一次弓动作不会再次攻击已经扫描过的位置; 某目标被箭击倒后会继续扫描后续站位, 不会把剩余箭补射到该目标.

每支箭独立执行闪避、暴击、Guard、最低伤害和HP变化, 并产生一个按结算顺序排列的 `CombatActionStep`. 弓暴击仍按普通暴击阈值判定并设置 `critical=true`, 但严格保留原版例外: 只使用普通 `BATTLE_DamageCalc` 等价伤害, 不追加非弓暴击的防御与等级增伤. 弓属于投掷武器分类, 在合击候选随机前即排除, 攻守任一方持弓也不会进入反击随机. 原版 `_FIXBUG_ATTACKBOW` 还会在道具/NPC/狐狸变身期间禁用远程武器; 当前PVE服务端尚无对应可达变身运行态, 后续接入该状态时必须在生成弓候选表前拒绝动作. 原版守护代挡运行态当前同样尚未接入; 接入时弓必须继续跳过守护者替换.

## 账号绑定与解绑

`OnlineBindAccount`:

1. 校验 aid、account、gatewayKey 和 accountSession.
2. 从 cache 读取并校验 `AccountRecord`.
3. 创建或绑定 Account actor.
4. 建立角色管理器并保存 gateway stream.

`OnlineUnbindAccount`:

1. 校验 gatewayKey 和 accountSession 与当前 actor 匹配.
2. 持久化仍在线角色的登出状态.
3. 清除各角色的 CombatRoom actor 指针, 并把外部脱离投递给对应房间; 房间移除这些参与者, 仍有其他参与者时继续战斗, 为空时无结算关闭.
4. 停止 Account actor.

cache account session 的结束由 gateway 负责.

## 排障

- `load game config failed`: 检查 `custom.gameConfigDir`、必需 YAML 文件和跨表引用.
- `invalid combat action`: 检查 battle ID、round、单位归属、技能是否存在、宠物是否持有技能及目标是否合法.
- `character mailbox is full`: 角色清理过期邮件后仍有 1000 封有效系统邮件, 新增邮件失败。
- 读取、已读或删除邮件返回 `NotFound`: 角色或邮件不存在, 或邮件已经过期; 客户端应移除本地对应邮件。
- online 返回业务错误后连接仍在: 这是预期行为, 客户端可修正请求后继续使用当前连接.
- 业务包无响应: 检查 gateway stream、online Account actor 和目标 aid.
- `DeadlineExceeded`: 检查 gateway 到 online 的 RPC 超时、online 日志和 actor 是否阻塞.

## 后续建议

- 为 `OnlineBindAccount` 和 `OnlineUnbindAccount` 补齐 actor 生命周期集成测试.
- 继续拆分较大的战斗结算文件, 但不要恢复已移除的旧技能兼容层.

## 捕获 8000004

角色通过现有技能请求提交敌方目标. 结算时目标已失效则沿用普通目标调整规则重选存活敌人. 只有 PVE 敌人组 `captured: true` 的普通宠物模板可被捕获, Boss 和显式禁止捕获的敌群不能捕获, 目标等级不得超过角色等级 5 级. 捕获不扣 MP, 不造成 HP 伤害, 不参加合击, 不触发反击或击杀收益; 行动前的既有毒伤仍正常结算.

概率沿用 8.5 `BATTLE_CaptureCheck` 的 float32 顺序:
`(10 - hp*hp/maxHp + sourceLevel/2 - targetLevel/2 + sourceDex/15 - targetDex/15 + captureBase + sourceLuck) * sourceCharm / 50`.
各差值先独立计算后相加, 阈值只封顶 99, 用一次 `RAND(1,100) < threshold` 判定, 不改为小于等于或添加最低成功率. 本版本尚无睡眠、临时捕获加成及特殊宠物道具门槛的运行态, 当前使用敌群捕获权限和已实现的属性.

敌人创建时额外冻结独立随机偏移后的 SavedBase, 它位于十点初始加点之前; Raw 四维沿用战斗中的实际个体. `common/pet.CaptureSnapshot` 同时冻结等级经验下限和出生七槽技能, 捕获时不重新随机、不继承 NPC AI 技能, 不受战斗期间模板更新影响. 品阶按四项独立偏移总和派生. 新档案沿用当前获得宠物的约定, 忠诚度 100, 携带状态为等待; 成长基线记录捕获等级及对应面板. 当前 PetRecord 不持久化即时 HP/MP.

CombatRoom 通过同步账号消息提交冻结数据. Account actor 再次校验角色仍属于该房间, 检查当前随身宠物容量与账号 UUID 游标, 生成完整候选档案并保存 cache; 失败恢复角色指针、账号记录和 UUID 游标. 保存成功后发送既有 `CharacterPetChangedNotify`, 房间才产生成功 Capture 和 UnitLeave(Captured). 容量不足返回 Capacity, 保存或账号状态异常返回 Persistence, 两者均保留敌人在场. 最后一只敌人被捕获可正常结束战斗.

