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
  - 0xb
  #- 0x10000
  #- 0x20000

# 登录验证成功后自动心跳间隔。
heartbeatInterval: 10s
