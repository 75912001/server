# Online 服务测试指南

## 适用范围

修改 online actor 绑定/解绑流程、Account actor 状态、gateway stream 路由、`AccountRecordReq/AccountCreateReq` 业务 handler 或 online gRPC handler 时，使用本文档。

## 快速检查

```bash
go test ./online ./proto/pb
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
- 已注册到 etcd 的 gateway 服务，用于下行路由
- 已注册到 etcd 的 cache 服务，用于用户档案和 cache userSession 状态

## 手动验证

典型检查项：

- `OnlineBindUser` 会从 cache 读取 account record；登录票据校验和 cache userSession 编排由 gateway 负责
- `AccountCreateReq` 会在 cache 账号壳档案上初始化 `account_record_create_timestamp_ms/used_uuid/character_record_map`, 并写回 cache
- 新账号首次登录后通过 `AccountCreateReq` 能拿到默认角色和宠物; 重启或重登后通过 `AccountRecordReq` 能读回同一份 `AccountRecord`
- 新建 `AccountRecord.uid > 0`, `character_record_map` 非空, map key 等于 `CharacterRecord.uuid`, 默认 `CharacterRecord.asset_id == 1000011`, 宠物记录仍挂在角色下
- online 只绑定 Account actor，不写入 cache userSession 到 cache
- 重复登录会正确删除或替换旧 cache userSession
- gateway stream 能接收下行 frame
- 解绑只在 `gatewayKey + userSession` 匹配时清理 actor，不写 cache userSession
