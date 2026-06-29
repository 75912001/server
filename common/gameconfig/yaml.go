package gameconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlMap map[string]*yaml.Node

func loadYAMLMap(dir string, filename string) (yamlMap, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, configError("读取配置文件失败: %s err:%v", path, err)
	}
	var doc yaml.Node
	if err = yaml.Unmarshal(data, &doc); err != nil {
		return nil, configError("YAML解析失败: %s err:%v", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, configError("配置文件根节点必须是对象: %s", path)
	}
	return requireMap(doc.Content[0], path)
}

func requireMap(node *yaml.Node, path string) (yamlMap, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, configError("配置字段必须是对象: %s", path)
	}
	out := yamlMap{}
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		key := keyNode.Value
		if key == "" {
			return nil, configError("配置 key 不能为空: %s", path)
		}
		if _, ok := out[key]; ok {
			return nil, configError("配置 key 重复: %s.%s", path, key)
		}
		out[key] = valueNode
	}
	return out, nil
}

func requireSeq(node *yaml.Node, path string) ([]*yaml.Node, error) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, configError("配置字段必须是数组: %s", path)
	}
	return node.Content, nil
}

func requireKey(data yamlMap, key string, path string) (*yaml.Node, error) {
	node, ok := data[key]
	if !ok {
		return nil, configError("配置缺少必填字段: %s.%s", path, key)
	}
	return node, nil
}

func assertKnownKeys(data yamlMap, allowed map[string]struct{}, path string) error {
	for key := range data {
		if _, ok := allowed[key]; !ok {
			return configError("配置字段未知: %s.%s", path, key)
		}
	}
	return nil
}

func assertAbsent(data yamlMap, key string, msg string, args ...any) error {
	if _, ok := data[key]; ok {
		return configError(msg, args...)
	}
	return nil
}

func intScalar(node *yaml.Node, path string) (int, error) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, configError("配置字段必须是整数: %s value:%s", path, nodeValue(node))
	}
	value, err := strconv.Atoi(node.Value)
	if err != nil {
		return 0, configError("配置字段必须是整数: %s value:%s", path, node.Value)
	}
	return value, nil
}

func nonNegativeIntScalar(node *yaml.Node, path string) (int, error) {
	value, err := intScalar(node, path)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, configError("配置字段不能为负数: %s value:%d", path, value)
	}
	return value, nil
}

func stringScalar(node *yaml.Node, path string) (string, error) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return "", configError("配置字段必须是字符串: %s value:%s", path, nodeValue(node))
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return "", configError("配置字段不能为空: %s", path)
	}
	return value, nil
}

func boolScalar(node *yaml.Node, path string) (bool, error) {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
		return false, configError("配置字段必须是布尔值: %s value:%s", path, nodeValue(node))
	}
	return node.Value == "true", nil
}

func floatScalar(node *yaml.Node, path string) (float64, error) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return 0, configError("配置字段必须是数字: %s value:%s", path, nodeValue(node))
	}
	value, err := strconv.ParseFloat(node.Value, 64)
	if err != nil {
		return 0, configError("配置字段必须是数字: %s value:%s", path, node.Value)
	}
	return value, nil
}

func intRange(node *yaml.Node, path string) (IntRange, error) {
	values, err := requireSeq(node, path)
	if err != nil {
		return IntRange{}, err
	}
	if len(values) != 2 {
		return IntRange{}, configError("配置范围必须包含两个整数: %s", path)
	}
	minValue, err := intScalar(values[0], path+".min")
	if err != nil {
		return IntRange{}, err
	}
	maxValue, err := intScalar(values[1], path+".max")
	if err != nil {
		return IntRange{}, err
	}
	if minValue > maxValue {
		return IntRange{}, configError("配置范围 min 不能大于 max: %s min:%d max:%d", path, minValue, maxValue)
	}
	return IntRange{Min: minValue, Max: maxValue}, nil
}

func assertRangeBounds(value IntRange, minValue int, maxValue int, path string) error {
	if value.Min < minValue || value.Max > maxValue {
		return configError("配置范围超出限制: %s range:[%d,%d] expected:[%d,%d]", path, value.Min, value.Max, minValue, maxValue)
	}
	return nil
}

func nodeValue(node *yaml.Node) string {
	if node == nil {
		return "<nil>"
	}
	return node.Value
}

func configError(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func stringSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
