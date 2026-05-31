# 测试消息

#0x000001#client->gateway#登录验证-请求
UserVerifyReq:
  id: 0x000001
  msg:
    uid: 10001
    token: "token-10001"

#0x00000A#client->gateway#心跳-请求
UserHeartbeatReq:
  id: 0x00000A
  msg:
    lastSession: 0

#0x00000C#client->gateway#请求用户数据-请求
UserRecordReq:
  id: 0x00000C
  msg:

#0x000003#client->gateway#主动下线-请求
UserOfflineReq:
  id: 0x000003
  msg:
    reason: 1
