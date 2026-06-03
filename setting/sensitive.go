package setting

import (
	"encoding/json"
	"strings"
	"sync"

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

// sensitiveMu 保护全局敏感词/规则和 PII 配置的并发读写。
// 在 controller HTTP handler（写）和 relay 请求（读）路径上必须加锁。
var sensitiveMu sync.RWMutex

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

// GetSensitiveWords 读取敏感词副本（线程安全）
func GetSensitiveWords() []string {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()
	out := make([]string, len(SensitiveWords))
	copy(out, SensitiveWords)
	return out
}

// SetSensitiveWords 替换敏感词（线程安全）
func SetSensitiveWords(words []string) {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	SensitiveWords = words
}

// AppendSensitiveWord 追加单个敏感词（线程安全）
func AppendSensitiveWord(word string) {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	SensitiveWords = append(SensitiveWords, word)
}

// GetSensitiveRules 读取敏感词规则副本（线程安全）
func GetSensitiveRules() []SensitiveRuleEntry {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()
	out := make([]SensitiveRuleEntry, len(SensitiveRules))
	copy(out, SensitiveRules)
	return out
}

// GetSensitiveRulesByGroup 读取过滤后的规则副本（线程安全）
func GetSensitiveRulesByGroup(group string) []SensitiveRuleEntry {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()
	if group == "" {
		out := make([]SensitiveRuleEntry, len(SensitiveRules))
		copy(out, SensitiveRules)
		return out
	}
	out := make([]SensitiveRuleEntry, 0, len(SensitiveRules))
	for _, r := range SensitiveRules {
		if r.Group == "" || r.Group == group {
			out = append(out, r)
		}
	}
	return out
}

// SetSensitiveRules 替换规则（线程安全）
func SetSensitiveRules(rules []SensitiveRuleEntry) {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	SensitiveRules = rules
}

// AppendSensitiveRule 追加规则（线程安全）
func AppendSensitiveRule(rule SensitiveRuleEntry) bool {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	for _, r := range SensitiveRules {
		if r.Word == rule.Word && r.Group == rule.Group {
			return false
		}
	}
	SensitiveRules = append(SensitiveRules, rule)
	return true
}

// UpdateSensitiveRule 原地更新规则级别/分类（线程安全）
func UpdateSensitiveRule(word, group string, level, category string) bool {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	for i, r := range SensitiveRules {
		if r.Word == word && r.Group == group {
			SensitiveRules[i].Level = level
			if category != "" {
				SensitiveRules[i].Category = category
			}
			return true
		}
	}
	return false
}

// DeleteSensitiveRule 删除规则（线程安全），返回是否存在
func DeleteSensitiveRule(word, group string) bool {
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	found := false
	newRules := make([]SensitiveRuleEntry, 0, len(SensitiveRules))
	for _, r := range SensitiveRules {
		if r.Word == word && r.Group == group {
			found = true
			continue
		}
		newRules = append(newRules, r)
	}
	if found {
		SensitiveRules = newRules
	}
	return found
}

func SensitiveWordsToString() string {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	words := make([]string, 0)
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.TrimSpace(w)
		if w != "" {
			words = append(words, w)
		}
	}
	SetSensitiveWords(words)
}

// SensitiveRulesToOptionsJson 将 SensitiveRules 序列化为 JSON 字符串
func SensitiveRulesToOptionsJson() string {
	sensitiveMu.RLock()
	rules := make([]SensitiveRuleEntry, len(SensitiveRules))
	copy(rules, SensitiveRules)
	sensitiveMu.RUnlock()

	if len(rules) == 0 {
		return "[]"
	}
	b, err := common.Marshal(rules)
	if err != nil {
		common.SysError("SensitiveRulesToOptionsJson marshal error: " + err.Error())
		return "[]"
	}
	return string(b)
}

// SensitiveRulesFromOptionsJson 从 JSON 字符串反序列化 SensitiveRules
func SensitiveRulesFromOptionsJson(s string) {
	if s == "" {
		SetSensitiveRules(nil)
		return
	}
	var rules []SensitiveRuleEntry
	if err := json.Unmarshal([]byte(s), &rules); err != nil {
		common.SysError("SensitiveRulesFromOptionsJson unmarshal error: " + err.Error())
		return
	}
	SetSensitiveRules(rules)
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

func ShouldCheckCompletionSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
}
