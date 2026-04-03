package framework

import "github.com/skynetplatform/skynet/pkg/protocol"

// frameworkVersion 是当前 Skynet Agent 框架 SDK 的版本号。
//
// 该版本号会包含在生成的 Agent Card 中，用于 Gateway 和调用方识别 Agent 使用的框架版本。
const frameworkVersion = "0.1.0"

// GenerateCard 根据 Agent 配置和已注册的 Skill 列表自动生成 Agent Card（能力名片）。
//
// Agent Card 是 Agent 在 Skynet 网络中的"身份证"，包含 Agent 的基本信息、
// 框架版本、连接模式以及所有 Skill 的能力描述（包括输入输出 Schema）。
// 在生产模式下，Agent Card 通过 WebSocket 注册消息发送给 Gateway；
// 在开发模式下，可通过本地 HTTP 服务器的 /agent-card 端点查看。
//
// 对于未显式设置 Visibility 或 ApprovalMode 的 Skill，
// 该函数会使用配置文件中 Defaults 段的默认值进行填充。
//
// 参数：
//   - cfg: Agent 配置，提供 Agent 基本信息和默认值
//   - skills: 已注册的 Skill 列表，每个 Skill 会转换为一个 CapabilityDef
//
// 返回值：
//   - protocol.AgentCard: 生成的 Agent Card，包含完整的 Agent 和能力描述信息
func GenerateCard(cfg *AgentConfig, skills []Skill) protocol.AgentCard {
	card := protocol.AgentCard{
		AgentID:          cfg.Agent.ID,
		OwnerAPIKey:      cfg.Network.APIKey,
		DisplayName:      cfg.Agent.DisplayName,
		Description:      cfg.Agent.Description,
		Version:          cfg.Agent.Version,
		FrameworkVersion: frameworkVersion,
		ConnectionMode:   "tunnel",
		Capabilities:     make([]protocol.CapabilityDef, 0, len(skills)),
	}

	for _, s := range skills {
		vis := string(s.Visibility)
		if vis == "" {
			vis = cfg.Defaults.Visibility
		}
		approval := string(s.ApprovalMode)
		if approval == "" {
			approval = cfg.Defaults.ApprovalMode
		}

		cap := protocol.CapabilityDef{
			Name:               s.Name,
			DisplayName:        s.DisplayName,
			Description:        s.Description,
			Category:           s.Category,
			Tags:               s.Tags,
			InputSchema:        s.Input.ToJSONSchema(),
			OutputSchema:       s.Output.ToJSONSchema(),
			Visibility:         vis,
			ApprovalMode:       approval,
			EstimatedLatencyMs: s.EstimatedLatencyMs,
		}
		card.Capabilities = append(card.Capabilities, cap)
	}

	return card
}
