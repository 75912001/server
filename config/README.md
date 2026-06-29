# 共享游戏配置

本目录是 server 和 sa.desktop 共享游戏配置的唯一源头.

当前共享文件:

- `character.yaml`: 角色资源配置.
- `enemy.group.yaml`: 敌人组配置.
- `exp.yaml`: 经验等级配置.
- `pet.skill.yaml`: 宠物技能配置.
- `pet.yaml`: 宠物主体配置.

server 只校验 YAML 自身结构, 服务端消费字段, 枚举, 数值范围和跨配置引用. 其中 `character.yaml` 在 server 侧只消费并校验 `id` 和 `isRole`; `pet.yaml` 在 server 侧只消费并校验 `id`, `rarity`, `elemental`, `attribute`, `growth`, `skill` 及技能引用. 角色和宠物的名称, 描述, 颜色, 栖息地, 出生地, sprite, PNG, `.tpsheet` 和 frame 资源完整性继续由 sa.desktop 校验.

客户端需要使用这些配置时, 从本目录单向同步到 sa.desktop 的 `config/` 目录, 不反向维护独立副本.
