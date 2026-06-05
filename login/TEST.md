# Login 服务测试指南

## 适用范围

修改 login HTTP handler、账号 token 流程、gateway 选择、connectTicket 签发或 login 配置时，使用本文档。

## 快速检查

```bash
go test ./login ./proto/pb
GOCACHE="$PWD/.gocache" go build -buildvcs=false ./login
```

## 依赖检查

当修改 cache account token、connectTicket、服务发现或 robot 登录行为时，运行：

```bash
go test ./login ./cache ./gateway ./online ./tool/robot/main ./proto/pb
```

## 运行时依赖

手动验证 login 需要：

- `bin/login.yaml` 中的 etcd 地址
- `bin/login.yaml` 中的 login HTTP 监听地址
- 已注册到 etcd 的 cache 服务
- 已注册到 etcd 的 gateway 服务

## 手动验证

典型检查项：

- token 接口会写入 account token 到 cache
- session 接口只消费一次 account token
- session 响应包含 uid、connectTicket、ticketExpireAt、gateway key 和 gateway address
- session 响应不暴露内部 session TTL
- 返回 session 数据前不会调用 gateway prepare-login
