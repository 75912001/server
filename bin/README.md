## Run server

## gRPC 消息大小配置

`bin`、`bin/*.template` 和 `deploy` 中的 Cache、Gateway、Login、Online 配置均将单条 gRPC 收发消息上限设置为 64MiB:

```yaml
grpc:
  maxReceiveMessageBytes: 67108864
  maxSendMessageBytes: 67108864
```

Login 未配置 `listenAddr`, 因此不会启动 gRPC 服务端; 该段配置仅供 Login 发起的 gRPC 客户端连接使用.

更新包含该配置能力的 xlib 版本后, 运行 `python 1.gen.py` 重新生成 `proto/pb/*_grpc.x.pb.go`, 使生成客户端应用相同限制.

# 删除日志
```bash
rm -rf ./log/*
```
