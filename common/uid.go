package common

const UIDSegmentSize uint64 = 1000000000000

// GroupUIDStart 返回指定 group 的用户 UID 起始值。
func GroupUIDStart(groupID uint32) uint64 {
	return uint64(groupID)*UIDSegmentSize + 1
}
