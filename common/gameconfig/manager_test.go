package gameconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRepositoryConfig(t *testing.T) {
	manager, err := Load(filepath.Join("..", "..", "config"))
	if err != nil {
		t.Fatalf("load repository config: %v", err)
	}
	if !manager.Pet.HasID(4000101) {
		t.Fatalf("expected pet 4000101")
	}
	if !manager.PetSkill.HasID(9000001) {
		t.Fatalf("expected pet skill 9000001")
	}
	if manager.Character.GetByID(1000011) == nil {
		t.Fatalf("expected character 1000011")
	}
	if manager.Enemy.GetByID(1) == nil {
		t.Fatalf("expected enemy group 1")
	}
	level, err := manager.Exp.GetLevel(0)
	if err != nil {
		t.Fatalf("get level: %v", err)
	}
	if level != 1 {
		t.Fatalf("level = %d, want 1", level)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		patch   func(files map[string]string)
		wantErr string
	}{
		{
			name: "missing section",
			patch: func(files map[string]string) {
				files[FilePetSkill] = "skills: []\n"
			},
			wantErr: "配置缺少必填字段",
		},
		{
			name: "invalid character id",
			patch: func(files map[string]string) {
				files[FileCharacter] = strings.Replace(files[FileCharacter], "  - id: 1000001\n", "  - id: 999999\n", 1)
			},
			wantErr: "角色ID超出范围",
		},
		{
			name: "duplicate character id",
			patch: func(files map[string]string) {
				files[FileCharacter] = strings.Replace(files[FileCharacter], "character:\n", "character:\n  - id: 1000001\n", 1)
			},
			wantErr: "角色ID重复",
		},
		{
			name: "invalid character isRole",
			patch: func(files map[string]string) {
				files[FileCharacter] = strings.Replace(files[FileCharacter], "    isRole: true\n", "    isRole: yes\n", 1)
			},
			wantErr: "配置字段必须是布尔值",
		},
		{
			name: "duplicate pet skill id",
			patch: func(files map[string]string) {
				files[FilePetSkill] = strings.Replace(files[FilePetSkill], "skill:\n", "skill:\n  - id: 9000001\n    name: 重复\n    description: 重复\n", 1)
			},
			wantErr: "宠物技能ID重复",
		},
		{
			name: "invalid pet id",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "  - id: 4000001\n", "  - id: 3999999\n", 1)
			},
			wantErr: "宠物ID超出范围",
		},
		{
			name: "duplicate pet id",
			patch: func(files map[string]string) {
				files[FilePet] = "pet:\n" + buildPetEntryYAML(4000001) + buildPetEntryYAML(4000001)
			},
			wantErr: "宠物ID重复",
		},
		{
			name: "invalid pet rarity",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "    rarity: 1\n", "    rarity: 999\n", 1)
			},
			wantErr: "宠物稀有度非法",
		},
		{
			name: "unknown pet elemental",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "      earth: 10\n", "      bad: 10\n", 1)
			},
			wantErr: "宠物 elemental 元素未知",
		},
		{
			name: "invalid pet attribute",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "      poisonResist: 0\n", "      poisonResist: bad\n", 1)
			},
			wantErr: "宠物 attribute 字段非法",
		},
		{
			name: "invalid pet growth",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "      lvupPointSource: 4.50\n", "      lvupPointSource: 0\n", 1)
			},
			wantErr: "宠物 growth.lvupPointSource 必须大于0",
		},
		{
			name: "invalid pet skill slot id",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "    skill: [9000001, 0]\n", "    skill: [1]\n", 1)
			},
			wantErr: "宠物 skill 槽位ID超出范围",
		},
		{
			name: "missing pet skill reference",
			patch: func(files map[string]string) {
				files[FilePet] = strings.Replace(files[FilePet], "    skill: [9000001, 0]\n", "    skill: [9000002]\n", 1)
			},
			wantErr: "宠物引用了未定义技能",
		},
		{
			name: "enemy baby rate out of range",
			patch: func(files map[string]string) {
				files[FileEnemyGroup] = strings.Replace(files[FileEnemyGroup], "    babyRate: 0\n", "    babyRate: 100001\n", 1)
			},
			wantErr: "babyRate 超出范围",
		},
		{
			name: "missing enemy pet reference",
			patch: func(files map[string]string) {
				files[FileEnemyGroup] = strings.Replace(files[FileEnemyGroup], "      - id: 4000001\n", "      - id: 4000002\n", 1)
			},
			wantErr: "敌人组引用了未定义宠物",
		},
		{
			name: "exp level incomplete",
			patch: func(files map[string]string) {
				files[FileExp] = buildExpYAML(139)
			},
			wantErr: "经验等级数量必须完整覆盖协议范围",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			files := validConfigFiles()
			tt.patch(files)
			writeConfigFiles(t, dir, files)

			_, err := Load(dir)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func validConfigFiles() map[string]string {
	return map[string]string{
		FilePetSkill:   buildPetSkillYAML(),
		FilePet:        buildPetYAML(),
		FileCharacter:  buildCharacterYAML(),
		FileEnemyGroup: buildEnemyGroupYAML(),
		FileExp:        buildExpYAML(140),
	}
}

func writeConfigFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func buildPetSkillYAML() string {
	return `skill:
  - id: 9000001
    name: 待机
    description: 什么也不做
`
}

func buildPetYAML() string {
	return "pet:\n" + buildPetEntryYAML(4000001)
}

func buildPetEntryYAML(id int) string {
	var b strings.Builder
	b.WriteString("  - id: ")
	b.WriteString(itoa(id))
	b.WriteString(`
    rarity: 1
    elemental:
      earth: 10
    attribute:
      poisonResist: 0
      paralysisResist: 0
      sleepResist: 0
      stoneResist: 0
      drunkResist: 0
      confusionResist: 0
      critical: 0
      counter: 0
    growth:
      initNum: 5
      lvupPointSource: 4.50
      baseVital: 10
      baseStr: 10
      baseTough: 10
      baseDex: 10
    skill: [9000001, 0]
`)
	return b.String()
}

func buildCharacterYAML() string {
	return `character:
  - id: 1000001
    isRole: true
    name: 测试角色
    description: 测试描述
    color: blue
`
}

func buildEnemyGroupYAML() string {
	return `enemyGroups:
  - id: 1
    name: 测试敌人组
    countRange: [1, 1]
    levelRange: [1, 1]
    captured: true
    babyRate: 0
    enemies:
      - id: 4000001
`
}

func buildExpYAML(maxLevel int) string {
	var b strings.Builder
	b.WriteString("levels:\n")
	for level := 1; level <= maxLevel; level++ {
		b.WriteString("  ")
		b.WriteString(itoa(level))
		b.WriteString(":\n    max: ")
		b.WriteString(itoa(level * 10))
		b.WriteString("\n")
	}
	return b.String()
}
