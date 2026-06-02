etcd:
  endpoints:
    - 192.168.71.123:2379
    - 192.168.71.123:22379
    - 192.168.71.123:32379
  ttlDuration: 3000000000s
  projectName: project

cacheTokenExpireSecond: 10

# proto 路径，用于记录协议来源。
protoPath: ../../../proto

# 收包打印时忽略的消息号列表。
ignoreMsgID:
  - 0x00000A
  - 0x00000B

# 登录验证成功后自动心跳间隔。
heartbeatInterval: 10s

controlPanel:
  enable: true
  addr: 127.0.0.1:18080

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
    summaryInterval: 10s
    detailFailures: true
