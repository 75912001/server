package common

const AIDSegmentSize uint64 = 1000000000000

// GroupAIDStart 返回指定 group 的用户 AID 起始值。
func GroupAIDStart(groupID uint32) uint64 {
	return uint64(groupID)*AIDSegmentSize + 1
}
