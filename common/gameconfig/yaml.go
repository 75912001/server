package gameconfig

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func loadYAMLFile(dir string, filename string, out any) error {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.Errorf("读取配置文件失败: %s err:%v", path, err)
	}
	if err = yaml.Unmarshal(data, out); err != nil {
		return errors.Errorf("YAML解析失败: %s err:%v", path, err)
	}
	return nil
}

func valuePtr[T any](value T) *T {
	return &value
}

func defaultBool(value **bool, defaultValue bool) {
	if *value == nil {
		*value = valuePtr(defaultValue)
	}
}
