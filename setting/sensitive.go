package setting

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/static"
)

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// SensitiveWords 敏感词（启动时自动合并内置词库）
var SensitiveWords []string

// SensitiveRules 分级敏感词规则列表
var SensitiveRules []SensitiveRuleEntry

// SensitiveRuleEntry 敏感词规则条目（用于 options 存储）
type SensitiveRuleEntry struct {
	Word     string `json:"word"`
	Level    string `json:"level"`              // block/warn/log
	Category string `json:"category,omitempty"` // 分类标签
	Group    string `json:"group,omitempty"`    // 所属分组
}

func init() {
	// 加载内置敏感词库
	SensitiveWords = static.GetBuiltinWords()
}

func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

// SensitiveRulesToOptionsJson 将 SensitiveRules 序列化为 JSON 字符串
func SensitiveRulesToOptionsJson() string {
	if len(SensitiveRules) == 0 {
		return "[]"
	}
	b, err := common.Marshal(SensitiveRules)
	if err != nil {
		common.SysError("SensitiveRulesToOptionsJson marshal error: " + err.Error())
		return "[]"
	}
	return string(b)
}

// SensitiveRulesFromOptionsJson 从 JSON 字符串反序列化 SensitiveRules
func SensitiveRulesFromOptionsJson(s string) {
	if s == "" {
		SensitiveRules = nil
		return
	}
	var rules []SensitiveRuleEntry
	if err := json.Unmarshal([]byte(s), &rules); err != nil {
		common.SysError("SensitiveRulesFromOptionsJson unmarshal error: " + err.Error())
		return
	}
	SensitiveRules = rules
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

func ShouldCheckCompletionSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
}
