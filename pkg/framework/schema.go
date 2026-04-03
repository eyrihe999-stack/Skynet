package framework

import (
	"encoding/json"
	"fmt"
)

// Schema 是 Skill 输入/输出参数的声明式模式定义，以字段名为键、FieldDef 为值的映射。
//
// Schema 使用声明式 API 定义 Skill 的输入输出数据结构，
// 最终会被转换为标准 JSON Schema 格式，用于 Agent Card 的能力描述和参数校验。
//
// 使用示例：
//
//	Schema{
//	    "query": String("搜索关键词").Required(),
//	    "limit": Int("返回结果数量"),
//	}
type Schema map[string]FieldDef

// FieldDef 描述 Schema 中单个字段的定义信息。
//
// FieldDef 是声明式 Schema 构建器的核心类型，支持链式调用（如 String("desc").Required()）。
// 每个 FieldDef 包含字段的类型、描述、是否必填等元信息，
// 最终通过 Schema.ToJSONSchema() 转换为标准 JSON Schema 属性。
//
// 字段说明：
//   - typ: JSON Schema 类型（"string"、"integer"、"number"、"boolean"、"array"、"object"）
//   - description: 字段的功能描述
//   - required: 是否为必填字段
//   - enumValues: 枚举类型的可选值列表（仅 typ="string" 时有效）
//   - items: 数组元素的类型定义（仅 typ="array" 时有效）
type FieldDef struct {
	typ         string
	description string
	required    bool
	enumValues  []string
	items       *FieldDef // 数组类型的元素定义
}

// 以下是 Schema 字段定义的构建器函数，用于以声明式方式创建各种类型的 FieldDef。

// String 创建一个字符串类型的字段定义。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "string" 的字段定义，支持链式调用 .Required()
func String(description string) FieldDef {
	return FieldDef{typ: "string", description: description}
}

// Int 创建一个整数类型的字段定义。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "integer" 的字段定义，支持链式调用 .Required()
func Int(description string) FieldDef {
	return FieldDef{typ: "integer", description: description}
}

// Number 创建一个浮点数类型的字段定义。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "number" 的字段定义，支持链式调用 .Required()
func Number(description string) FieldDef {
	return FieldDef{typ: "number", description: description}
}

// Bool 创建一个布尔类型的字段定义。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "boolean" 的字段定义，支持链式调用 .Required()
func Bool(description string) FieldDef {
	return FieldDef{typ: "boolean", description: description}
}

// Enum 创建一个带枚举约束的字符串类型字段定义。
//
// 生成的 JSON Schema 中会包含 "enum" 字段，限制值只能为指定的选项之一。
//
// 参数：
//   - description: 字段的功能描述
//   - values: 允许的枚举值列表
//
// 返回值：
//   - FieldDef: 带枚举约束的 "string" 类型字段定义
func Enum(description string, values ...string) FieldDef {
	return FieldDef{typ: "string", description: description, enumValues: values}
}

// StringArray 创建一个字符串数组类型的字段定义。
//
// 生成的 JSON Schema 中 type 为 "array"，items.type 为 "string"。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 元素类型为 string 的数组字段定义
func StringArray(description string) FieldDef {
	item := FieldDef{typ: "string"}
	return FieldDef{typ: "array", description: description, items: &item}
}

// Array 创建一个通用数组类型的字段定义（未指定元素类型）。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "array" 的字段定义
func Array(description string) FieldDef {
	return FieldDef{typ: "array", description: description}
}

// Object 创建一个对象类型的字段定义。
//
// 参数：
//   - description: 字段的功能描述
//
// 返回值：
//   - FieldDef: 类型为 "object" 的字段定义
func Object(description string) FieldDef {
	return FieldDef{typ: "object", description: description}
}

// Required 将当前字段标记为必填，并返回新的 FieldDef（值拷贝语义，不修改原值）。
//
// 标记为 Required 的字段会出现在生成的 JSON Schema 的 "required" 数组中。
// 支持链式调用，例如：String("名称").Required()
//
// 返回值：
//   - FieldDef: 标记了 required=true 的字段定义副本
func (f FieldDef) Required() FieldDef {
	f.required = true
	return f
}

// ToJSONSchema 将 Schema 转换为标准 JSON Schema 格式的 JSON 字节数据。
//
// 该方法遍历 Schema 中的所有字段定义，生成符合 JSON Schema 规范的对象结构，
// 包含 type、properties、required 等标准字段。
// 生成的 JSON Schema 用于 Agent Card 的能力描述，使调用方了解 Skill 的输入输出格式。
//
// 如果 Schema 为 nil，返回空 JSON 对象 "{}"。
//
// 返回值：
//   - json.RawMessage: 标准 JSON Schema 格式的 JSON 字节数据
func (s Schema) ToJSONSchema() json.RawMessage {
	if s == nil {
		return json.RawMessage(`{}`)
	}

	properties := make(map[string]any)
	var required []string

	for name, field := range s {
		prop := map[string]any{
			"type":        field.typ,
			"description": field.description,
		}
		if len(field.enumValues) > 0 {
			prop["enum"] = field.enumValues
		}
		if field.items != nil {
			items := map[string]any{"type": field.items.typ}
			prop["items"] = items
		}
		properties[name] = prop

		if field.required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	b, _ := json.Marshal(schema)
	return b
}

// Validate 根据 Schema 定义校验输入数据。
//
// 校验规则：
//   - 检查所有标记为 Required 的字段是否存在且不为 nil
//   - 检查已存在字段的值类型是否与 Schema 声明的类型匹配
//
// 如果 Schema 为 nil（未定义输入 Schema），跳过校验直接返回 nil。
//
// 参数：
//   - input: 从 JSON 解析出的原始键值对映射
//
// 返回值：
//   - error: 校验失败时返回描述性错误，通过时返回 nil
func (s Schema) Validate(input map[string]any) error {
	if s == nil {
		return nil
	}

	for name, field := range s {
		val, exists := input[name]

		// 必填字段检查
		if field.required && (!exists || val == nil) {
			return fmt.Errorf("missing required field '%s'", name)
		}

		// 字段不存在则跳过类型检查
		if !exists || val == nil {
			continue
		}

		// 类型检查
		if err := checkType(name, field.typ, val); err != nil {
			return err
		}
	}
	return nil
}

// checkType 检查单个字段值是否符合声明的 JSON Schema 类型。
//
// 参数：
//   - name: 字段名，用于错误信息
//   - typ: Schema 声明的类型（"string"、"integer"、"number"、"boolean"、"array"、"object"）
//   - val: 实际的字段值
//
// 返回值：
//   - error: 类型不匹配时返回错误，匹配时返回 nil
func checkType(name, typ string, val any) error {
	switch typ {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("field '%s' expects string, got %T", name, val)
		}
	case "integer":
		// JSON 数字在 Go 中解析为 float64，检查是否为整数值
		f, ok := val.(float64)
		if !ok {
			return fmt.Errorf("field '%s' expects integer, got %T", name, val)
		}
		if f != float64(int64(f)) {
			return fmt.Errorf("field '%s' expects integer, got float", name)
		}
	case "number":
		if _, ok := val.(float64); !ok {
			return fmt.Errorf("field '%s' expects number, got %T", name, val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("field '%s' expects boolean, got %T", name, val)
		}
	case "array":
		if _, ok := val.([]any); !ok {
			return fmt.Errorf("field '%s' expects array, got %T", name, val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("field '%s' expects object, got %T", name, val)
		}
	}
	return nil
}
