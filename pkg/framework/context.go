package framework

import (
	"context"
	"encoding/json"

	"github.com/skynetplatform/skynet/pkg/protocol"
)

// Context 是 Skill 执行时的上下文结构体，封装了标准 context.Context 和调用元数据。
//
// Context 在每次 Skill 被调用时由框架自动创建，并传入 HandlerFunc。
// 它既可用作标准的 Go context（支持取消、超时等），也携带了 Skynet 特有的调用信息。
//
// 字段说明：
//   - Context: 内嵌的标准 context.Context，支持取消和超时控制
//   - RequestID: 当前请求的唯一标识符，用于日志追踪和结果回传
//   - Caller: 调用者信息，包含发起调用的 Agent 或用户的身份信息
type Context struct {
	context.Context
	RequestID string
	Caller    protocol.CallerInfo
}

// Input 提供对 Skill 输入数据的类型安全访问。
//
// Input 封装了从 JSON 解析出的原始 map 数据，并提供一系列类型化的访问器方法
// （String、Int、Bool、StringArray 等），使开发者无需手动进行类型断言。
// 当字段不存在或类型不匹配时，访问器返回对应类型的零值而非 panic。
//
// 字段说明：
//   - raw: 从 JSON 输入解析出的原始键值对映射
type Input struct {
	raw map[string]any
}

// NewInput 从原始 JSON 数据创建一个 Input 实例。
//
// 该函数将 JSON 格式的原始字节数据反序列化为 map[string]any，
// 并封装为 Input 结构体。如果传入 nil 或反序列化失败，
// 会创建一个空的 Input（内含空 map），确保后续访问不会 panic。
//
// 参数：
//   - data: JSON 格式的原始输入数据，可以为 nil
//
// 返回值：
//   - Input: 封装了解析结果的输入参数访问器
func NewInput(data json.RawMessage) Input {
	var m map[string]any
	if data != nil {
		_ = json.Unmarshal(data, &m)
	}
	if m == nil {
		m = make(map[string]any)
	}
	return Input{raw: m}
}

// Has 检查输入数据中是否存在指定字段。
//
// 参数：
//   - key: 要检查的字段名
//
// 返回值：
//   - bool: true 表示字段存在，false 表示不存在
func (in Input) Has(key string) bool {
	_, ok := in.raw[key]
	return ok
}

// String 获取输入数据中指定字段的字符串值。
//
// 如果字段不存在或值不是 string 类型，返回空字符串。
//
// 参数：
//   - key: 要获取的字段名
//
// 返回值：
//   - string: 字段的字符串值，字段不存在或类型不匹配时返回 ""
func (in Input) String(key string) string {
	v, ok := in.raw[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// Int 获取输入数据中指定字段的整数值。
//
// 由于 JSON 数字在 Go 中默认反序列化为 float64，
// 该方法会先提取 float64 值再转换为 int。
// 如果字段不存在或值不是数字类型，返回 0。
//
// 参数：
//   - key: 要获取的字段名
//
// 返回值：
//   - int: 字段的整数值，字段不存在或类型不匹配时返回 0
func (in Input) Int(key string) int {
	v, ok := in.raw[key]
	if !ok {
		return 0
	}
	// JSON 数字在 Go 中默认解析为 float64
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int(f)
}

// Bool 获取输入数据中指定字段的布尔值。
//
// 如果字段不存在或值不是 bool 类型，返回 false。
//
// 参数：
//   - key: 要获取的字段名
//
// 返回值：
//   - bool: 字段的布尔值，字段不存在或类型不匹配时返回 false
func (in Input) Bool(key string) bool {
	v, ok := in.raw[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// StringArray 获取输入数据中指定字段的字符串数组值。
//
// 该方法从 JSON 数组（[]any）中提取所有 string 类型的元素，
// 非 string 类型的元素会被静默跳过。
// 如果字段不存在或值不是数组类型，返回 nil。
//
// 参数：
//   - key: 要获取的字段名
//
// 返回值：
//   - []string: 字段的字符串数组值，字段不存在或类型不匹配时返回 nil
func (in Input) StringArray(key string) []string {
	v, ok := in.raw[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// Raw 返回输入数据的底层原始 map。
//
// 当类型化的访问器方法无法满足需求时（如需要访问嵌套对象或动态字段），
// 可以使用该方法获取原始 map 进行自定义处理。
//
// 返回值：
//   - map[string]any: 输入数据的原始键值对映射
func (in Input) Raw() map[string]any {
	return in.raw
}

// NeedInputResult 是 Skill Handler 的特殊返回值，表示需要更多用户输入。
// 当 Handler 返回此类型时，框架会发送 need_input 消息给 Gateway，
// 而不是发送 result 消息。
type NeedInputResult struct {
	Question protocol.Question
}

// NeedInput 创建一个追问结果，表示 Skill 需要更多输入才能完成。
// 在 Handler 中使用：return framework.NeedInput(protocol.Question{...}), nil
func NeedInput(q protocol.Question) *NeedInputResult {
	return &NeedInputResult{Question: q}
}
