# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头. 客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.

## 配置文件

- `character.yaml`: 角色资源配置.
- `character.sprite.yaml`: 角色动画帧配置.
- `combat.sprite.yaml`: 战斗表现资源配置.
- `skill.yaml`: 角色和宠物共用的技能配置.
- `enemy.group.yaml`: 敌人编组、宠物模板、数量和等级规则.
- `enemy.exp.yaml`: 敌人等级基础经验配置.
- `exp.yaml`: 角色和宠物等级经验配置.
- `item.yaml`: 道具和装备配置.
- `pet.yaml`: 宠物主体、成长、技能槽和战斗 AI 配置.
- `pet.sprite.yaml`: 宠物动画帧和攻击表现配置.
- `scene.yaml`: 场景及其可遇敌编组配置.
- `../docs/offset.yaml`: 帧资源图片偏移配置.

online 启动会加载 `character.yaml`、`skill.yaml`、`enemy.group.yaml`、`enemy.exp.yaml`、`exp.yaml`、`item.yaml`、`pet.yaml` 和 `scene.yaml`. 任一必需文件缺失、字段非法或跨表引用无效时, 服务必须直接启动失败.

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

- `id`, `rarity`.
- `elemental`: 地、水、火、风总和必须为 10, 只能是单元素或两个相邻元素.
- `attribute`: 异常抗性、暴击、反击、捕获和服务端战斗特性.
- `growth`: 初始和升级成长参数.
- `skill`: 统一技能槽.
- `battleAI`: 玩家宠物模板和自动遇敌单位共用的 AI 配置.

自动遇敌单位直接使用敌人组引用宠物的 `battleAI`. `battleAI.skillSlotWeights` 固定对应 7 个技能槽; 非 0 权重不能引用空槽.

## 跨表关系

主要引用关系:

```text
scene.yaml
  -> enemy.group.yaml
  -> pet.yaml
  -> skill.yaml
```

- `scene.yaml` 引用敌人组.
- `enemy.group.yaml` 的 `enemies[].id` 直接引用宠物模板.
- `pet.yaml` 的非 0 技能槽引用 `skill.yaml`.

server 只校验 YAML 结构、服务端消费字段、枚举、数值范围和跨表引用. 名称、描述、sprite、PNG、`.tpsheet` 和动画帧完整性由 sa.desktop 的资源流程校验.
