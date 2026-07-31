package common

import (
	"server/proto/pb"
	"unicode/utf8"

	xnetcommon "github.com/75912001/xlib/net/common"
	xpacket "github.com/75912001/xlib/packet"
)

const (
	OnlineServerName  = "online"
	OnlinePackageName = "online"
	OnlineServiceName = "OnlineService"

	GatewayServerName  = "gateway"
	GatewayPackageName = "gateway"
	GatewayServiceName = "GatewayService"

	CacheServerName  = "cache"
	CachePackageName = "cache"
	CacheServiceName = "CacheService"

	LoginServerName = "login"

	ConnectTicketSecretDefault = "server-dev-connect-ticket-secret"
)

const (
	// [10000,20000] 留给业务使用
	DisconnectReason_xxx xnetcommon.DisconnectReason = 10000 // 未知原因
)

func init() {
	xpacket.SetEndianMode(xpacket.LittleEndian)
}

// IsValidCharacterNick 判断角色昵称是否合法
func IsValidCharacterNick(characterNick string) bool {
	nameRuneCount := utf8.RuneCountInString(characterNick)
	return 0 < nameRuneCount && nameRuneCount <= int(pb.Constants_Constants_Character_Name_Max_Length)
}

// IsValidElementalAllocation 判断元素点分配是否合法。
// 规则: 总点数为 10, 最多 2 种元素, 双元素按地-水-火-风-地判断相邻。
func IsValidElementalAllocation(points *pb.ElementalPoints) bool {
	values := [...]uint32{
		points.GetEarth(),
		points.GetWater(),
		points.GetFire(),
		points.GetWind(),
	}

	total := uint32(0)
	activeIndexes := make([]int, 0, pb.Constants_Constants_Elemental_Max_Active_Type_Count)

	for index, value := range values {
		total += value
		if value == 0 {
			continue
		}
		if value > uint32(pb.Constants_Constants_Elemental_Total_Point) {
			return false
		}

		activeIndexes = append(activeIndexes, index)
		if len(activeIndexes) > int(pb.Constants_Constants_Elemental_Max_Active_Type_Count) {
			return false
		}
	}

	if total != uint32(pb.Constants_Constants_Elemental_Total_Point) {
		return false
	}
	if len(activeIndexes) == 1 {
		return true
	}
	if len(activeIndexes) != 2 {
		return false
	}

	diff := activeIndexes[0] - activeIndexes[1]
	if diff < 0 {
		diff = -diff
	}
	return diff == 1 || diff == len(values)-1
}
