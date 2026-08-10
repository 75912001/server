package gameconfig

import pb "server/proto/pb"

const petSavedBaseGradeOffsetMin int32 = -2

func isPetID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Pet_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Pet_End)
}

func isSkillID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Skill_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Skill_End)
}

func isCharacterID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Character_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Character_End)
}

func isSceneID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Scene_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Scene_End)
}

func isItemID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Item_Item_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Item_Item_End)
}

func isEquipmentID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Item_Equipment_End)
}
