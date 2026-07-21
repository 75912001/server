package gameconfig

import (
	"strings"

	pb "server/proto/pb"

	xmap "github.com/75912001/xlib/map"
	xruntime "github.com/75912001/xlib/runtime"
	"github.com/pkg/errors"
)

type ItemUseTarget string

const (
	ItemUseTargetCharacter ItemUseTarget = "character"
	ItemUseTargetPet       ItemUseTarget = "pet"
)

type ItemConfig struct {
	*xmap.MapMgr[uint32, *ItemEntry]
}

type ItemEntry struct {
	ID       *uint32       `yaml:"-"`
	Name     *string       `yaml:"name"`
	MaxStack *uint64       `yaml:"maxStack"`
	Use      *ItemUseEntry `yaml:"use"`
}

type ItemUseEntry struct {
	Target *ItemUseTarget `yaml:"target"`
	Exp    *uint64        `yaml:"exp"`
}

func newItemConfig() *ItemConfig {
	return &ItemConfig{MapMgr: xmap.NewMapMgr[uint32, *ItemEntry]()}
}

func (p *ItemConfig) load(dir string) error {
	var root struct {
		Items map[uint32]*ItemEntry `yaml:"items"`
	}
	if err := loadYAMLFile(dir, FileItem, &root); err != nil {
		return err
	}
	for itemID, entry := range root.Items {
		if itemID < uint32(pb.AssetIDRange_AssetIDRange_Item_Start) || itemID > uint32(pb.AssetIDRange_AssetIDRange_Item_End) {
			return errors.Errorf("道具ID超出协议范围: id:%d %v", itemID, xruntime.Location())
		}
		if entry == nil {
			return errors.Errorf("道具配置不能为空: id:%d %v", itemID, xruntime.Location())
		}
		itemIDValue := itemID
		entry.ID = &itemIDValue
		if entry.Name == nil || strings.TrimSpace(*entry.Name) == "" {
			return errors.Errorf("道具名称不能为空: id:%d %v", itemID, xruntime.Location())
		}
		if entry.MaxStack == nil || *entry.MaxStack == 0 {
			return errors.Errorf("道具堆叠上限必须大于0: id:%d %v", itemID, xruntime.Location())
		}
		if entry.Use == nil || entry.Use.Target == nil || entry.Use.Exp == nil || *entry.Use.Exp == 0 {
			return errors.Errorf("道具使用配置不完整: id:%d %v", itemID, xruntime.Location())
		}
		switch *entry.Use.Target {
		case ItemUseTargetCharacter, ItemUseTargetPet:
		default:
			return errors.Errorf("道具使用目标无效: id:%d target:%q %v", itemID, *entry.Use.Target, xruntime.Location())
		}
		p.Add(itemID, entry)
	}
	if len(root.Items) == 0 {
		return errors.Errorf("道具配置不能为空: %s %v", FileItem, xruntime.Location())
	}
	return nil
}

func (p *ItemConfig) check() error {
	return nil
}

func (p *ItemConfig) assemble() error {
	return nil
}
