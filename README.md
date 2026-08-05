# grpc/template

## Generate protobuf code (Including standard stubs & Go-gRPC-X extensions)

`./proto` 是服务端和 sa.desktop 共享协议的唯一源头。客户端仓库只在生成前从这里单向同步 `.proto` 文件, 不反向维护独立协议定义。

`./config` 是服务端和 sa.desktop 共享配置的唯一源头。客户端仓库从 `server/config` 处单向同步配置文件, 不反向维护配置。

To generate all files in `./proto/pb` (including `*.pb.go`, `*_grpc.pb.go`, and `*_grpc.x.pb.go`):

```bash
python 1.gen.py
```

`1.gen.py` 会先汇总全部源 proto 中的 `//0xHEX#...` 注释, 校验命令值和消息名全局唯一, 再统一生成 `proto/cmd.proto` 中的 `MsgID` 枚举. 生成的 CMD 枚举值使用十进制数字, 注释保留原始十六进制命令号, 以同时兼容 `protoc` 和 sa.desktop 使用的 Godobuf 解析器.

```bash
go get github.com/75912001/xlib@latest
```

```bash
./2.deploy.sh
```

一键重建本地 Docker 部署的 `gateway`/`cache`/`online`/`login` 四个 `.1` 服务实例。

## 待办
