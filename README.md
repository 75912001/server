# grpc/template

## Generate protobuf code (Including standard stubs & Go-gRPC-X extensions)

`./proto` 是服务端和 sa.desktop 共享协议的唯一源头。客户端仓库只在生成前从这里单向同步 `.proto` 文件, 不反向维护独立协议定义。

To generate all files in `./proto/pb` (including `*.pb.go`, `*_grpc.pb.go`, and `*_grpc.x.pb.go`):

```bash
python gen.py
```

`gen.py` 会先根据源 proto 中的 `//0xHEX#...` 注释生成 `*.cmd.proto`。生成的 cmd 枚举值使用十进制数字, 注释保留原始十六进制命令号, 以同时兼容 `protoc` 和 sa.desktop 使用的 Godobuf 解析器。

```bash
go get github.com/75912001/xlib@latest
```

## 待办

- [ ] 补充服务器重启, 宕机, 用户重连正确性验证。覆盖 login accountVerifyToken/session, connectTicket, gateway UserVerifyReq, cache userSession CAS, online bind, heartbeat 恢复路径; 重点确认旧 cache userSession 不阻塞重登, 顶号清理不误删新 cache userSession, gateway/online 绑定最终一致。
- [ ] 修复 connectTicket 可重复使用问题。当前同一个 `connectTicket` 在有效期内可重复通过 gateway `UserVerifyReq`, 并触发顶号; 应改为一次性消费语义, 首次验证成功后后续复用返回未认证, 且不进入顶号和 bind 流程。可选方案: 在 gateway 中缓存已使用 ticket/nonce 并定期清理过期数据, 或在 Redis 中记录票据已使用状态并通过原子操作消费。
- [ ] 收口 cache gRPC 控制面暴露风险。当前非服务调用方可直接调用 `CacheBeginUserSessionCAS` 写入伪造 `user:{uid}:session`, 可能阻塞用户登录; 需要限制端口访问, 增加服务间鉴权, 并限制 cache userSession TTL 上限。
- [ ] 收口 gateway gRPC 控制面暴露风险。当前 `GatewayKickUser` 会断开用户连接并清理 cache userSession, 但接口本身未做服务间鉴权; 需要限制端口只对可信内网服务开放, 并增加 mTLS 或 metadata token/HMAC 校验调用方身份。
- [ ] 修复 online `OnlineStreamTunnel` 未鉴权可绑定真实 `gateway_key` 的问题。当前非 gateway 调用方可能覆盖真实 gateway stream, 导致下行消息丢失或被劫持; 需要限制端口访问并校验 gateway 身份。
- [ ] 修复 online `OnlineStreamTunnel` 未鉴权可伪造任意在线 uid 上行业务帧的问题。当前非 gateway 调用方可绕过 gateway 登录态和 heartbeat 校验, 直接驱动 online 用户命令; 需要校验 stream 调用方身份和 gateway_key, 并确保用户帧绑定到可信 gateway userSession。
- [ ] 收口 login `/api/login/accountVerifyToken` 写入入口。当前接口未鉴权可为任意 account 写入 accountVerifyToken, 进而通过 `/api/login/session` 获取 connectTicket; 需要将 accountVerifyToken 写入接口收口到可信认证服务, 增加鉴权或签名校验, 并避免直接暴露给客户端或公网。
