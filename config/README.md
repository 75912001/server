# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头. 客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.

## 配置文件

- `character.yaml`: 角色资源配置和独立的编辑器测试状态.
- `character.sprite.yaml`: 角色及角色骑宠动画帧、逐动作 FPS、攻击声音、命中表现、原版 Raw 参考配置和独立的编辑器测试状态.
- `ai.yaml`: 服务端战斗 AI 配置, 由敌人组显式引用, 保存技能 ID、相对权重和目标策略.
- `common.sprite.yaml`: 通用精灵资源配置; `atlas` 使用相对 `assets` 的无扩展名路径并且必须以 `common/` 开头, `value` 保存客户端逐帧消费的有序帧号, 8702-8710和347511-347513是地水火风的大中小属性图标, 242302是设置窗口角色随身物品位置的原版背景板, 暴击8723条目合并保存前14帧小星和后13帧大星的60Hz时间线; 9195和9196分别是设置窗口角色属性加点按钮的未按下和按下状态. 900000100/900000110/900000120/900000130分别是普通伤害、暴击伤害、HP恢复和MP恢复的数字精灵组, 每组`value`按0-9顺序保存十帧.
- `技能.yaml`: 角色和宠物共用的技能配置.
- `enemy.group.yaml`: 敌人编组、宠物模板、数量、等级、普通掉落和必填的战斗 AI 引用.
- `enemy.exp.yaml`: 敌人等级基础经验配置.
- `exp.yaml`: 角色和宠物等级经验配置.
- `information.yaml`: 从 STW1.13 `Mission.txt` 转换的 UTF-8 石器情报树和正文, 由 sa.desktop 只读展示.
- `item.yaml`: 道具编辑器从`../../sa.desktop/config.raw/item.yaml`确定性生成的普通道具运行配置, 使用`items.item.<id>`分组; 当前包含11条自定义或任务普通道具和297条资源已核对的合成相关普通道具. 非零`sprite`必须同时配置以`item/`开头的无扩展名`atlas`路径, 不允许反向编辑本文件.
- `道具.素材.yaml`: 合成素材运行配置, 使用 `items.material.<id>` 分组, 当前发布34个素材系的120条素材.
- `item.currency.yaml`: 货币配置, 使用 `items.currency.<id>` 分组, 与普通道具、素材和装备独立维护.
- `item.synthesis.yaml`: 已暂停的旧原子合成数据, 保留合成规则、50种原子定义、20档材料等级和道具资格. Online运行时不读取或校验该文件.
- `道具.天工司.yaml`: 宠物技能“加工”的基础武器制造、附技能及附元素配方. Online分别按`recipes.<weaponId>.materials`、`enchantments.<skillId>.materials`和`attributes.<ground|water|fire|wind>.materials`精确匹配素材. 基础配方允许多个产物使用相同素材签名, 命中后在候选产物间等概率随机; 服务启动时仍校验武器、素材和技能引用, 拒绝未知元素键、超过4种元素以及同一装备下同类附技能或附元素素材签名重复. 附加技能只要求技能存在, 不使用`usableBy`限制写入装备档案; 附元素命中时把对应地、水、火、风元素以固定值20写入装备实例.
- `item.equipment.<类型>.yaml`: 铠甲, 头盔, 盾牌, 手套, 腰带, 鞋子和首饰共7个运行输出, 分别使用 `items.equipmentChest`, `equipmentHelmet`, `equipmentShield`, `equipmentGloves`, `equipmentBelt`, `equipmentBoots` 和 `equipmentAccessory`. 首饰的 `accessory_type` 必须匹配协议类型及 ID 子区间. 七类文件允许空发布分组, 不能配置 use; 服务端展示字段按编辑器 C/S 归属独立裁剪, 已提供的 sprite 必须为正数, 已提供的 atlas 必须合法.
- `道具.武器.<类型>.yaml`: 8个武器配置文件, 每个文件只保存对应的 `items.<weaponGroup>.<id>` 分组; 武器ID必须位于对应协议分组区间. 武器条目保存名称、说明、装备等级、职业限制、套装编号、攻击次数、能力随机范围、元素和异常抗性; 是否上架及售价统一由 `商店.yaml` 决定.
- `商店.yaml`: 服务端开放商品配置, 使用无 `type` 的扁平 `items`. 商品 `id` 直接使用道具或技能资源 ID, 服务端按 ID 范围区分道具与技能; 道具商品配置 `item_count`, `costs[]` 是购买时同时消耗的资源列表, 显式空数组表示免费. 如后续配置敬请期待商品, 仅导出到客户端; 当前商店配置不包含此类条目.
- `task.yaml`: 运行任务、串行步骤、循环规则、NPC/对话/地图入口、接取/开始/完成条件、提交扣除和奖励包引用.
- `reward.yaml`: 可复用奖励包, `items`支持普通道具、角色资产和装备实例, `pets`支持指定等级与随机品质的宠物实例.
- `pet.yaml`: 宠物主体、成长、图鉴面板参考值、出生技能槽和编辑器测试状态配置.
- `pet.sprite.yaml`: 宠物八方向的 `attack/faint/hurt/defense/stand/walk` 六个动画帧、攻击表现和独立的编辑器测试状态配置.
- `scene/*.yaml`: 与客户端 `map_id` 一致的地图尺寸、阻挡、平铺遇敌规则、NPC功能选项和传送配置.
- `../docs/offset.yaml`: 全局帧视觉元数据总表, 格式为 `frame_id: [offset_x, offset_y, width, height]`.

## 敌人成员等级

`enemy.group.yaml enemies[]` 支持固定等级 `level` 和随机等级闭区间 `levelRange: [最小等级, 最大等级]`, 两个字段互斥, 等级必须处于协议范围 `[1,140]`. Boss 的每个成员必须且只能配置其中一个; 普通组成员可以都不配置, 此时使用组级 `levelRange` 或 `roleLevelOffset`. Boss 仍不允许配置组级等级范围或玩家等级偏移, 并按成员顺序固定出怪.

`enemy.group.yaml enemies[].normalDrops`是可选普通掉落槽列表, 每名敌人最多10项. 每项必须提供正式`itemId`和`[1,10000]`万分比`probability`; 相同道具允许重复配置为多个独立槽位. 服务端创建每只敌人实例时按顺序独立判定, 未配置掉落不消费对应随机数.

阿布洞窟敌群70000的五名守护者全部使用成员级 `levelRange`: 4900294为 `[39,41]`, 4900295为 `[38,40]`, 4900296为 `[37,39]`, 4900297为 `[35,38]`, 4900298为 `[40,43]`. 这些区间来自8.0原版 `gmsv/data/enemy1.txt` 的294-298号敌人. 服务端每次创建敌人时独立抽取等级, 客户端只展示区间.

Godot `make_enemy`以客户端项目的`config.raw/enemy.group.yaml`作为敌群唯一编辑源. 顶部`发布/草稿`保存目录整体状态; `保存RAW`只更新编辑源, `同步C/S`校验敌群结构以及宠物、AI和普通掉落道具引用后, 事务写入本文件及客户端`config/enemy.group.yaml`. 任一目标提交失败会回滚全部已提交文件, 外部修改会阻止覆盖. 本文件为生成结果, 不手工编辑, 也不再通过`cp.config.from.server.sh`同步.

## 运行任务与奖励包

Godot `make_task`以客户端项目的`config.raw/task.yaml`作为任务和奖励包唯一编辑源. `保存RAW`只更新编辑源, `同步C/S`校验结构和跨表引用后, 将`task.yaml`、`reward.yaml`事务写入本目录及客户端`config/`; 任一目标提交失败会回滚全部已提交文件. 本目录两份文件为生成结果, 不手工编辑, 也不再通过`cp.config.from.server.sh`同步. `../docs/任务/task.yaml`仍为原版调研资料, 不由服务端加载或自动转换.

任务使用`tasks: [...]`, 每项包含`id/name/description/isMain/repeatable/sort/acceptConditions/steps`, 并可保存客户端展示用的`npc/dialogue`. 多个任务可并行接取, 内部步骤按数组顺序串行推进, 步骤ID从1连续递增. 步骤包含`id/name/description/startConditions/completionConditions/completionMode/consumeItems/consumePets/dialogue/rewardId`, 可选`navigation: {mapId, label}`配置该步骤的客户端地图入口, 也可配置挑战入口.

条件数组全部使用AND语义. 支持`characterLevel(level)`、`itemPossession(itemId, quantity)`、`petPossession(petId, level, quantity)`、`taskCompleted(taskId)`、`taskRewardsClaimed(taskId)`和`battleVictory(enemyGroupId)`. `itemPossession`对装备按实例数量检查, `petPossession`要求宠物ID和实际等级同时匹配; 两者都只检查持有. 战斗胜利只用于automatic步骤的完成条件; 接取和开始条件不接受瞬时战斗事件. `completionMode`省略即`automatic`, `submit`由玩家主动提交. `consumeItems`和`consumePets`只允许配置在submit步骤中, 提交成功后才扣除符合条件的实例.

奖励包使用`rewards: [...]`, 每项包含`id/name/items/pets`. `items`可发普通道具、角色资产或独立装备实例; `pets`使用`petId/level/grade/quantity`, 当前`grade`只接受`random`. 数量必须大于0, 同类列表不得重复ID. 任务步骤以非0`rewardId`引用奖励包; `rewardId: 0`必须显式填写, 表示无奖励, 步骤完成时同时记为已领取. 已领取状态不因后来扩展奖励包或补配奖励而重置.

任务60“阿布洞窟”保留单步骤挑战: 战胜敌群70000后完成且无额外奖励. 任务61`[任务][卡坦的愿望][1]`在地图70001分两批交付4只25级宠物, 第一批奖励火难的戒指, 第二批交回戒指后奖励1级随机品质修宝. 任务62`[任务][卡坦的愿望][2]`要求任务61的全部奖励已经领取, 在地图70002按相同两批流程奖励1级随机品质朵拉比斯; `repeatable: true`使最后一步领奖后重置记录并立即开始下一轮.

任务挑战BGM配置在`task.yaml`的`tasks[].steps[].challenge.battleBgmIndex`, 阿布洞窟使用索引6; 该字段仅供客户端选择音乐, 不放在`reward.yaml`或`enemy.group.yaml`. 场景NPC挑战仍使用客户端NPC表现配置, 服务端不选择或播放BGM.

步骤通过可选`challenge`配置挑战入口, 服务端只读取其中的`enemyGroupId`; `npcs`中的宠物显示名、动作、方向和`battleBgmIndex`由客户端消费. 每个挑战NPC必须对应当前敌群成员, 但不会改变实际参战成员. 任务级`npc.visual`可配置静态帧、宠物动作或带武器的角色动作, 同样只属于客户端表现. 已接任务中已经开始的挑战步骤可重复挑战, 已完成后也保留入口, 无需新增重复挑战配置.

## 武器目录与导出

`../../sa.desktop/config.raw/道具.武器.<类型>.yaml` 是八类武器的8个编辑来源, 每个文件只保存一种武器. 8个文件合计保存5137条完整武器记录. `status: draft` 表示只保存在源文件中, 可以暂缺现代ID、名称或帧资源; `status: published` 表示必须具备完整运行字段并通过ID区间、重复ID、数值范围和客户端图集帧校验.

目录采用领域字段, `modernId`、`frameId`、`effectString`、`profession`、`elementType`和各项`*Min/*Max`在导出时映射到8个`道具.武器.<类型>.yaml`的既有服务端字段. 每个源文件独立保存字段导出归属, 编辑器字段名称后的下拉框作用于当前类型文件的全部记录. `idTier`固定导出到C/S, 供客户端显示武器等级及服务端判定配方档位; `originalId`只保留原版追溯关系. 原版`hirt`和`neguard`分别保存为`legacyHitRight`和`legacyNeglectGuard`; 当前运行配置尚未支持这两个字段, 任一值非0都会阻止发布, 不会被静默丢弃或修补. 图集路径按`weaponType`固定派生, 不在目录中重复编辑.

sa.desktop 的 Godot `make_weapon` 主屏一次保存8个源目录文件, 不在发布按钮或保存目录时改写运行配置. 用户必须单独点击“导出双版本”, 且目录存在未保存修改时导出会被拒绝. 导出先完成全部published条目和8类非空校验, 再把16个候选写入暂存文件、重新解析、检查目录与目标文件外部指纹, 最后事务替换服务端和客户端各8个文件; 草稿永不进入运行配置. 导出不会运行Go测试.

无需打开编辑器时, 可在`sa.desktop`目录使用同一套GDScript核心校验或导出:

```bash
Godot_v4.6.3-stable_win64_console.exe --headless --path . --script res://addons/make_weapon/headless/weapon_catalog_cli.gd -- --check
Godot_v4.6.3-stable_win64_console.exe --headless --path . --script res://addons/make_weapon/headless/weapon_catalog_cli.gd -- --export
```

需要单独同步某个客户端副本时仍可使用`./cp.config.from.server.sh 道具.武器.<类型>.yaml`, 但武器编辑器的“导出双版本”已经一次写入客户端8个文件.

## 合成素材源资料

`../../sa.desktop/config.raw/道具.素材.yaml` 保存原版8.0合成素材源资料, 供资源核对、运行配置生成和后续合成编辑器使用. 目录包含461条`type=16`基础/状态/稀有素材和48条`type=11, acode=By`镶嵌宝石, 共34系509条记录. 14个基础素材系的11-20级共140条当前为草稿, 未进入包含83个唯一资源帧的素材图集.

协议将普通道具、素材、货币和装备直接划分为同级分类: 合成相关普通道具预留`3090000-3099999`, 素材使用`3100000-3199999`, 货币使用`3490000-3499999`, 装备使用`3500000-3999999`. 素材现代ID采用`310GGLL`, `GG`为素材系编号01-34, `LL`为等级或系内序号; 01-14使用01-20, 15-25及27-33使用01-10, 26仅使用01表示卡鲁娜矿石, 34使用01-48表示镶嵌宝石. 已核对记录标记为`published`, 确定性导出到`道具.素材.yaml`, 统一引用客户端`item/道具.素材`图集; 这只完成素材目录接入, 不代表合成结算逻辑已经接入.

`../../sa.desktop/config.raw/item.other.yaml`保存313条可参与合成但不属于基础素材的普通道具证据, 按原版ID升序固定分配`3090001-3090313`. 其中297条资源已核对并迁移到普通道具编辑器RAW, 16条对应正式缺失的15帧, 保留`draft`且不使用草稿目录的同号图片替代. `tool/other_item_catalog.py`负责确定性校验该证据目录; 正式`item.yaml`只由道具编辑器导出.

`../../sa.desktop/config.raw/item.synthesis.rule.yaml` 保存当前合成资料实际使用的50种原子、20档原子数值基准、基础随机匹配参数和8.5服务端扩展证据. `item.synthesis.original.yaml` 保存11271条8.0非料理合成输入记录及其中8756条结果资格. 这两个文件是原版只读基准, 不由编辑器或服务端改写. `item.synthesis.catalog.yaml`是Godot合成编辑器维护的当前有效源, 使用现代ID保存规则、草稿/发布状态、输入/产出资格和1-5种原子. 料理`type=20`属于独立系统, 不进入这三份非料理合成数据. 原版规则以8.0的`itemset6.txt/itematom.txt`作为内容基线, 以8.5的`item_gen.c`作为算法和宠物、家族、炼金扩展证据, 不把8.5源码中没有配套数据的分支反推成8.0素材.

`tool/synthesis_catalog.py`默认校验`item.synthesis.catalog.yaml`并生成只含已发布记录的`item.synthesis.yaml`; `--initialize-editor-catalog`只用于首次从原版证据和各道具RAW迁移, 不能覆盖日常编辑结果. 原版ID不会进入运行配置或服务端代码, 不建立运行时ID映射表. 当前源包含6254条已发布资格和5585种产物, 初次迁移前后现代ID及运行语义不变. 输出包含输入数量和冷却、5次候选重试、无候选回退、千分制概率、14档同原子累计系数、20档材料等级及50种当前合成原子定义. `common/gameconfig.SynthesisConfig`和`SynthesizeOrdinaryPet`仅作为暂停功能的离线解析、算法与测试代码保留, 不挂接 Online 配置管理器. 生成和校验命令:

```bash
python -B -m unittest tool.test_other_item_catalog tool.test_synthesis_catalog
python -B tool/other_item_catalog.py --write
python -B tool/other_item_catalog.py
python -B tool/synthesis_catalog.py --write
python -B tool/synthesis_catalog.py
# 仅首次迁移使用:
python -B tool/synthesis_catalog.py --initialize-editor-catalog
```

宠物加工修正证据由`tool/pet_synthesis_correction.py`读取8.0 `enemy1.txt/enemybase.txt`和当前`config/pet.yaml`, 输出到客户端`config.raw/pet.synthesis.correction.yaml`. 原版敌人ID重复时按服务端顺序查找的行为保留第一条; 后续宠物无法映射时显式输出`unmapped`, 不补默认值. 生成时必须显式传入正式8.0数据路径:

```bash
python tool/pet_synthesis_correction.py --enemy-table "<8.0>/gmsv/data/enemy1.txt" --enemy-base "<8.0>/gmsv/data/enemybase.txt" --write
python -m unittest tool.test_pet_synthesis_correction
```

2026-09-09资源审计确认非料理合成索引需要1116个唯一帧, 正式`已合成精灵集`与`原资源`联合覆盖1101帧, 缺失`397004-397018`共15帧. 服务端合成只依赖道具资格、原子值和规则计算, 不读取客户端PNG, 因此缺图不影响合成算法和结算; 对应道具在客户端正式发布前仍必须补齐或确认图标, 不允许用草稿目录中的同号冲突图片替代.

## 首饰编辑目录和发布

`../../sa.desktop/config.raw/item.armor.meta.yaml`、7 个 `道具.装备.<类型>.yaml` 和 2 个 `item.armor.<类型>.yaml` 是装备和首饰的编辑来源. 首饰在记录元数据中保存 `accessoryType`, `values` 保留当前支持的 79 个字段; 因尚不支持合成, 10 个材料字段已从 RAW 移除. Godot `make_armor` 右侧显示六类首饰及 proto ID 子区间, 类型和 ID 同次校验与保存; 装备分类由当前类型页签和对应源文件决定, 表单内不可修改.

发布后先使用编辑器的 `保存RAW`, 再使用 `导出C/S`. 导出器按铠甲, 头盔, 盾牌, 手套, 腰带, 鞋子和首饰直接生成 server/client 两端共14个 `item.equipment.<类型>.yaml`; 每个文件只包含对应唯一分组. 14份输出全部暂存并校验后才提交, 失败会回滚已提交部分. 宠装和参考资料不进入运行导出. 导出不调用同步脚本.

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

online 启动会加载 `character.yaml`, `技能.yaml`, `ai.yaml`, `enemy.group.yaml`, `enemy.exp.yaml`, `exp.yaml`, `item.yaml`, `道具.素材.yaml`, `item.currency.yaml`, `道具.天工司.yaml`, 8个`道具.武器.<类型>.yaml`, 7个`item.equipment.<类型>.yaml`, `reward.yaml`, `task.yaml`, `pet.yaml`, `商店.yaml` 和 `scene/*.yaml`. 旧 `item.synthesis.yaml` 不在启动加载链路中. 任一必需文件缺失, 字段非法或跨表引用无效时, 服务必须直接启动失败.

## 统一技能配置

`技能.yaml` 是技能编辑器确定性生成的 server 运行配置, 不允许反向编辑. 唯一编辑源是 `../../sa.desktop/config.raw/技能.yaml`.

`8000001-8000007` 依次为攻击、防御、逃跑、捕获、换宠、使用道具和更换装备, 在技能目录中统一归入 `basic_action` 基础动作, 不按各自处理器名称拆分功能分类. 原版宠物技能“待机”归入 `other`, “修复”归入 `craft_life`, 两者都不是基础动作.

`../../sa.desktop/config.raw/技能.yaml` 同时保存全量调研目录和 `runtimeSkills` 开发参数. 它聚合基础战斗动作、宠物/NPC技能、角色精灵与魔法、角色职业技能四类可再生证据, 可选保存编辑器新增的 `custom:<现代ID>` 技能, 并保留 `../docs/pet.skill.runtime.yaml` 中218条9000000段历史现代配置的迁移映射. 编辑器左侧把相同现代ID的来源记录合并为唯一技能实体, 并提供角色、宠物两个可用主体勾选列. 已实现技能至少勾选一项; “保存RAW”只更新唯一编辑源,“导出C/S”只把已实现技能写入 server/client 两端运行配置. “扫描ID引用”只读扫描两端 config 目录.

全量目录由 `tool/skill_catalog.py` 生成. 默认只检查来源与目录语义是否一致, 明确传入 `--write` 才原子写入; 已有人工 `curation` 按稳定 `key` 保留, 自定义技能完整记录也会保留, ID冲突时直接失败:

```bash
python tool/skill_catalog.py --write
```

每个 `skill[]` 条目必须配置:

- `id`: 技能资源 ID, 必须位于协议技能区间.
- `usableBy`: 非空单位类型数组, 只接受 `character` 和 `pet`, 同一类型不能重复.
- 服务端 `技能.yaml` 不包含客户端展示使用的 `name/description`; 两字段只保留在 RAW 和客户端运行配置中.
- 宠物初始槽位和学习请求只接受 `usableBy` 包含 `pet` 的技能; 不匹配时配置加载或请求会直接失败.
- `技能.yaml` 不保存售价或可学习状态. 宠物是否可学习及学习价格由 `商店.yaml` 中同技能 ID 的条目决定; 技能配置残留旧 `cost` 字段会直接加载失败.
- `continuationAttack.segmentCount`: 可选. 配置后表示连续攻击, 段数范围为 1-10.
- `mightyAttack`: 可选. 配置后表示一击必杀, 与 `continuationAttack` 互斥. 必须同时提供整数 `damageMultiplier` 和 `targetDodgeBonus`; 前者为最终伤害倍率, 范围1-655, 后者为目标基础闪避的百分点加值, 范围0-32767. 范围对应原版COM3低16位百分比和高16位有符号加值, 不接受缺项、空值或小数截断.
- `poisonAttack`: 可选. 普通中毒物理攻击, 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0). 时长是目标实际扣毒血的行动次数; 原版状态攻击会向运行态写入 `durationActions+1`, 让最后一次毒伤后的下一次行动只解除状态. 攻击修正是施放者攻击属性的百分比加值. 所有技能行为参数块彼此互斥.
- `stoneAttack`: 可选. 普通石化物理攻击, 必须同时提供整数 `durationActions` (1-32767) 和 `attackPercentModifier` (-100至0). 原版向运行态写入 `durationActions+1`, 且每一点剩余时长都会阻止目标行动, 因而配置3/9分别对应4/10次无法行动. 该块只允许宠物使用, 不接受`mpCost/targetScope`, 并与其他行为块互斥.
- `chargeAttack`: 可选. 突击蓄力攻击, 必须同时提供整数 `chargeRounds` (1-10) 和 `attackPercentModifier` (0-32767). 前者为完整等待行动次数, 后者为释放时基础攻击力的百分比加值. 不接受空值, 缺项, 字符串数字或小数.
- `earthRound`: 可选. 地球一周两阶段物理攻击, 必须提供整数`damagePercentModifier`(0-32767). 首次行动隐藏, 下一回合自动现身攻击; 百分比作用于命中、暴击、Guard和最低伤害之后的最终伤害. 原版121配置`200`, 即最终伤害乘`1+200%=3.0`, 不是两倍. 该块只允许宠物使用, 不接受`mpCost/targetScope`, 并与其他行为块互斥.
- `guardian`: 可选. 忠犬普通物理攻击及本回合守护关系参数. 整数`attackPercentModifier`必填, 整数`defensePercentModifier`可选; 两者使用int32范围且不额外设置旧源码没有的百分比上限. 守护关系和攻防修正均为回合运行态, 不永久修改基础属性.
- `noGuard`: 可选. 不防守战法的一回合姿态, 必须同时提供整数`dodgePercent`(0-32767)、`counterPercent`(0-255)和`criticalPercent`(0-255). 前两项进入原版闪避和非玩家反击公式; `criticalPercent`只保留原始命令参数, 8.5结算没有读取点. 该动作不需要目标, 不是Guard防御动作.
- `showMercy`: 可选. 手下留情只接受空对象 `{}`, 不接受空值或参数. 先完成一次普通物理判定, 本次伤害若致死则限制为目标当前HP减1, 目标1HP时允许0伤害. 不形成持续保命状态.
- `poisonSpirit`, `stoneSpirit`, `confusionSpirit`, `drunkSpirit`, `sleepSpirit`: 五类角色装备异常精灵的独立参数块, 分别固定对应状态ID 1、4、6、5、3. 每个技能同时配置自己的 `mpCost`, `targetScope`, 行动时长、基础成功率、等级差范围及施放/受术/持续状态特效ID. 五类参数块彼此互斥, 也不能与其他技能行为块并存.
- `healingSpirit`, `moistureSpirit`, `graceSpirit`: 治愈、滋润、恩惠三类角色治疗精灵的独立参数块. 每条技能集中保存 `mpCost`, `targetScope`, `healPower`, `castEffectId` 和 `healEffectId`; 目标范围依次固定为 `self`, `singleAlly`, `allyCamp`. 三类参数块彼此互斥, 也不能与其他技能行为块并存.

装备条目的 `magicid` 只授予一个现代技能ID, 不保存技能耗蓝或成功率. 原版130-139、150-159、160-169、170-179、180-189依次迁移到 `8200130-8200139`、`8200150-8200159`、`8200160-8200169`、`8200170-8200179`、`8200180-8200189`; 耗蓝和全部行为参数只在上述技能条目中维护.

原版治愈0-4、滋润10-19、恩惠20-31分别迁移到 `8200000-8200004`、`8200010-8200019`、`8200020-8200031`. 共1915条装备引用只保留现代技能ID, 历史 `magicMp/magicusemp` 已移除. 迁移工具对旧ID 0额外要求同时匹配“治愈的精灵 Lv1”和MP 5, 不修改普通零值装备.

`商店.yaml` 当前开放的 21 个宠物技能价格来自原版8.0 `gmsv/data/petskill2.txt` 的价格列, 并由8.5服务端 `npc_petskillshop.c` 读取 `PETSKILL_COST` 的逻辑交叉确认: 待机500, 攻击/防御各1000, 破除防御1500, 二至十段攻击依次为2000, 5000, 15000, 25000, 225000, 625000, 625000, 1625000, 2005000; 一击必杀2500, 一击必杀改和改2各25000, 猛毒攻击4000, 突击4000, 双重突击8000, 加工1500, 手下留情10000. 本项目按商品 `costs` 直接扣款, 不应用原版NPC可选的 `skill_rate` 倍率.

三重突击605已在编辑器RAW中映射为 `8100605`, 保存 `chargeRounds: 3` 和 `attackPercentModifier: 250`, 原版学习价证据为8500石币. 当前状态为 `pending_test`, 因而不进入本目录的正式 `技能.yaml` 或 `商店.yaml`; 自动化测试通过临时配置注入验证通用 `chargeAttack` 链路, 不代表已开放学习、出生模板或敌方AI.

原版宠物121“地球一周”已映射为`8100121`, RAW保存`earthRound.damagePercentModifier: 200`, 原说明和2000石币证据继续保留. 原版120是被GM取消并替换为背水之战其之2的历史记录, 只从技能编辑器目录排除, 逆向审计资料继续保留. 121当前为`pending_test`, 因而正式`技能.yaml`、客户端配置、商店、出生模板和敌方AI均不包含它.

原版60“毒攻击”和61“猛毒攻击”是同一`PETSKILL_StatusChange`的两个参数等级, 不是重复记录. 60映射`8100060`, RAW使用`poisonAttack: {durationActions: 3, attackPercentModifier: -30}`, 保留2500石币证据并标记`pending_test`; 因此正式C/S技能配置、商店、出生模板和敌方AI均不包含`8100060`. 61保持已发布的`8100061`, 5次/-30%和4000石币, 不被60覆盖.

原版80和509“石化攻击”是同一`PETSKILL_StatusChange`的两个参数等级, 分别映射`8100080/8100509`. RAW使用`stoneAttack: {durationActions: 3/9, attackPercentModifier: -30}`, 两项原价均为2300石币并标记`pending_test`; 原版708与509数据相同但继续保留为未实现草稿. 两个现代技能均不进入正式C/S技能配置、商店、出生模板或敌方AI.

忠犬20/21/22已在编辑器RAW中分别映射为`8100020/8100021/8100022`, 保存攻击修正`-20/-10/+50`和可选防御修正`无/+40/+100`. 三者原版学习价均为2500石币; `T忠犬/T忠犬2`保留为原始名称, 现代显示名为“忠犬改/忠犬2”. 当前均为`pending_test`, 因而未进入本目录正式`技能.yaml`或`商店.yaml`; `9000242/9000243/9000244`只作为历史配置迁移证据保留.

不防守战法150/151/152已在编辑器RAW中映射为`8100150/8100151/8100152`, 参数分别为`30/50/20`、`40/60/30`和`50/70/40`, 原价为3000/15000/25000石币. 三项均为`pending_test`, 正式`技能.yaml`和`商店.yaml`不包含它们; `9000008/9000009/9000010`只作为历史配置映射证据.

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
| 8100060/8100061 | 毒攻击/猛毒攻击 | 不支持 | 按 `poisonAttack` 支持, 60仅测试配置 | 按 `poisonAttack` 支持, 60仅测试配置 |
| 8100080/8100509 | 石化攻击 | 不支持 | 按 `stoneAttack` 支持, 仅测试配置 | 按 `stoneAttack` 支持, 仅测试配置 |
| 8100150-8100152 | 不防守战法 | 不支持 | 测试配置按 `noGuard` 支持 | 测试配置按 `noGuard` 支持 |
| 8100626 | 手下留情 | 不支持 | 按 `showMercy` 支持 | 按 `showMercy` 支持 |
| 8200000-8200031指定三段 | 治愈/滋润/恩惠精灵 | 装备授予后支持 | 不支持 | 不支持 |
| 8200130-8200189指定五段 | 五系异常精灵 | 装备授予后支持 | 不支持 | 不支持 |

原版626手下留情映射 `8100626`, 学习价10000石币, 使用 `showMercy: {}`. 名称和描述保留 `petskill2.txt:182` 原文. 技能不参加合击, 使用者整回合不能反击, 受击目标仍按普通规则反击. 限伤发生在普通命中, 暴击, Guard和最低伤害之后, 扣血和击飞判断之前; 限伤产生的0伤害保留原命中类型. 编辑器目录归入 `physical_attack` 并标记已实现, 不自动分配给宠物出生模板或敌人AI. 捕获及尚未接入的特殊伤害反应保持各自开发范围.

原版宠物30/31分别映射 `8100030/8100031`, 参数为1次蓄力/+90%攻击力和2次蓄力/+110%攻击力. 30在第二次行动, 31在第三次行动释放单次普通物理攻击. 修正作用于释放时的基础攻击力, 不直接乘最终伤害. 技能名称和 `description` 保留原文, 精确机制只写入配置注释和内部文档. 技能目录标记为已实现, 不自动给出生模板或敌方AI添加技能. 后续回合的技能和目标由服务端续用, 客户端跳过宠物选择直到释放完成.

原版宠物40、41、42分别映射 `8100040`、`8100041`、`8100042`, 参数为2倍/+30、3倍/+40、4倍/+50. 倍率作用于暴击、防御姿态减伤和最低伤害判定后的单段最终伤害; 闪避加值在基础75%封顶之前计入, 后置独立装备闪避继续生效. 它们不保证暴击或秒杀, 不参加合击, 开始主动行动后才取得普通反击资格, 反击不继承倍率和闪避加值.

技能目录将这三条记录归入 `physical_attack`, 标记为已实现. 原版宠物39的文案写2倍而参数为3倍/+500, 本地资料未确认其用途, 因此从编辑器目录及 `tool/skill_catalog.py` 的宠物生成入口排除; 原始 `docs/pet.skill.yaml` 保留, 其他来源系统的同号技能不受影响.

原版61映射 `8100061`, 使用实际启动日志确认加载的 `petskill2.txt` 第29行: `毒 turn 5 攻%-30`. 该行说明仍写减攻50%, 旧 `petskill.txt` 也确实配置-50%, 当前运行值遵循可执行参数-30%. 附毒按原版写入durationActions+1=6. 目标正常行动开始先减1, 计数仍为正数时扣毒血; 第5次扣血后保留计数1, 下一次行动只解除中毒和标记. 解除前不叠加或刷新, 解除后允许再次附毒. 毒伤按目标基础四维计算, 最多扣至1HP. 不给宠物出生模板或敌人AI自动添加该技能; 玩家学习后或敌人AI明确引用后才能使用.

“未开放”表示配置合法, 但 online 收到玩家动作后返回业务错误. “启动拒绝”表示该技能出现在敌人引用的 AI 技能列表时, Online 在注册 etcd 和 gRPC 前直接启动失败. 玩家宠物必须在自己的实例技能槽中持有技能, 敌方 NPC 必须在本场冻结的 AI 技能列表中持有技能.

## 宠物技能槽

`pet.yaml skill` 固定 7 槽, `0` 表示空槽, 非 0 ID 必须存在于 `技能.yaml`. 它只作为新宠物实例的出生技能模板; 创建后以 `PetRecord.skill_id_list` 为权威, 学习、替换和遗忘不回写模板. 捕获创建玩家宠物时也使用出生技能, 不继承敌人的 AI 技能.

`enemy.group.yaml` 的每个 `enemies[]` 必须配置 `battleAI`, 直接引用 `ai.yaml`. 敌人战斗技能完全由 AI 定义, 不回退 `pet.yaml skill`. 旧 `pet.yaml battleAI`、`enemies[].skill` 和 AI 分离权重字段都会使配置加载失败.

多数宠物当前配置为:

```yaml
skill: [8000001,8000002,0,0,0,0,0]
```

除阿布洞窟敌群70000外, 当前敌人统一使用 AI 1, 攻击、防御、逃跑权重为 `10:1:1`; 敌群70000的五名守护者按顺序使用 AI 13-17. 敌人 AI 只保存在服务端战斗运行态, 不写入 `CombatUnit.skill_id_list`.

## 宠物主体配置

`pet.yaml` 使用 `pet.<family>: [...]` 按系别组织. server 加载后按宠物 ID 建立全局索引, 运行时不保留系别层级.

server 消费的主要字段:

- `id`, `name`, `rarity`.
- `elemental`: 地、水、火、风使用整数百分比, 总和必须为100, 只能是单元素或两个相邻元素.
- `attribute`: 异常抗性、暴击、反击、捕获和服务端战斗特性.
- `growth`: 初始和升级成长参数.
- `panelReference`: 客户端图鉴直接展示的1级和140级普通品阶平均值、神话品阶平均值及总成长上下限, 只保存服务端预计算结果.
- `skill`: 新宠物出生时的固定7槽技能.
宠物模板不保存战斗 AI 引用; 同一种宠物可由不同敌人条目选择不同 AI.

此外, `pet.yaml` 和 `pet.sprite.yaml` 的条目都可保存可选的编辑器元数据 `testStatus`. 缺省或0表示未测试, 1表示通过, 2表示未通过; 状态0不落盘. 该字段不参与 server 或客户端运行时业务, 宠物主体和 sprite 的测试状态相互独立.

`ai.yaml` 使用 `ai: [...]` 保存共享配置, `skills[]` 将技能 `id` 与相对 `weight` 放在同一条记录中. 攻击、防御、逃跑及特殊技能使用同一种结构, 不要求凑满7槽. 技能 ID 不得重复, 权重范围为 `[1,2147483647]`, 不使用的技能直接移除, 总权重必须处于 `[1,2147483647]`. `targetScope`、`targetSelection` 和可选 `targetRandomRollMax` 保留现有目标选择语义.

`enemy.group.yaml enemies[].weight` 控制出怪时选择哪种敌人, `ai.yaml skills[].weight` 控制战斗时选择哪个技能. 要给同一种宠物设置不同的技能概率, 定义不同 AI 并在对应敌人条目中引用.

自动遇敌从敌人组分别取得宠物模板、AI和普通掉落槽. `load()` 校验单表结构, `check()` 校验 AI 到技能、敌人组到宠物、AI及掉落道具的跨表引用, `assemble()` 在敌人条目挂载只读 AI. Online 在注册服务前验证每个敌人 AI 技能的 NPC 执行能力, 建房时深拷贝技能、权重及目标策略并抽取本场敌人携带的普通掉落; 回合中不再查询宠物模板的技能或 AI.

`growth` 保存非负的原版模板值, 但应用品阶偏移和原版公式后, 宠物实例的 `SavedBase*`、`Raw*` 以及成长基线中的防御、敏捷均使用 `int32`, 允许为0或负数. 配置和档案校验不得再要求这些中间值大于0; 只有派生后的最大生命和攻击必须大于0, 才能构成有效存活战斗单位.

### 图鉴面板参考值核验

`panelReference` 的完整计算规则记录在 `pet.yaml` 文件头. 客户端运行时只解析并显示这些预计算结果, 不保存成长基础四维, 也不实现参考值算法. Godot `make_pet` 编辑器另有一份仅供编辑时实时预览和写回的 GDScript 实现; 它不进入客户端运行时, 并通过全量宠物测试逐项对照 server 权威结果.

online 启动加载配置时, server 使用权威宠物生成、升级和面板换算规则重新计算所有宠物的 `panelReference`. 核验会集中收集全部不一致项, 每项日志包含宠物 ID、名称、配置实际值以及可直接复制回 `pet.yaml` 的正确 YAML. 存在任一不一致时配置加载失败, 服务不得启动.

`make_pet` 编辑器以客户端仓库的 `config.raw/pet.yaml` 和 `config.raw/pet.sprite.yaml` 为唯一编辑源, 并读取 `config.raw/技能.yaml` 供选择出生技能. 它不读取 AI 配置, 不新增、删除或重排宠物和sprite. `保存RAW`校验并原子提交两份编辑源; `同步C/S`去除RAW顶层 `format/status`, 检查外部修改后原子生成本目录和客户端 `config/` 中的4份运行配置. 本目录的宠物运行配置不得直接编辑, 也不得通过客户端复制脚本反向覆盖.

## 跨表关系

主要引用关系:

```text
scene/*.yaml
  -> enemy.group.yaml
       -> pet.yaml (出生技能 -> 技能.yaml)
       -> ai.yaml (战斗技能及权重 -> 技能.yaml)
```

- `scene/*.yaml` 不设置格式版本字段, 地图 ID 与客户端 `map_id` 一致; `collision.blockedRows` 保存服务端阻挡, `encounter.enabled` 和 `encounter.enemyGroups` 定义全地图遇敌开关与敌人组权重, `npcs` 保存NPC实体及其独立功能选项, `warps` 保存传送起点与目标. 当前目录包含70000至70002这3张任务地图, 80000、80001、80010、80020、80030、80040、80050、80060、80070这9张测试地图, 以及从90001开始的14张练级地图: 90001和90010至90130按10递增. 正常角色地图进入允许任务范围`[70000,79999]`、测试范围`[80000,89999]`和练级范围`[90000,99999]`. 可进入地图必须启用遇敌并配置有效敌人组.
- `encounter.enabled` 必须显式配置; 启用遇敌时 `encounter.enemyGroups` 不能为空且总权重必须大于0.
- `enemy.group.yaml enemies[].id` 引用宠物模板, `enemies[].battleAI` 必填且引用 `ai.yaml`.
- `ai.yaml skills[].id` 引用 `技能.yaml`, 权重与技能 ID 在同一条记录中.
- `pet.yaml` 的非0出生技能槽引用 `技能.yaml`, 不参与敌人 AI 的技能选择.

server 只校验 YAML 结构、服务端消费字段、枚举、数值范围和跨表引用. 名称、描述、sprite、PNG、`.tpsheet` 和动画帧完整性由 sa.desktop 的资源流程校验.

## 捕获资源与配置

`8000004` 是角色基础动作, 不新增学习费用或宠物技能槽配置. 捕获权限由 `enemy.group.yaml captured` 决定, Boss 固定禁止; 基础捕获值使用 `pet.yaml attribute.get`. CaptureSnapshot 在开战时冻结出生技能和实际个体, 捕获成功不会把敌群 AI 带入玩家宠物档案.

`common.sprite.yaml` 注册状态图集中的 `8820` Capture! 和 `8814` Get!, 失败复用已有 `8813` Fail.... 不增加捕获按钮资源.

`pet.sprite.yaml` 可成对配置 `walkActionSoundFrameNumberList` 和 `walkActionSoundIdList`, 省略时表示 Walk 没有声音. 两个数组等长, 帧号从 1 开始且严格递增, 不得超出任一方向 Walk 的长度. 当前原版 ID 支持 76(`sae_26.wav`) 和 79(`sae_29.wav`). sprite 354/584 使用帧 1、4 的 76, 1152/1153/1154/1155 使用帧 2 的 79, 八方向共享. 客户端和宠物编辑器均按 Walk 帧事件消费, 与攻击的动作声音、命中声音分别保存.

