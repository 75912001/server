# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头. 客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.

## 配置文件

- `character.yaml`: 角色资源配置和独立的编辑器测试状态.
- `character.sprite.yaml`: 角色及角色骑宠动画帧、逐动作 FPS、攻击声音、命中表现、原版 Raw 参考配置和独立的编辑器测试状态.
- `ai.yaml`: 服务端战斗 AI 配置, 由敌人组显式引用, 保存技能 ID、相对权重和目标策略.
- `common.sprite.yaml`: 通用精灵资源配置; `atlas` 使用相对 `assets` 的无扩展名路径并且必须以 `common/` 开头, `value` 保存客户端逐帧消费的有序帧号, 8702-8710和347511-347513是地水火风的大中小属性图标, 242302是设置窗口角色随身物品位置的原版背景板, 暴击8723条目合并保存前14帧小星和后13帧大星的60Hz时间线; 9195和9196分别是设置窗口角色属性加点按钮的未按下和按下状态. 900000100/900000110/900000120/900000130分别是普通伤害、暴击伤害、HP恢复和MP恢复的数字精灵组, 每组`value`按0-9顺序保存十帧.
- `skill.yaml`: 角色和宠物共用的技能配置.
- `enemy.group.yaml`: 敌人编组、宠物模板、数量、等级和必填的战斗 AI 引用.
- `enemy.exp.yaml`: 敌人等级基础经验配置.
- `exp.yaml`: 角色和宠物等级经验配置.
- `information.yaml`: 从 STW1.13 `Mission.txt` 转换的 UTF-8 石器情报树和正文, 由 sa.desktop 只读展示.
- `item.yaml`: 普通道具和角色资产配置, 使用 `items.item.<id>` 分组; 非零 `sprite` 必须同时配置以 `item/` 开头的无扩展名 `atlas` 路径.
- `item.weapon.yaml`: 八类武器配置, 使用 `items.<weaponGroup>.<id>` 分组; 武器ID必须位于对应协议分组区间. 武器条目可保存原版名称、说明、价格、装备等级、职业限制、套装编号、攻击次数、能力随机范围、元素和异常抗性; `cost > 0` 表示可在全局商店购买, `cost = 0` 表示不可购买; 未配置的数值字段默认为0, 非法范围会导致启动失败.
- `task.yaml`: 运行任务、串行步骤、接取/开始/完成条件、提交扣除和奖励包引用.
- `reward.yaml`: 可复用奖励包, 当前仅支持普通道具和角色资产的`itemId + quantity`数组.
- `pet.yaml`: 宠物主体、成长、图鉴面板参考值、出生技能槽和编辑器测试状态配置.
- `pet.sprite.yaml`: 宠物八方向的 `attack/faint/hurt/defense/stand/walk` 六个动画帧、攻击表现和独立的编辑器测试状态配置.
- `scene/*.yaml`: 与客户端 `map_id` 一致的地图尺寸、阻挡、平铺遇敌规则、NPC功能选项和传送配置.
- `../docs/offset.yaml`: 全局帧视觉元数据总表, 格式为 `frame_id: [offset_x, offset_y, width, height]`.

## 敌人成员等级

`enemy.group.yaml enemies[]` 支持固定等级 `level` 和随机等级闭区间 `levelRange: [最小等级, 最大等级]`, 两个字段互斥, 等级必须处于协议范围 `[1,140]`. Boss 的每个成员必须且只能配置其中一个; 普通组成员可以都不配置, 此时使用组级 `levelRange` 或 `roleLevelOffset`. Boss 仍不允许配置组级等级范围或玩家等级偏移, 并按成员顺序固定出怪.

阿布洞窟敌群1000701的五名守护者全部使用成员级 `levelRange`: 4900294为 `[39,41]`, 4900295为 `[38,40]`, 4900296为 `[37,39]`, 4900297为 `[35,38]`, 4900298为 `[40,43]`. 这些区间来自8.0原版 `gmsv/data/enemy1.txt` 的294-298号敌人. 服务端每次创建敌人时独立抽取等级, 客户端只展示区间; 修改后使用客户端现有脚本 `./cp.config.from.server.sh enemy.group.yaml` 单向同步.

## 运行任务与奖励包

Godot `make_task` 的运行任务和奖励包页直接编辑本目录的`task.yaml`、`reward.yaml`. `../docs/任务/task.yaml`仍为原版调研资料, 不由服务端加载, 也不自动转换成运行任务. 编辑器保存前校验结构和跨表引用, 两文件事务提交并检测外部修改; 保存不会自动同步客户端或部署服务.

任务使用`tasks: [...]`, 每项包含`id/name/description/isMain/sort/acceptConditions/steps`. 多个任务可并行接取, 同一任务只接一次, 内部步骤按数组顺序串行推进, 步骤ID从1连续递增. 步骤包含`id/name/description/startConditions/completionConditions/completionMode/consumeItems/rewardId`.

条件数组全部使用AND语义. V1支持`characterLevel(level)`、`itemPossession(itemId, quantity)`、`taskCompleted(taskId)`、`battleVictory(enemyGroupId)`. 战斗胜利只用于automatic步骤的完成条件; 接取和开始条件不接受瞬时战斗事件. `completionMode`省略即`automatic`, `submit`由玩家主动提交. `itemPossession`只检查持有, 不扣除; `consumeItems`只允许配置在submit步骤中, 提交成功才扣除.

奖励包使用`rewards: [...]`, 每项包含`id/name/items`. 道具数量必须大于0, 同一列表不得重复itemId. 任务步骤以非0`rewardId`引用奖励包; `rewardId: 0`必须显式填写, 表示无奖励, 步骤完成时同时记为已领取. 已领取状态不因后来扩展奖励包或补配奖励而重置.

V1仅有非主线任务60“阿布洞窟”, 无额外接取条件, 一个挑战步骤, 战胜敌群1000701后完成且无道具奖励. 完成记录作为以后购买阿布水的资格依据, 商店购买入口的资格检查尚待接入. 任务挑战BGM配置在`task.yaml`的`tasks[].steps[].challenge.battleBgmIndex`, 阿布洞窟使用索引6; 该字段仅供客户端选择音乐, 不放在`reward.yaml`或`enemy.group.yaml`. 场景NPC挑战仍使用客户端NPC表现配置, 服务端不选择或播放BGM.

步骤通过可选`challenge`配置挑战入口, 服务端只读取其中的`enemyGroupId`; `npcPetIds`和`battleBgmIndex`由客户端消费. 已接任务中已经开始的挑战步骤可重复挑战, 已完成后也保留入口, 无需新增重复挑战配置. 重打不重新接取任务, 不重置完成记录或步骤奖励领取状态.

## 武器目录与导出

`../docs/item.weapon.catalog.yaml` 是八类武器的唯一编辑来源. 它使用稳定 `key` 保存4564条完整武器记录: 迁移记录使用 `legacy:<原版ID>`, Godot编辑器新增记录使用单调递增的 `custom:<序号>`. `status: draft` 表示只保存在目录中, 可以暂缺现代ID、名称或帧资源; `status: published` 表示必须具备完整运行字段并通过ID区间、重复ID、数值范围和客户端图集帧校验.

目录采用领域字段, `modernId`、`frameId`、`secretName`、`effectString`、`profession`、`elementType`和各项`*Min/*Max`在导出时映射到`item.weapon.yaml`的既有服务端字段. `idTier`只用于编辑器整理, `originalId`只保留原版追溯关系. 原版`hirt`和`neguard`分别保存为`legacyHitRight`和`legacyNeglectGuard`; 当前运行配置尚未支持这两个字段, 任一值非0都会阻止发布, 不会被静默丢弃或修补. 图集路径按`weaponType`固定派生, 不在目录中重复编辑.

sa.desktop 的 Godot `make_weapon` 主屏只保存目录, 不在发布按钮或保存目录时改写本文件. 用户必须单独点击“导出item.weapon.yaml”, 且目录存在未保存修改时导出会被拒绝. 导出先完成全部published条目和8类非空校验, 再把候选写入暂存文件、重新解析、检查目录与目标文件外部指纹, 最后事务替换本文件; 草稿永不进入运行配置. 导出不会运行Go测试, 也不会自动同步`sa.desktop/config/item.weapon.yaml`副本.

无需打开编辑器时, 可在`sa.desktop`目录使用同一套GDScript核心校验或导出:

```bash
Godot_v4.6.3-stable_win64_console.exe --headless --path . --script res://addons/make_weapon/headless/weapon_catalog_cli.gd -- --check
Godot_v4.6.3-stable_win64_console.exe --headless --path . --script res://addons/make_weapon/headless/weapon_catalog_cli.gd -- --export
```

显式同步客户端副本仍使用`./cp.config.from.server.sh item.weapon.yaml`, 不属于武器编辑器的保存或导出事务.

## 石器情报配置

`information.yaml` 保留 STW1.13 `Mission.txt` 的 489 条 `PATH/DATA` 记录. `information.entries[].path` 按原文件的 `->` 分隔结果保存从根节点到当前节点的完整层级, `content` 使用带 `\n` 转义的 YAML 双引号字符串保存右侧正文, 避免正文自身缩进被解释为 YAML 结构. 同级节点允许重名, 原始记录顺序决定客户端树节点顺序.

该配置属于 server 统一维护、sa.desktop 单向同步的客户端只读资料, online 服务不加载也不执行业务校验. 客户端通过 `ConfigInformation` 在 `load()` 阶段校验单表结构, 在 `assemble()` 阶段处理子记录先于父记录出现的原始顺序并组装主题树.

## 角色动画元数据生成

`../tool/character_sprite_metadata.py` 从原版 `spr_115.bin` 和 `spradrn_115.bin` 审计 `character.sprite.yaml` 的104个方向动作, 并生成13动作FPS、当前攻击声音、Throw投射物释放帧、Throw动作声音及原版Raw参考数据. 默认只读审计, 明确传入 `--write` 才会并发校验后原子写入:

```bash
python tool/character_sprite_metadata.py \
  --spr D:/csa_8.0/data/spr_115.bin \
  --spr-address D:/csa_8.0/data/spradrn_115.bin \
  --config config/character.sprite.yaml \
  --write
```

`throwReleaseFrameNumber`记录原版Throw动作中10000-10099投射物事件映射后的1-based帧位置, 没有该事件时为0; `throwActionSoundFrameNumberList`和`throwActionSoundIdList`记录生效Throw动作的逐帧声音事件. 默认值来自原版事件, 新版另对吉米四种颜色实战复用的unarmed sprite 0/5/10/15在原版第5项释放前的第4项补充声音ID 4. 生成器要求同一sprite八方向映射完全一致, 不允许客户端用固定帧或“倒数第几帧”猜测释放时点.

4个事件字段以 `Raw` 结尾, 记录原版攻击事件的1-based帧位置和声音ID. 当前方向动作与原版帧序列不同时, 生成器还会在生效动作后写入 `<action>Raw`, 例如 `attackRaw`. 所有Raw字段只供后期对照, 客户端只校验而不创建播放缓存; 修改生效动作前必须同时确认当前图集帧、`.tpsheet`和offset完整, 不能直接用Raw覆盖.

## 角色配置编辑器

`character.yaml` 的 `character[]` 和 `character.sprite.yaml` 的 `sprite[]` 都可保存可选编辑器元数据 `testStatus`. 缺省或0表示未测试, 1表示通过, 2表示未通过; 状态0不落盘. 角色状态只表示主体字段已验证, sprite状态表示整套sprite已在全部角色本体、武器和骑宠引用上下文中人工验证. 该字段不参与 server 或客户端运行时业务.

sa.desktop 的 Godot `make_character` 编辑器直接把本目录的 `character.yaml` 和 `character.sprite.yaml` 作为权威源. 它只编辑、查阅和测试已有条目, 不新增、删除或重排角色、sprite及骑宠行. 角色 ID、名称、sprite ID和所有 `*Raw` 参考字段只读; 骑宠只能选择当前实际存在的资源, sprite引用只能选择目标图集中包含全部生效帧的兼容项.

显式保存时先校验完整双表、资源、跨表引用、8方向x13动作帧、FPS、声音及事件边界, 再以双文件事务替换原文件. 外部修改会阻止覆盖, 单文件提交失败会回滚. 编辑器不会自动同步 `sa.desktop/config` 运行时副本.

online 启动会加载 `character.yaml`、`skill.yaml`、`ai.yaml`、`enemy.group.yaml`、`enemy.exp.yaml`、`exp.yaml`、`item.yaml`、`item.weapon.yaml`、`reward.yaml`、`task.yaml`、`pet.yaml` 和 `scene/*.yaml`. 任一必需文件缺失、字段非法或跨表引用无效时, 服务必须直接启动失败.

## 统一技能配置

`skill.yaml` 是 server 唯一的技能定义来源, 不再存在独立的宠物技能配置文件.

`8000001-8000007` 依次为攻击、防御、逃跑、捕获、换宠、使用道具和更换装备, 在技能目录中统一归入 `basic_action` 基础动作, 不按各自处理器名称拆分功能分类. 原版宠物技能“待机”归入 `other`, “修复”归入 `craft_life`, 两者都不是基础动作.

`../docs/skill.catalog.yaml` 是技能调研与开发排期目录, 不是运行配置. 它聚合基础战斗动作、宠物/NPC技能、角色精灵与魔法、角色职业技能四类可再生证据, 可选保存编辑器新增的 `custom:<现代ID>` 技能, 并保留 `../docs/pet.skill.runtime.yaml` 中218条9000000段历史现代配置的迁移映射. sa.desktop 的 Godot `make_skill` 编辑器同时读取该目录和本文件: 左侧把相同现代ID的来源记录合并为唯一技能实体, 目录校正写入来源记录的 `curation` 覆盖层; “导入开发”把原版技能绑定到未占用现代ID并向本文件追加最小运行骨架, “新增技能”则同时创建自定义目录记录和运行骨架. 两种入口的状态都从“未实现”开始. 保存使用双文件暂存、重新解析、外部修改检测和失败回滚, 不会自动同步 `sa.desktop/config/skill.yaml` 客户端副本.

全量目录由 `tool/skill_catalog.py` 生成. 默认只检查来源与目录语义是否一致, 明确传入 `--write` 才原子写入; 已有人工 `curation` 按稳定 `key` 保留, 自定义技能完整记录也会保留, ID冲突时直接失败:

```bash
python tool/skill_catalog.py --write
```

每个 `skill[]` 条目必须配置:

- `id`: 技能资源 ID, 必须位于协议技能区间.
- `name`: 非空技能名称.
- `description`: 非空技能说明.
- `cost`: 可选的非负整数. 字段存在表示该技能允许宠物在设置页学习, 数值是一次学习或替换消耗的石币; `cost: 0` 仍表示明确开放且免费. 字段缺失表示不可学习.
- `continuationAttack.segmentCount`: 可选. 配置后表示连续攻击, 段数范围为 1-10.
- `mightyAttack`: 可选. 配置后表示一击必杀, 与 `continuationAttack` 互斥. 必须同时提供整数 `damageMultiplier` 和 `targetDodgeBonus`; 前者为最终伤害倍率, 范围1-655, 后者为目标基础闪避的百分点加值, 范围0-32767. 范围对应原版COM3低16位百分比和高16位有符号加值, 不接受缺项、空值或小数截断.
- `poisonAttack`: 可选. 普通中毒物理攻击, 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0). 时长是目标实际扣毒血的行动次数, 攻击修正是施放者攻击属性的百分比加值. `continuationAttack`, `mightyAttack`, `poisonAttack`, `chargeAttack`, `showMercy` 五者互斥.
- `chargeAttack`: 可选. 突击蓄力攻击, 必须同时提供整数 `chargeRounds` (1-10) 和 `attackPercentModifier` (0-32767). 前者为完整等待行动次数, 后者为释放时基础攻击力的百分比加值. 不接受空值, 缺项, 字符串数字或小数.

- `showMercy`: 可选. 手下留情只接受空对象 `{}`, 不接受空值或参数. 先完成一次普通物理判定, 本次伤害若致死则限制为目标当前HP减1, 目标1HP时允许0伤害. 不形成持续保命状态.

当前开放的 20 个宠物技能价格来自原版8.0 `gmsv/data/petskill2.txt` 的价格列, 并由8.5服务端 `npc_petskillshop.c` 读取 `PETSKILL_COST` 的逻辑交叉确认: 待机500, 攻击/防御各1000, 破除防御1500, 二至十段攻击依次为2000, 5000, 15000, 25000, 225000, 625000, 625000, 1625000, 2005000; 一击必杀2500, 一击必杀改和改2各25000, 猛毒攻击4000, 突击4000, 双重突击8000, 手下留情10000. 本项目设置页按该基础价直接扣款, 不应用原版NPC可选的 `skill_rate` 倍率.

当前 online 执行能力如下:

| 技能 ID | 名称 | 角色 | 玩家宠物 | 敌方 NPC |
| --- | --- | --- | --- | --- |
| 8000001 | 攻击 | 支持 | 支持 | 支持 |
| 8000002 | 防御 | 支持 | 支持 | 支持 |
| 8000003 | 逃跑 | 支持 | 不支持 | 支持 |
| 8000004 | 捕获 | 支持, 校验敌方目标 | 不支持 | 启动拒绝 |
| 8000005 | 换宠 | 未开放 | 不支持 | 启动拒绝 |
| 8000006 | 使用道具 | 未开放 | 不支持 | 启动拒绝 |
| 8000007 | 更换装备 | 未开放 | 不支持 | 启动拒绝 |
| 8100000 | 待机 | 不支持 | 支持 | 支持 |
| 8100003 | 破除防御 | 不支持 | 支持 | 支持 |
| 8100010-8100018 | 连续攻击 | 不支持 | 按 `segmentCount` 支持 | 按 `segmentCount` 支持 |
| 8100030-8100031 | 突击 | 不支持 | 按 `chargeAttack` 支持 | 按 `chargeAttack` 支持 |
| 8100040-8100042 | 一击必杀 | 不支持 | 按 `mightyAttack` 支持 | 按 `mightyAttack` 支持 |
| 8100061 | 猛毒攻击 | 不支持 | 按 `poisonAttack` 支持 | 按 `poisonAttack` 支持 |
| 8100626 | 手下留情 | 不支持 | 按 `showMercy` 支持 | 按 `showMercy` 支持 |

原版626手下留情映射 `8100626`, 学习价10000石币, 使用 `showMercy: {}`. 名称和描述保留 `petskill2.txt:182` 原文. 技能不参加合击, 使用者整回合不能反击, 受击目标仍按普通规则反击. 限伤发生在普通命中, 暴击, Guard和最低伤害之后, 扣血和击飞判断之前; 限伤产生的0伤害保留原命中类型. 编辑器目录归入 `physical_attack` 并标记已实现, 不自动分配给宠物出生模板或敌人AI. 捕获及尚未接入的特殊伤害反应保持各自开发范围.

原版宠物30/31分别映射 `8100030/8100031`, 参数为1次蓄力/+90%攻击力和2次蓄力/+110%攻击力. 30在第二次行动, 31在第三次行动释放单次普通物理攻击. 修正作用于释放时的基础攻击力, 不直接乘最终伤害. 技能名称和 `description` 保留原文, 精确机制只写入配置注释和内部文档. 技能目录标记为已实现, 不自动给出生模板或敌方AI添加技能. 后续回合的技能和目标由服务端续用, 客户端跳过宠物选择直到释放完成.

原版宠物40、41、42分别映射 `8100040`、`8100041`、`8100042`, 参数为2倍/+30、3倍/+40、4倍/+50. 倍率作用于暴击、防御姿态减伤和最低伤害判定后的单段最终伤害; 闪避加值在基础75%封顶之前计入, 后置独立装备闪避继续生效. 它们不保证暴击或秒杀, 不参加合击, 开始主动行动后才取得普通反击资格, 反击不继承倍率和闪避加值.

技能目录将这三条记录归入 `physical_attack`, 标记为已实现. 原版宠物39的文案写2倍而参数为3倍/+500, 本地资料未确认其用途, 因此从编辑器目录及 `tool/skill_catalog.py` 的宠物生成入口排除; 原始 `docs/pet.skill.yaml` 保留, 其他来源系统的同号技能不受影响.

原版61映射 `8100061`, 使用实际启动日志确认加载的 `petskill2.txt` 第29行: `毒 turn 5 攻%-30`. 该行说明仍写减攻50%, 旧 `petskill.txt` 也确实配置-50%, 当前运行值遵循可执行参数-30%. 附毒计数直接取durationActions=5, 表示剩余毒伤次数. 目标正常行动开始结算一次毒伤并减1, 第5次扣血同时解除中毒和标记; 解除前不叠加或刷新, 解除后允许再次附毒. 毒伤按目标基础四维计算, 最多扣至1HP. 不给宠物出生模板或敌人AI自动添加该技能; 玩家学习后或敌人AI明确引用后才能使用.

“未开放”表示配置合法, 但 online 收到玩家动作后返回业务错误. “启动拒绝”表示该技能出现在敌人引用的 AI 技能列表时, Online 在注册 etcd 和 gRPC 前直接启动失败. 玩家宠物必须在自己的实例技能槽中持有技能, 敌方 NPC 必须在本场冻结的 AI 技能列表中持有技能.

## 宠物技能槽

`pet.yaml skill` 固定 7 槽, `0` 表示空槽, 非 0 ID 必须存在于 `skill.yaml`. 它只作为新宠物实例的出生技能模板; 创建后以 `PetRecord.skill_id_list` 为权威, 学习、替换和遗忘不回写模板. 捕获创建玩家宠物时也使用出生技能, 不继承敌人的 AI 技能.

`enemy.group.yaml` 的每个 `enemies[]` 必须配置 `battleAI`, 直接引用 `ai.yaml`. 敌人战斗技能完全由 AI 定义, 不回退 `pet.yaml skill`. 旧 `pet.yaml battleAI`、`enemies[].skill` 和 AI 分离权重字段都会使配置加载失败.

多数宠物当前配置为:

```yaml
skill: [8000001,8000002,0,0,0,0,0]
```

90001和90010至90130的14个练级组使用 AI 1, 攻击、防御、逃跑权重为 `10:1:1`. 其余普通敌人使用 AI 18, 攻击、防御权重为 `10:1`. 查罕·乌尔夫和查罕·吉鲁使用 AI 19, 仅配置攻击和防御, 权重为 `10:1`, 保持原有选择行为. 敌人 AI 只保存在服务端战斗运行态, 不写入 `CombatUnit.skill_id_list`.

## 宠物主体配置

`pet.yaml` 使用 `pet.<family>: [...]` 按系别组织. server 加载后按宠物 ID 建立全局索引, 运行时不保留系别层级.

server 消费的主要字段:

- `id`, `name`, `rarity`.
- `elemental`: 地、水、火、风总和必须为 10, 只能是单元素或两个相邻元素.
- `attribute`: 异常抗性、暴击、反击、捕获和服务端战斗特性.
- `growth`: 初始和升级成长参数.
- `panelReference`: 客户端图鉴直接展示的1级和140级普通品阶平均值、神话品阶平均值及总成长上下限, 只保存服务端预计算结果.
- `skill`: 新宠物出生时的固定7槽技能.
宠物模板不保存战斗 AI 引用; 同一种宠物可由不同敌人条目选择不同 AI.

此外, `pet.yaml` 和 `pet.sprite.yaml` 的条目都可保存可选的编辑器元数据 `testStatus`. 缺省或0表示未测试, 1表示通过, 2表示未通过; 状态0不落盘. 该字段不参与 server 或客户端运行时业务, 宠物主体和 sprite 的测试状态相互独立.

`ai.yaml` 使用 `ai: [...]` 保存共享配置, `skills[]` 将技能 `id` 与相对 `weight` 放在同一条记录中. 攻击、防御、逃跑及特殊技能使用同一种结构, 不要求凑满7槽. 技能 ID 不得重复, 权重范围为 `[1,2147483647]`, 不使用的技能直接移除, 总权重必须处于 `[1,2147483647]`. `targetScope`、`targetSelection` 和可选 `targetRandomRollMax` 保留现有目标选择语义.

`enemy.group.yaml enemies[].weight` 控制出怪时选择哪种敌人, `ai.yaml skills[].weight` 控制战斗时选择哪个技能. 要给同一种宠物设置不同的技能概率, 定义不同 AI 并在对应敌人条目中引用.

自动遇敌从敌人组分别取得宠物模板和 AI. `load()` 校验单表结构, `check()` 校验 AI 到技能、敌人组到宠物及 AI 的跨表引用, `assemble()` 在敌人条目挂载只读 AI. Online 在注册服务前验证每个敌人 AI 技能的 NPC 执行能力, 建房时深拷贝技能、权重及目标策略; 回合中不再查询宠物模板的技能或 AI.

`growth` 保存非负的原版模板值, 但应用品阶偏移和原版公式后, 宠物实例的 `SavedBase*`、`Raw*` 以及成长基线中的防御、敏捷均使用 `int32`, 允许为0或负数. 配置和档案校验不得再要求这些中间值大于0; 只有派生后的最大生命和攻击必须大于0, 才能构成有效存活战斗单位.

### 图鉴面板参考值核验

`panelReference` 的完整计算规则记录在 `pet.yaml` 文件头. 客户端运行时只解析并显示这些预计算结果, 不保存成长基础四维, 也不实现参考值算法. Godot `make_pet` 编辑器另有一份仅供编辑时实时预览和写回的 GDScript 实现; 它不进入客户端运行时, 并通过全量宠物测试逐项对照 server 权威结果.

online 启动加载配置时, server 使用权威宠物生成、升级和面板换算规则重新计算所有宠物的 `panelReference`. 核验会集中收集全部不一致项, 每项日志包含宠物 ID、名称、配置实际值以及可直接复制回 `pet.yaml` 的正确 YAML. 存在任一不一致时配置加载失败, 服务不得启动.

`make_pet` 编辑器直接编辑本目录的 `pet.yaml` 和 `pet.sprite.yaml`, 并读取 `skill.yaml` 供选择出生技能. 它不读取 AI 配置, 不新增、删除或重排宠物和sprite. 保存前校验完整双表、技能与资源引用, 然后通过双文件事务提交; 外部修改阻止覆盖, 单文件失败回滚. 编辑器不自动同步 `sa.desktop/config`.

## 跨表关系

主要引用关系:

```text
scene/*.yaml
  -> enemy.group.yaml
       -> pet.yaml (出生技能 -> skill.yaml)
       -> ai.yaml (战斗技能及权重 -> skill.yaml)
```

- `scene/*.yaml` 不设置格式版本字段, 地图 ID 与客户端 `map_id` 一致; `collision.blockedRows` 保存服务端阻挡, `encounter.enabled` 和 `encounter.enemyGroups` 定义全地图遇敌开关与敌人组权重, `npcs` 保存NPC实体及其独立功能选项, `warps` 保存传送起点与目标. 当前目录包含80000、80001、80010、80020、80030、80040这6张测试地图, 以及从90001开始的14张练级地图: 90001和90010至90130按10递增. 正常角色地图进入允许测试范围`[80000,89999]`和练级范围`[90000,99999]`. 可进入地图必须启用遇敌并配置有效敌人组.
- `encounter.enabled` 必须显式配置; 启用遇敌时 `encounter.enemyGroups` 不能为空且总权重必须大于0.
- `enemy.group.yaml enemies[].id` 引用宠物模板, `enemies[].battleAI` 必填且引用 `ai.yaml`.
- `ai.yaml skills[].id` 引用 `skill.yaml`, 权重与技能 ID 在同一条记录中.
- `pet.yaml` 的非0出生技能槽引用 `skill.yaml`, 不参与敌人 AI 的技能选择.

server 只校验 YAML 结构、服务端消费字段、枚举、数值范围和跨表引用. 名称、描述、sprite、PNG、`.tpsheet` 和动画帧完整性由 sa.desktop 的资源流程校验.

## 捕获资源与配置

`8000004` 是角色基础动作, 不新增学习费用或宠物技能槽配置. 捕获权限由 `enemy.group.yaml captured` 决定, Boss 固定禁止; 基础捕获值使用 `pet.yaml attribute.get`. CaptureSnapshot 在开战时冻结出生技能和实际个体, 捕获成功不会把敌群 AI 带入玩家宠物档案.

`common.sprite.yaml` 注册状态图集中的 `8820` Capture! 和 `8814` Get!, 失败复用已有 `8813` Fail.... 不增加捕获按钮资源.

`pet.sprite.yaml` 可成对配置 `walkActionSoundFrameNumberList` 和 `walkActionSoundIdList`, 省略时表示 Walk 没有声音. 两个数组等长, 帧号从 1 开始且严格递增, 不得超出任一方向 Walk 的长度. 当前原版 ID 支持 76(`sae_26.wav`) 和 79(`sae_29.wav`). sprite 354/584 使用帧 1、4 的 76, 1152/1153/1154/1155 使用帧 2 的 79, 八方向共享. 客户端和宠物编辑器均按 Walk 帧事件消费, 与攻击的动作声音、命中声音分别保存.

