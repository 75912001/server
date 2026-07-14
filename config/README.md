# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头.

当前共享文件:

- `character.yaml`: 角色资源配置.
- `enemy.group.yaml`: 敌人组配置.
- `enemy.exp.yaml`: 敌人等级基础经验配置.
- `exp.yaml`: 经验等级配置.
- `pet.skill.yaml`: 宠物技能配置.
- `pet.yaml`: 宠物主体配置.
- `scene.yaml`: 场景配置, 维护场景可遇敌敌人组和权重.

server 只校验 YAML 自身结构, 服务端消费字段, 枚举, 数值范围和跨配置引用. 其中 `character.yaml` 在 server 侧只消费并校验 `id` 和 `isRole`; `scene.yaml` 用于 online 根据角色当前场景按权重选择敌人组, 并校验 `enemyGroups[].id` 必须存在于 `enemy.group.yaml`; `enemy.group.yaml` 用于生成敌人模板, 数量和等级; `enemy.exp.yaml` 提供敌人生成时按等级查询的基础 EXP; `pet.yaml` 在 server 侧消费并校验 `id`, `rarity`, `elemental`, `attribute`, `growth`, `skill`, `battleAI` 及技能引用. `growth` 用于生成宠物战斗属性; `attribute.get` 和 `attribute.rate` 与异常抗性、暴击、反击字段一起参与敌人生成EXP公式; `skill`保存玩家宠物可配置技能; `battleAI`仅用于敌方PVE单位, 使用完整英文值配置攻击、防御、逃跑相对权重及普通攻击目标规则. `escapeWeight`是敌方基础逃跑动作权重, 逃跑不是宠物技能; 三项权重总和为0时服务端直接防御. `targetScope`允许`allOpponents`、`playerCharacters`、`playerPets`、`partyLeader`; `targetSelection`允许`random`、`highestHp`、`lowestHp`、`highestAttack`、`highestAgility`、`lowestAgility`、`elementalSubdue`. `pet.skill.yaml`提供技能槽引用校验; `exp.yaml`用于角色经验推导等级和敌方等级换算. 角色和宠物的名称、描述、颜色、栖息地、出生地、sprite、PNG、`.tpsheet`和frame资源完整性继续由sa.desktop校验.

客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.
