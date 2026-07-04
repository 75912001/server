package gameconfig

import pb "server/proto/pb"

const petSavedBaseGradeOffsetMin = -2

var elementOrder = []pb.AssetElemental{
	pb.AssetElemental_AssetElemental_Earth,
	pb.AssetElemental_AssetElemental_Water,
	pb.AssetElemental_AssetElemental_Fire,
	pb.AssetElemental_AssetElemental_Wind,
}

var elementByKey = map[string]pb.AssetElemental{
	"earth": pb.AssetElemental_AssetElemental_Earth,
	"water": pb.AssetElemental_AssetElemental_Water,
	"fire":  pb.AssetElemental_AssetElemental_Fire,
	"wind":  pb.AssetElemental_AssetElemental_Wind,
}

var petAttributeKeys = []string{
	"poisonResist",
	"paralysisResist",
	"sleepResist",
	"stoneResist",
	"drunkResist",
	"confusionResist",
	"critical",
	"counter",
}

func isPetID(id int) bool {
	return id >= int(pb.AssetIDRange_AssetIDRange_Pet_Start) && id <= int(pb.AssetIDRange_AssetIDRange_Pet_End)
}

func isPetSkillID(id int) bool {
	return id >= int(pb.AssetIDRange_AssetIDRange_Pet_Skill_Start) && id <= int(pb.AssetIDRange_AssetIDRange_Pet_Skill_End)
}

func isCharacterID(id int) bool {
	return id >= int(pb.AssetIDRange_AssetIDRange_Character_Start) && id <= int(pb.AssetIDRange_AssetIDRange_Character_End)
}

func isSceneID(id int) bool {
	return id >= int(pb.AssetIDRange_AssetIDRange_Scene_Start) && id <= int(pb.AssetIDRange_AssetIDRange_Scene_End)
}
