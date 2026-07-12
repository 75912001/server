package main

import (
	pb "server/proto/pb"
)

func nextAccountRecordUUID(record *pb.AccountRecord) uint64 {
	record.UsedUuid++
	return record.GetUsedUuid()
}

// accountRecordReady 表示账号角色档案已初始化完成, 即 account_record_create_timestamp_ms 已写入。
// 角色上线、进场和战斗等业务必须满足该状态。
func (p *Account) accountRecordReady() bool {
	return p.accountRecord != nil && p.accountRecord.GetAccountRecordCreateTimestampMs() != 0
}
