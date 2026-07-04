package gameconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigIncludesScene(t *testing.T) {
	manager, err := Load("../../config")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if manager.Scene == nil {
		t.Fatalf("manager.Scene is nil")
	}
	for _, sceneID := range []int{2000001, 2000002, 2000003} {
		scene := manager.Scene.GetByID(sceneID)
		if scene == nil {
			t.Fatalf("scene %d not found", sceneID)
		}
		if len(scene.EnemyGroups) != 1 {
			t.Fatalf("scene %d enemyGroups len = %d, want 1", sceneID, len(scene.EnemyGroups))
		}
	}
}

func TestSceneConfigValidation(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		checkEnemy *EnemyGroupConfig
		wantErr    bool
	}{
		{
			name: "valid",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups:
      - id: 1
        weight: 100
`,
			checkEnemy: testEnemyGroupConfig(1),
		},
		{
			name: "scene_id_out_of_range",
			yaml: `scenes:
  - id: 1999999
    name: 场景一
    enemyGroups:
      - id: 1
        weight: 100
`,
			wantErr: true,
		},
		{
			name: "duplicate_scene_id",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups:
      - id: 1
        weight: 100
  - id: 2000001
    name: 场景二
    enemyGroups:
      - id: 1
        weight: 100
`,
			wantErr: true,
		},
		{
			name: "empty_enemy_groups",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups: []
`,
			wantErr: true,
		},
		{
			name: "enemy_group_missing",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups:
      - id: 2
        weight: 100
`,
			checkEnemy: testEnemyGroupConfig(1),
			wantErr:    true,
		},
		{
			name: "weight_zero",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups:
      - id: 1
        weight: 0
`,
			wantErr: true,
		},
		{
			name: "weight_negative",
			yaml: `scenes:
  - id: 2000001
    name: 场景一
    enemyGroups:
      - id: 1
        weight: -1
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sceneConfig, err := loadSceneConfigForTest(t, tt.yaml)
			if err == nil && tt.checkEnemy != nil {
				err = sceneConfig.check(tt.checkEnemy)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("scene config error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("scene config error = %v", err)
			}
		})
	}
}

func loadSceneConfigForTest(t *testing.T, content string) (*SceneConfig, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileScene), []byte(content), 0o644); err != nil {
		t.Fatalf("write scene config failed: %v", err)
	}
	sceneConfig := newSceneConfig()
	return sceneConfig, sceneConfig.load(dir)
}

func testEnemyGroupConfig(ids ...int) *EnemyGroupConfig {
	config := &EnemyGroupConfig{
		byID: map[int]*EnemyGroupEntry{},
	}
	for _, id := range ids {
		config.byID[id] = &EnemyGroupEntry{ID: id}
		config.ids = append(config.ids, id)
	}
	return config
}
