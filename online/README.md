# Online 服务

Online 服务负责 Account actor、账号业务、角色业务和 PVE 战斗. gateway 负责连接、登录会话、顶号和断线编排.

部署、端口、容器启动和验证命令见 `deploy/online/README.md`.

## 能力边界

Online 当前负责:

- `OnlineBindAccount` 和 `OnlineUnbindAccount`.
- 通过 `OnlineStreamTunnel` 接收 gateway 转发的客户端业务包并返回业务响应.
- 读取、校验和持久化 `AccountRecord`.
- 角色创建、上线、下线、场景切换和自动遇敌.
- 宠物携带状态、仓库存取和昵称.
- 道具使用、背包和仓库存取.
- GM 结构化命令.
- 每场独立 `CombatRoom` actor 的 PVE 回合、动作和结算.

Online 不负责:

- 写入或续期 cache account session.
- 顶号和连接断开.
- 客户端图片、sprite、`.tpsheet` 或动画帧完整性校验.

业务失败只返回对应业务错误码. Online 返回的 `InvalidArgument`、`FailedPrecondition`、`NotFound` 或 `Internal` 不表示要求 gateway 断开用户连接. gateway 只有在连接协议、会话安全或服务自身明确进入断线流程时才断开.

## 共享游戏配置

`custom.gameConfigDir` 指向共享配置目录, 未配置时默认读取当前工作目录下的 `config`.

Online 启动必需文件:

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

Docker 镜像会把仓库 `config/` 复制到 `/app/config`, `deploy/online/*.yaml` 使用 `custom.gameConfigDir: /app/config`.

配置加载依次执行单表 `load`、跨表 `check` 和 `assemble`. 文件缺失、字段非法或引用无效时直接启动失败, 不继续注册 etcd 或启动业务服务.

自动遇敌按 `scene.yaml -> enemy.group.yaml -> pet.yaml` 生成单位. 场景按权重选择敌人组, 敌人组的 `enemies[].id` 直接引用宠物模板; 数量和等级来自敌人组, 属性和 AI 来自宠物模板, 经验由 `enemy.exp.yaml` 结合宠物模板计算.

## 宠物创建与品阶

创建宠物时显式传入 `Common` 到 `Mythic`, 会分别对四项 SavedBase 统一应用 `-2`、`-1`、`0`、`1`、`2` 偏移. 传入 `Unknow` 表示随机品阶: 四项分别独立等概率随机整数偏移 `[-2,2]`, 再按总偏移计算并保存实际品阶, `PetRecord.grade` 不会保存 `Unknow`.

随机品阶按总偏移划分为 `Common=-8~0`、`Rare=1~2`、`Epic=3~4`、`Legendary=5~6`、`Mythic=7~8`, 对应概率为 `56.80%`、`23.68%`、`13.92%`、`4.80%`、`0.80%`.

## 统一技能模型

`skill.yaml` 是角色和宠物共用的唯一技能定义来源. `pet.yaml skill` 只保存宠物实际拥有的技能 ID, 不保存技能行为.

当前角色动作:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8000003`: 逃跑.
- `8000004-8000007`: 配置保留, 当前未开放. 请求返回 `InvalidArgument`, 不执行动作.

当前宠物解析器支持:

- `8000001`: 攻击.
- `8000002`: 防御.
- `8100000`: 待机.
- `8100003`: 破除防御.
- 配置了 `continuationAttack.segmentCount` 的连续攻击.

宠物使用技能必须同时满足:

1. 技能存在于 `skill.yaml`.
2. 技能 ID 出现在该宠物模板的 `pet.yaml skill` 槽位.
3. Online 已实现对应行为.
4. 请求目标和阵营参数合法.

当前 `pet.yaml` 中所有宠物都只配置 `[8000001,8000002,0,0,0,0,0]`, 因此生产宠物当前只能主动使用攻击和防御.

未知技能、未开放技能、未持有技能和非法目标都返回 `CombatRoundActionRes` 的 `InvalidArgument`. 该响应不会触发 gateway 断线.

## 角色交互设置

`CharacterSettingSetReq` 修改当前账号登录会话内指定角色的组队和决斗开关. 状态保存在 Account actor 的角色管理器中, 不进入 `CharacterRecord`, 也不调用 cache 持久化.

每次 `OnlineBindAccount` 创建新的角色管理器时, 所有角色的组队和决斗开关都初始化为关闭. 客户端发送修改请求后保持原显示状态, 仅在收到 `CharacterSettingSetRes` 成功回复后应用服务端返回的权威值; 请求失败时继续显示原状态.

## 战斗流程

```text
CombatBattleStartNotify
  -> CombatRoundActionReq
  -> CombatRoundActionRes
  -> CombatRoundResultNotify
  -> CombatFlowCompleteReq
```

- `CombatBattleStartNotify` 下发 `battle_id` 和初始单位列表.
- 角色和当前参战宠物分别提交动作.
- 每个单位每回合只能提交一次合法动作.
- 所有存活可控单位提交后立即结算; 超时单位使用默认防御.
- `CombatRoundResultNotify.event_list` 和其中的动作、效果数组顺序就是客户端播放顺序.
- 战斗结束后结算敌人经验, 再等待客户端完成表现流程.

基础物理战斗当前保留普通攻击、防御、逃跑、待机、破防、连续攻击、合击、反击、伤害、死亡和 PVE 结算. 合击每个成员都下发自己的`CombatEffect.damage`, 前序成员以`displayed_damage=0`和空`unit_delta_list`只表达命中、暴击及防守表现, 最后一名成员才携带合击总表现伤害和累计 HP 差量. 已移除的旧宠物技能处理器、状态链和专用配置不会再通过兼容分支执行.

## 账号绑定与解绑

`OnlineBindAccount`:

1. 校验 aid、account、gatewayKey 和 accountSession.
2. 从 cache 读取并校验 `AccountRecord`.
3. 创建或绑定 Account actor.
4. 建立角色管理器并保存 gateway stream.

`OnlineUnbindAccount`:

1. 校验 gatewayKey 和 accountSession 与当前 actor 匹配.
2. 持久化仍在线角色的登出状态.
3. 取消自动遇敌和战斗引用.
4. 停止 Account actor.

cache account session 的结束由 gateway 负责.

## 排障

- `load game config failed`: 检查 `custom.gameConfigDir`、必需 YAML 文件和跨表引用.
- `invalid combat action`: 检查 battle ID、round、单位归属、技能是否存在、宠物是否持有技能及目标是否合法.
- online 返回业务错误后连接仍在: 这是预期行为, 客户端可修正请求后继续使用当前连接.
- 业务包无响应: 检查 gateway stream、online Account actor 和目标 aid.
- `DeadlineExceeded`: 检查 gateway 到 online 的 RPC 超时、online 日志和 actor 是否阻塞.

## 后续建议

- 为 `OnlineBindAccount` 和 `OnlineUnbindAccount` 补齐 actor 生命周期集成测试.
- 继续拆分较大的战斗结算文件, 但不要恢复已移除的旧技能兼容层.
