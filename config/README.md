# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头. 客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.

## 配置文件

- `character.yaml`: 角色资源配置.
- `character.sprite.yaml`: 角色及角色骑宠动画帧、逐动作 FPS、攻击声音、命中表现和原版 Raw 参考配置.
- `common.sprite.yaml`: 通用精灵资源配置; `atlas` 使用相对 `assets` 的无扩展名路径并且必须以 `common/` 开头, `value` 保存客户端逐帧消费的有序帧号, 8702-8710和347511-347513是地水火风的大中小属性图标, 242302是设置窗口角色随身物品位置的原版背景板, 暴击8723条目合并保存前14帧小星和后13帧大星的60Hz时间线; 9195和9196分别是设置窗口角色属性加点按钮的未按下和按下状态. 900000100/900000110/900000120/900000130分别是普通伤害、暴击伤害、HP恢复和MP恢复的数字精灵组, 每组`value`按0-9顺序保存十帧.
- `skill.yaml`: 角色和宠物共用的技能配置.
- `enemy.group.yaml`: 敌人编组、宠物模板、数量和等级规则.
- `enemy.exp.yaml`: 敌人等级基础经验配置.
- `exp.yaml`: 角色和宠物等级经验配置.
- `information.yaml`: 从 STW1.13 `Mission.txt` 转换的 UTF-8 石器情报树和正文, 由 sa.desktop 只读展示.
- `item.yaml`: 普通道具和角色资产配置, 使用 `items.item.<id>` 分组; 非零 `sprite` 必须同时配置以 `item/` 开头的无扩展名 `atlas` 路径.
- `item.weapon.yaml`: 八类武器配置, 使用 `items.<weaponGroup>.<id>` 分组; 武器ID必须位于对应协议分组区间. 武器条目可保存原版名称、说明、价格、装备等级、职业限制、套装编号、攻击次数、能力随机范围、元素和异常抗性; `cost > 0` 表示可在全局商店购买, `cost = 0` 表示不可购买; 未配置的数值字段默认为0, 非法范围会导致启动失败.
- `pet.yaml`: 宠物主体、成长、图鉴面板参考值、技能槽和战斗 AI 配置.
- `pet.sprite.yaml`: 宠物动画帧和攻击表现配置.
- `scene.yaml`: 与客户端 `map_id` 一致的地图尺寸、阻挡、默认/区域遇敌规则和传送配置.
- `../docs/offset.yaml`: 帧资源图片偏移配置.

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

online 启动会加载 `character.yaml`、`skill.yaml`、`enemy.group.yaml`、`enemy.exp.yaml`、`exp.yaml`、`item.yaml`、`item.weapon.yaml`、`pet.yaml` 和 `scene.yaml`. 任一必需文件缺失、字段非法或跨表引用无效时, 服务必须直接启动失败.

## 统一技能配置

`skill.yaml` 是 server 唯一的技能定义来源, 不再存在独立的宠物技能配置文件.

每个 `skill[]` 条目必须配置:

- `id`: 技能资源 ID, 必须位于协议技能区间.
- `name`: 非空技能名称.
- `description`: 非空技能说明.
- `continuationAttack.segmentCount`: 可选. 配置后表示连续攻击, 段数范围为 1-10.

当前 online 执行能力如下:

| 技能 ID | 名称 | 角色 | 宠物 |
| --- | --- | --- | --- |
| 8000001 | 攻击 | 支持 | 支持 |
| 8000002 | 防御 | 支持 | 支持 |
| 8000003 | 逃跑 | 支持 | 不支持 |
| 8000004 | 捕获 | 未开放 | 不支持 |
| 8000005 | 换宠 | 未开放 | 不支持 |
| 8000006 | 使用道具 | 未开放 | 不支持 |
| 8000007 | 更换装备 | 未开放 | 不支持 |
| 8100000 | 待机 | 不支持 | 支持 |
| 8100003 | 破除防御 | 不支持 | 支持 |
| 8100010-8100018 | 连续攻击 | 不支持 | 按 `segmentCount` 支持 |

“未开放”表示配置合法, 但 online 收到动作后返回业务错误, 不执行行为. 技能存在于 `skill.yaml` 只是第一层条件; 宠物还必须在自己的 `pet.yaml skill` 槽位中实际拥有该技能.

## 宠物技能槽

`pet.yaml` 的 `skill` 字段按顺序保存技能槽, `0` 表示空槽. 非 0 技能 ID 必须存在于 `skill.yaml`.

当前所有宠物统一配置为:

```yaml
skill: [8000001,8000002,0,0,0,0,0]
```

因此当前生产配置中的宠物只能主动提交攻击和防御. 即使 `skill.yaml` 中还定义了待机、破防或连续攻击, 未分配到宠物技能槽前也不能使用.

## 宠物主体配置

`pet.yaml` 使用 `pet.<family>: [...]` 按系别组织. server 加载后按宠物 ID 建立全局索引, 运行时不保留系别层级.

server 消费的主要字段:

- `id`, `name`, `rarity`.
- `elemental`: 地、水、火、风总和必须为 10, 只能是单元素或两个相邻元素.
- `attribute`: 异常抗性、暴击、反击、捕获和服务端战斗特性.
- `growth`: 初始和升级成长参数.
- `panelReference`: 客户端图鉴直接展示的1级和140级普通品阶平均值、神话品阶平均值及总成长上下限, 只保存服务端预计算结果.
- `skill`: 统一技能槽.
- `battleAI`: 玩家宠物模板和自动遇敌单位共用的 AI 配置.

自动遇敌单位直接使用敌人组引用宠物的 `battleAI`. `battleAI.skillSlotWeights` 固定对应 7 个技能槽; 非 0 权重不能引用空槽.

### 图鉴面板参考值核验

`panelReference` 的完整计算规则记录在 `pet.yaml` 文件头. 客户端只解析并显示这些预计算结果, 不保存成长基础四维, 也不实现参考值算法.

online 启动加载配置时, server 使用权威宠物生成、升级和面板换算规则重新计算所有宠物的 `panelReference`. 核验会集中收集全部不一致项, 每项日志包含宠物 ID、名称、配置实际值以及可直接复制回 `pet.yaml` 的正确 YAML. 存在任一不一致时配置加载失败, 服务不得启动.

## 跨表关系

主要引用关系:

```text
scene.yaml
  -> enemy.group.yaml
  -> pet.yaml
  -> skill.yaml
```

- `scene.yaml` 使用 `sa-scene-v1`, 地图 ID 与客户端 `map_id` 一致; `collision.blockedRows` 保存服务端阻挡, `encounter.default` 和 `encounter.regions` 引用敌人组, `warps` 保存传送起点与目标.
- 遇敌区域按逐行 `[y,startX,endX]` 闭区间保存, 不得重叠或覆盖阻挡格; 未命中区域时使用全图默认规则.
- `enemy.group.yaml` 的 `enemies[].id` 直接引用宠物模板.
- `pet.yaml` 的非 0 技能槽引用 `skill.yaml`.

server 只校验 YAML 结构、服务端消费字段、枚举、数值范围和跨表引用. 名称、描述、sprite、PNG、`.tpsheet` 和动画帧完整性由 sa.desktop 的资源流程校验.
