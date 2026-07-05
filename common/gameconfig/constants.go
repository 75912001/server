package gameconfig

import pb "server/proto/pb"

const petSavedBaseGradeOffsetMin int32 = -2

func isPetID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Pet_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Pet_End)
}

func isPetSkillID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Pet_Skill_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Pet_Skill_End)
}

func isCharacterID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Character_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Character_End)
}

func isSceneID(id uint32) bool {
	return id >= uint32(pb.AssetIDRange_AssetIDRange_Scene_Start) && id <= uint32(pb.AssetIDRange_AssetIDRange_Scene_End)
}
