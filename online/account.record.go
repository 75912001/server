package main

import (
	pb "server/proto/pb"
)

func nextAccountRecordUUID(record *pb.AccountRecord) uint64 {
	record.UsedUuid++
	return record.GetUsedUuid()
}
