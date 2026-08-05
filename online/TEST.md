# Online 服务测试指南

## 适用范围

修改 Account actor、共享配置加载、角色或宠物业务、`CombatRoom`、回合动作、gateway stream 路由或 online gRPC handler 时使用本文档.

## 快速检查

```bash
GOCACHE="$PWD/.gocache" go test ./common/gameconfig ./online ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./online
```

涉及 gateway、cache、login 或协议契约时追加:

```bash
GOCACHE="$PWD/.gocache" go test ./gateway ./cache ./login ./tool/robot/main
```

## 配置检查

`custom.gameConfigDir` 必须包含:

```text
character.yaml
skill.yaml
enemy.group.yaml
enemy.exp.yaml
exp.yaml
item.yaml
pet.yaml
scene.yaml
```

验证要点:

- 缺少任一必需文件时, online 明确启动失败.
- `skill.yaml` 的 ID、名称、说明和连续攻击段数非法时加载失败.
- `pet.yaml skill` 的非 0 ID 不存在于 `skill.yaml` 时加载失败.
- `scene.yaml` 引用不存在的敌人组时加载失败.
- `enemy.group.yaml enemies[].id` 引用不存在的宠物模板时加载失败.
- 当前所有宠物技能槽都是 `[8000001,8000002,0,0,0,0,0]`.
- 配置加载失败后不注册 etcd, 不启动 gRPC 业务入口.

## 统一技能测试

角色:

- `8000001` 生成普通攻击动作并校验敌方目标.
- `8000002` 生成以自身为目标的防御动作.
- `8000003` 生成逃跑动作.
- `8000004-8000007` 已配置但未开放, 必须返回业务错误且不写入本回合动作.
- `skill.yaml` 不存在的 ID 必须拒绝.

宠物:

- 技能必须同时存在于 `skill.yaml` 和该宠物的 `pet.yaml skill` 槽位.
- `8000001`、`8000002` 分别生成攻击和防御动作.
- 测试配置分配后, `8100000`、`8100003` 和连续攻击应由统一解析器生成对应动作.
- 未持有技能即使存在于 `skill.yaml` 也必须拒绝.
- 未实现行为即使已配置并持有也必须拒绝, 不允许回退成普通攻击、防御或待机.

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

## 基础战斗回归

- 攻击、闪避、暴击、伤害、权威 HP 和死亡结果一致.
- 防御在行动排序前生效, 并按独立动作返回.
- 合击只生成真实成员动作; 每个成员都携带独立 Damage 表现结果, 前序成员不显示伤害且没有 HP 差量, 首成员记录完整顶层来源, 最后一名成员显示并统一应用累计伤害.
- 反击链、最大深度和行动顺序稳定.
- 逃跑只移除实际参战单位, 无战宠时不得生成虚构宠物键.
- 宠物离场或死亡前已获得的战斗经验仍参与结算.
- PVE 经验持久化失败时不发布部分内存状态.
- 战斗结束后取消 timer、清理房间引用, `CombatFlowCompleteReq` 后按开关恢复自动遇敌.

## Docker 验证

1. 重新构建 online 镜像, 确保代码和 `/app/config` 来自同一工作树.
2. 启动 online 容器.
3. 查看容器日志, 确认配置加载成功、gRPC 监听成功且服务完成注册.
4. 若启动失败, 先修复日志中首个缺失文件或配置错误, 不用临时空数据绕过后续校验.
5. 发送一个未开放技能请求, 确认客户端收到业务错误且 gateway 连接保持可用.
