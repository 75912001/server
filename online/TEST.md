# Online 服务测试指南

## 适用范围

修改 online actor 绑定/解绑流程, Account actor 状态, gateway stream 路由, 共享游戏配置加载, `AccountRecordReq/AccountCreateReq` 业务 handler 或 online gRPC handler 时, 使用本文档.

## 快速检查

```bash
go test ./common/gameconfig ./online ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./online
```

## 依赖检查

当修改 gateway cache userSession 编排、gateway 路由、login 或 proto 契约时，运行：

```bash
go test ./online ./gateway ./cache ./login ./tool/robot/main ./proto/pb
```

## 运行时依赖

手动验证 online 需要：

- `bin/online.yaml` 中的 etcd 地址
- `bin/online.yaml` 中的 online gRPC 监听地址
- `bin/online.yaml` 中的 `custom.gameConfigDir`, 本地 bin 启动默认应指向项目根目录 `config`
- `custom.gameConfigDir` 下存在 `character.yaml`、`enemy.group.yaml`、`exp.yaml`、`pet.skill.yaml` 和 `pet.yaml`
- 已注册到 etcd 的 gateway 服务，用于下行路由
- 已注册到 etcd 的 cache 服务，用于用户档案和 cache userSession 状态

## 手动验证

典型检查项：

- `OnlineBindUser` 会从 cache 读取 account record；登录票据校验和 cache userSession 编排由 gateway 负责
- online 启动会先加载共享游戏配置; 配置缺失, 服务端消费字段非法或跨表引用错误时应启动失败, 且不继续注册 etcd/gRPC
- `pet.yaml` 的 server 校验范围只包含 `id`, `rarity`, `elemental`, `attribute`, `growth`, `skill` 和技能引用; 宠物名称, 栖息地, 出生地, 描述, sprite, PNG, `.tpsheet` 和 frame 完整性由 sa.desktop 校验
- `AccountCreateReq` 会按客户端传入的 `character_slot_index` 在 cache 账号壳档案上初始化 `account_record_create_timestamp_ms/used_uuid/character_record_list`, 并写回 cache
- 新账号首次登录后通过 `AccountCreateReq` 能拿到默认角色和宠物; 重启或重登后通过 `AccountRecordReq` 能读回同一份 `AccountRecord`
- 新建 `AccountRecord.uid > 0`, `character_record_list` 非空, 至少一个角色 `uuid > 0`, 默认 `CharacterRecord.asset_id == 1000011`, 宠物记录仍挂在角色下
- online 只绑定 Account actor，不写入 cache userSession 到 cache
- 重复登录会正确删除或替换旧 cache userSession
- gateway stream 能接收下行 frame
- 解绑只在 `gatewayKey + userSession` 匹配时清理 actor，不写 cache userSession
