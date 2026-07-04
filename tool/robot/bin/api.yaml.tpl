# 测试消息

#0x000001#client->gateway#账号登录验证-请求
AccountVerifyReq:
  id: 0x000001
  msg:
    aid: 10001
    connectTicket: ""

#0x00000A#client->gateway#心跳-请求
AccountHeartbeatReq:
  id: 0x00000A
  msg:
    lastHeartbeatSession: ""

#0x00000C#client->gateway#请求账号档案-请求
AccountRecordReq:
  id: 0x00000C
  msg:

#0x000003#client->gateway#账号主动下线-请求
AccountOfflineReq:
  id: 0x000003
  msg:
    reason: 1

#0x00000E#client->gateway#创建角色-请求
CharacterCreateReq:
  id: 0x00000E
  msg:
    characterElemental:
      earth: 10
      water: 0
      fire: 0
      wind: 0
    characterAttribute:
      vitality: 5
      strength: 5
      toughness: 5
      dexterity: 5

#0x000010#client->gateway#机器人压测-请求
RobotPingReq:
  id: 0x000010
  msg:
    seq: 0
    clientTime: 0
    payload: "robot-ping"
