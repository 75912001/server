## 客户端模拟器 Robot 模式

`robot` 用于模拟真实 TCP 客户端连接 gateway。现在统一使用 robot 管理器运行，配置 1 个 robot 时可以作为单用户调试，配置 1000/10000 个 robot 时可以作为压测客户端。

## 构建

在仓库根目录生成 protobuf 代码：

```bash
cd D:/src/github.com/server
python gen.py
```

生成客户端消息注册表并构建客户端：

```bash
cd D:/src/github.com/server/tool/robot
./gen.register.message.sh
./build.sh
```

## 运行

运行目录必须是：

```text
D:\src\github.com\server\tool\robot\bin
```

启动：

```bash
cd D:/src/github.com/server/tool/robot/bin
./robot.exe
```

客户端启动后会读取 `config.yaml`，发现 gateway/cache 服务，按 robot 配置分批连接 gateway，并自动发送 `UserVerifyReq` 登录。登录成功后会先发送 `UserRecordReq` 获取用户数据；如果用户数据为空才发送 `UserCreateReq` 创建用户。用户数据就绪后才会启动随机业务发包。

如果开启控制面板，启动日志会输出面板地址：

```text
control panel: http://127.0.0.1:18080/
```

## 配置

`bin/config.yaml` 中的 robot 配置示例：

```yaml
robot:
  count: 1
  uidStart: 10001
  uidStep: 1
  startupBatchSize: 100
  startupBatchInterval: 100ms
  heartbeatInterval: 10s
  actionInterval: 0s
  actionJitter: 500ms
  sendChanCapacity: 1000
  messages:
    - name: RobotPingReq
      weight: 80
    - name: UserRecordReq
      weight: 20
  logging:
    summaryInterval: 5s
    detailFailures: true
```

控制面板配置：

```yaml
controlPanel:
  enable: true
  addr: 127.0.0.1:18080
```

`enable` 为 `true` 时，客户端进程内会启动一个本地 HTTP 控制面板。`addr` 可以改成其他本机端口。

关键字段：

- `count`：启动 robot 数量。设置为 `1` 时就是单用户调试。
- `uidStart` / `uidStep`：生成 UID，例如 `10001,10002,10003`。
- `startupBatchSize` / `startupBatchInterval`：分批启动，避免瞬间打满本机和 gateway。
- `heartbeatInterval`：每个 robot 的心跳间隔。
- `actionInterval`：随机业务发包间隔。`0s` 表示不自动发业务包，只手工命令发包。
- `actionJitter`：随机发包抖动，减少所有 robot 同时发包。
- `messages`：自动业务发包的消息池，按 `weight` 权重随机选择。
- `summaryInterval`：汇总日志输出间隔。
- `detailFailures`：失败时输出详细日志。

1000 个 robot 示例：

```yaml
robot:
  count: 1000
  startupBatchSize: 100
  startupBatchInterval: 100ms
  actionInterval: 1s
```

10000 个 robot 示例：

```yaml
robot:
  count: 10000
  startupBatchSize: 100
  startupBatchInterval: 100ms
  actionInterval: 1s
```

## api.yaml

`api.yaml` 每次发包都会重新读取，所以运行中修改消息内容后，下一次发送会立即生效。

`RobotPingReq` 示例：

```yaml
RobotPingReq:
  id: 0x000010
  msg:
    seq: 0
    clientTime: 0
    payload: "robot-ping"
```

动态字段：

- `UserVerifyReq.uid` 和 `UserVerifyReq.connectTicket` 会被 robot 当前登录结果覆盖。
- `UserHeartbeatReq.lastHeartbeatSession` 为空时，会使用当前 robot 的 heartbeatSession。
- `RobotPingReq.seq` 为 `0` 时，会使用当前 robot 自增序号。
- `RobotPingReq.clientTime` 为 `0` 时，会使用当前毫秒时间。

## 命令

```text
list
stats
all RobotPingReq
uid 10001 RobotPingReq
quit
exit
```

说明：

- `list`：重新读取并列出 `api.yaml` 中的消息。
- `stats`：打印连接、登录、发包、收包、失败等统计。
- `all RobotPingReq`：所有 robot 发送一次 `RobotPingReq`。
- `uid 10001 RobotPingReq`：指定 UID 的 robot 发送一次 `RobotPingReq`。
- `quit` / `exit`：关闭所有 robot 并退出。

## 控制面板

浏览器打开：

```text
http://127.0.0.1:18080/
```

面板支持：

- 查看 robot 总数、在线数、登录成功数、发送/接收/失败统计。
- 查看最多前 200 个 robot 的 UID、连接、登录、用户数据、gateway、heartbeatSession、seq、队列。
- 查看当前发现的 gateway 列表。
- 从 `api.yaml` 读取消息列表。
- 点击 `all` 给所有 robot 发送当前选择的消息。
- 输入 UID 后点击 `uid` 给指定 robot 发送当前选择的消息。

面板接口：

- `GET /api/overview`：返回统计、robot 快照、gateway、api 消息。
- `POST /api/send`：发送消息，body 示例：`{"scope":"uid","uid":10001,"message":"RobotPingReq"}`。

如果指定 UID 的 robot 正在登录，业务命令会先进入待发送队列。`RobotPingReq` 必须等用户数据创建或确认存在后才会发送。如果已经登录但第一次心跳还没返回，业务消息仍可发送，包头 `SessionID` 使用 robot 本地递增值。

## RobotPing

`RobotPingReq/RobotPingRes` 是专门的压测消息，链路为：

```text
robot -> gateway -> online -> gateway -> robot
```

online 收到后直接回显：

- `seq`
- `clientTime`
- `serverTime`
- `payload`

该消息不写 Redis，不修改用户数据。

调用要求：

- `RobotPingReq` 只能在用户数据已经创建后调用。
- robot 登录成功后会先发送 `UserRecordReq`，用户数据为空时才发送 `UserCreateReq`。
- 如果 `UserCreateReq` 返回用户已存在，robot 也会认为用户数据已经就绪。

## 压测注意事项

- 当前 Windows 动态 TCP 端口范围通常约 16384 个端口，10000 个连接理论上够，但刚退出后立刻重启可能受 `TIME_WAIT` 影响。
- 不要对 10000 个 robot 开启逐包成功日志，默认只输出汇总和失败明细。
- `cacheTokenExpireSecond` 要大于批量启动和登录耗时，避免 token 在登录前过期。
- 建议按 `100 -> 1000 -> 5000 -> 10000` 逐步加压。
- 压测时同时观察 gateway、online、cache、Redis 和 Docker 资源。
