package model

// SensitiveRuleLevel 敏感词级别
type SensitiveRuleLevel string

const (
	SensitiveRuleLevelBlock SensitiveRuleLevel = "block" // 阻断：直接拒绝请求
	SensitiveRuleLevelWarn  SensitiveRuleLevel = "warn"  // 警告：记录审计日志，允许通过
	SensitiveRuleLevelLog   SensitiveRuleLevel = "log"   // 日志：仅记录，不影响请求
)

// SensitiveRule 敏感词规则
type SensitiveRule struct {
	Word     string             `json:"word"`               // 敏感词（小写）
	Level    SensitiveRuleLevel `json:"level"`              // 级别：block/warn/log
	Category string             `json:"category,omitempty"` // 分类标签（可选）
	Group    string             `json:"group,omitempty"`    // 所属分组（可选，用于多租户隔离）
}

// SensitiveRuleKey 生成规则唯一键（word + group）
func (r *SensitiveRule) SensitiveRuleKey() string {
	if r.Group == "" {
		return r.Word
	}
	return r.Word + "\x00" + r.Group
}
